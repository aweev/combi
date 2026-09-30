// notification-service — notification service as an fx library module.
//
// Ported from api/internal/services/notification (Phase 3, Tier 2) onto the
// combi chassis: persistence lives in pkg/persistence/postgres
// (NotificationRepository), domain types in pkg/domain/notification. This
// module has no main package; a host binary (cmd/platform-server) mounts it
// via Module below.
package notification

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the notification service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, cache.Service (platform utils),
// security.RandomService (platform utils).
//
// Optional dependencies — audit recorder, dispatcher, event bus,
// localization service, integration provider factory — arrive nil (dig
// `optional:"true"`) until their modules are mounted. Registrar/registry
// fall back to discards so the module also validates standalone.
var Module = fx.Module(
	"notification",
	fx.Provide(
		provideServices,
	),
	fx.Invoke(
		RegisterHooks,
	),
)

type moduleDeps struct {
	fx.In

	DB     *sqlx.DB
	Config *config.Config
	Logger log.Logger
	Cache  cache.Service
	Random security.RandomService

	Audit      audit.EventRecorder         `optional:"true"`
	Bus        events.Bus                  `optional:"true"`
	Dispatcher queue.Dispatcher           `optional:"true"`
	LocSvc     localization.Service       `optional:"true"`
	ProvFac    integration.ProviderFactory `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service      notification.Service
	AdminService notification.AdminService
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registrar := d.Registrar
	if registrar == nil {
		registrar = discardRegistrar{}
	}
	registry := d.Registry
	if registry == nil {
		registry = discardRegistry{}
	}
	crypto, err := cryptoFromAuditKey(d.Config, d.Random)
	if err != nil {
		return servicesOut{}, err
	}
	c, err := NewContainer(Dependencies{
		DB:              d.DB,
		Logger:          d.Logger,
		Config:          d.Config.Email,
		Audit:           d.Audit,
		EventBus:        d.Bus,
		Dispatcher:      d.Dispatcher,
		Cache:           d.Cache,
		Crypto:          crypto,
		Registrar:       registrar,
		Registry:        registry,
		LocalizationSvc: d.LocSvc,
		ProviderFactory: d.ProvFac,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:    c,
		Service:      c.Service,
		AdminService: c.AdminService,
	}, nil
}

// cryptoFromAuditKey builds the module's AES-GCM crypto service from the
// shared audit encryption key (mirrors api wiring, which reuses the audit
// key for notification/integration payload crypto).
func cryptoFromAuditKey(cfg *config.Config, random security.RandomService) (security.CryptoService, error) {
	key := []byte(cfg.Auth.AuditEncryptionKey)
	if len(key) == 0 {
		return nil, errors.New("auth.auditEncryptionKey is not set: notification payload crypto needs it")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("auth.auditEncryptionKey must be 16, 24, or 32 bytes (AES-128/192/256)")
	}
	return security.NewAesGcmCryptoService(key, random)
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Logger    log.Logger
	Container *Container
	Bus       events.Bus `optional:"true"`
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("notification", func(ctx context.Context) error {
		return nil
	})

	d.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if d.Bus != nil {
				d.Container.RegisterSubscribers(d.Bus)
			} else {
				d.Logger.Debug("event bus not mounted; notification subscribers idle")
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
}

type discardRegistrar struct{}

func (discardRegistrar) Register(rbac.PermissionDefinition) {}
func (discardRegistrar) SyncWithDataStore(context.Context) error { return nil }
func (discardRegistrar) GetDefinitions() []rbac.PermissionDefinition { return nil }

type discardRegistry struct{}

func (discardRegistry) RegisterService(core.ServiceManifest) {}
func (discardRegistry) GetSystemCapabilities(context.Context) ([]core.ServiceManifest, error) {
	return nil, nil
}
