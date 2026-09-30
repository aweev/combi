// realtime-service — WebSocket realtime gateway as an fx library module.
//
// Ported onto the combi chassis: presence persistence lives in the redis
// adapter, domain types in pkg/domain/realtime. This module has no main
// package; a host binary (cmd/platform-server) mounts it via Module below.
package realtime

import (
	"context"

	redis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	drealtime "github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pmonitoring "github.com/combi/pkg/monitoring"
)

// Module is the fx module for the realtime service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *redis.Client, *config.Config, log.Logger + pmonitoring.Service
// (platform utils; Service embeds Recorder so it satisfies the hub's
// Metrics dependency).
//
// Optional dependencies — rbac.AuthorizationService, audit.EventRecorder —
// arrive nil (dig `optional:"true"`) until their modules are mounted; the
// hub/authorizer store them for request-time use. Registrar/registry fall
// back to discards so the module also validates standalone.
var Module = fx.Module(
	"realtime",
	fx.Provide(
		provideServices,
	),
	fx.Invoke(
		RegisterHooks,
	),
)

type moduleDeps struct {
	fx.In

	Redis   *redis.Client
	Logger  log.Logger
	Config  *config.Config
	Metrics pmonitoring.Service

	RBACService   rbac.AuthorizationService `optional:"true"`
	AuditRecorder audit.EventRecorder       `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Hub          drealtime.Hub
	PresenceRepo drealtime.PresenceRepository
	Subscriber   *EventSubscriber
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registrar := d.Registrar
	if registrar == nil {
		registrar = fxnoop.NoopRegistrar{}
	}
	registry := d.Registry
	if registry == nil {
		registry = fxnoop.NoopRegistry{}
	}
	// NOTE: RBACService/AuditRecorder arrive nil (dig `optional:"true"`)
	// until their modules are mounted (nil-safe stopgap convention).
	// NewContainer takes the realtime config twice: once inside
	// Dependencies and once as the second argument.
	c, err := NewContainer(Dependencies{
		RedisClient:   d.Redis,
		RBACService:   d.RBACService,
		AuditRecorder: d.AuditRecorder,
		Metrics:       d.Metrics,
		Logger:        d.Logger,
		Registrar:     registrar,
		Registry:      registry,
		Config:        d.Config.Realtime,
	}, d.Config.Realtime)
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:    c,
		Hub:          c.Hub,
		PresenceRepo: c.PresenceRepo,
		Subscriber:   c.Subscriber,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Logger    log.Logger
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("realtime", func(ctx context.Context) error {
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
