// integration-service — integration service as an fx library module.
//
// Ported from api/internal/services/integration (Phase 3, Tier 2) onto the
// combi chassis: persistence lives in pkg/persistence/postgres
// (IntegrationRepository), domain types in pkg/domain/integration. This
// module has no main package; a host binary (cmd/platform-server) mounts it
// via Module below.
package integration

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the integration service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, security.RandomService (platform utils).
//
// Optional dependencies — audit recorder, dispatcher, event bus — arrive nil
// (dig `optional:"true"`) until mounted. Registrar/registry fall back to
// discards so the module also validates standalone.
var Module = fx.Module(
	"integration",
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
	Random security.RandomService

	Audit      audit.EventRecorder `optional:"true"`
	Dispatcher queue.Dispatcher   `optional:"true"`
	Bus        events.Bus          `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service         *Service
	ProviderFactory integration.ProviderFactory
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
		DB:            d.DB,
		Logger:        d.Logger,
		AuditRecorder: d.Audit,
		Dispatcher:    d.Dispatcher,
		CryptoService: crypto,
		EventBus:      d.Bus,
		Registry:      registry,
	}, registrar)
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:       c,
		Service:         c.Service,
		ProviderFactory: c.ProviderFactory,
	}, nil
}

// cryptoFromAuditKey builds the module's AES-GCM crypto service from the
// shared audit encryption key (mirrors api wiring, which reuses the audit
// key for integration credential crypto).
func cryptoFromAuditKey(cfg *config.Config, random security.RandomService) (security.CryptoService, error) {
	key := []byte(cfg.Auth.AuditEncryptionKey)
	if len(key) == 0 {
		return nil, errors.New("auth.auditEncryptionKey is not set: integration credential crypto needs it")
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
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("integration", func(ctx context.Context) error {
		return nil
	})

	d.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
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
