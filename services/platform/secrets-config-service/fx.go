// secrets-config-service — secrets + configuration as one fx library module.
//
// Ports api/internal/services/secret and api/internal/services/configuration
// (Phase 3, Tier 0/1; merged per plan.md) onto the combi chassis:
// persistence lives in pkg/persistence/postgres (SecretRepository,
// ConfigRepository), domain types in pkg/domain/secret and
// pkg/domain/configuration. The two services stay in their own subpackages
// (secret/, configuration/) — merged only at the fx wiring layer here. This
// module has no main package; a host binary (cmd/platform-server) mounts it
// via Module below.
package secretsconfig

import (
	"context"

	"github.com/jmoiron/sqlx"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	configsvc "github.com/combi/config/configuration"
	secretsvc "github.com/combi/config/secret"
	"github.com/combi/pkg/cache"
	pCircuit "github.com/combi/pkg/circuit"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/configuration"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the secrets-config service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, *redis.Client,
// cache.Service + security.RandomService (platform utils),
// monitoring.Service (fxcore).
//
// Optional dependencies — audit recorder, dispatcher, event bus — arrive nil
// (dig `optional:"true"`) until mounted. Registrar/registry fall back to
// discards so the module also validates standalone.
var Module = fx.Module(
	"secretsconfig",
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
	Redis  *redis.Client
	Cache  cache.Service
	Random security.RandomService
	Mon    monitoring.Service

	Audit      audit.EventRecorder `optional:"true"`
	Dispatcher queue.Dispatcher    `optional:"true"`
	Bus        events.Bus          `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
}

type servicesOut struct {
	fx.Out

	SecretContainer *secretsvc.Container
	ConfigContainer *configsvc.Container

	SecretService   secret.Service
	SecretAdmin     secret.AdminService
	ConfigService   configuration.Service
	ServiceRegistry core.Registry
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registrar := d.Registrar
	if registrar == nil {
		registrar = fxnoop.NoopRegistrar{}
	}
	breakerFactory := func(name string) pCircuit.Breaker {
		return pCircuit.NewCircuitBreaker(name, d.Redis, d.Config.CB, d.Logger)
	}
	configC, err := configsvc.NewContainer(configsvc.Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		EventBus:  d.Bus,
		Registrar: registrar,
		Config:    d.Config,
	})
	if err != nil {
		return servicesOut{}, err
	}
	// The configuration service IS the service registry (it implements
	// core.Registry): every other container in this process registers its
	// manifest with it, mirroring api wiring order (configuration first).
	secretC, err := secretsvc.NewContainer(secretsvc.Dependencies{
		Config:         d.Config,
		DB:             d.DB,
		Logger:         d.Logger,
		AuditRecorder:  d.Audit,
		Monitoring:     d.Mon,
		BreakerFactory: breakerFactory,
		EventBus:       d.Bus,
		Dispatcher:     d.Dispatcher,
		Registrar:      registrar,
		Registry:       configC.Service,
		Cache:          d.Cache,
		RandomSvc:      d.Random,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		SecretContainer: secretC,
		ConfigContainer: configC,
		SecretService:   secretC.Service,
		SecretAdmin:     secretC.AdminService,
		ConfigService:   configC.Service,
		ServiceRegistry: configC.Service,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC      fx.Lifecycle
	Checker *health.Checker
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("secretsconfig", func(ctx context.Context) error {
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
