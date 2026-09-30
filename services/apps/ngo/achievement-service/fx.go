// achievement-service — gamification as an fx library module.
//
// Ported onto the combi chassis: persistence lives in pkg/persistence/postgres
// (AchievementRepository), domain types in pkg/domain/achievement. This module
// has no main package; a host binary mounts it via Module below.
package achievement

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the achievement service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, cache.Service, events.Bus (fxcore).
//
// Optional dependencies — user.Service — arrive nil (dig `optional:"true"`)
// until their modules are mounted; the service layer stores them for
// request-time use (nil-safe stopgap). The achievement sub-config maps from
// the shared *config.Config (zero value when the section is unset).
// Registrar/registry fall back to discards so the module also validates
// standalone.
var Module = fx.Module(
	"achievement",
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
	Logger log.Logger
	Bus    events.Bus
	Cache  cache.Service
	Config *config.Config

	UserSvc user.Service `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service    achievement.Service
	Subscriber *Subscriber
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
	// NewContainer takes a single Dependencies value with Registrar/Registry
	// nested inside; Config is a config.AchievementConfig value mapped from
	// the shared *config.Config.
	c, err := NewContainer(Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		EventBus:  d.Bus,
		UserSvc:   d.UserSvc,
		Registrar: registrar,
		Registry:  registry,
		Cache:     d.Cache,
		Config:    d.Config.Achievement,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:  c,
		Service:    c.Service,
		Subscriber: c.Subscriber,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("achievement", func(ctx context.Context) error {
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
