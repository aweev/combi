// messaging-service — chat & messaging service as an fx library module.
//
// Ported onto the combi chassis: persistence lives in
// pkg/persistence/postgres (MessagingRepository), domain types in
// pkg/domain/messaging. This module has no main package; a host binary
// (cmd/platform-server) mounts it via Module below.
package messaging

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the messaging service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, events.Bus (fxcore).
//
// Optional dependencies — user.Service — arrive nil (dig `optional:"true"`)
// until their modules are mounted; the service layer stores them for
// request-time use. Registrar/registry fall back to discards so the module
// also validates standalone.
var Module = fx.Module(
	"messaging",
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

	UserSvc user.Service `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service messaging.Service
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
	// NOTE: UserSvc arrives nil (dig `optional:"true"`) until the user
	// module is mounted; the service layer stores it for request-time
	// enrichment (nil-safe stopgap convention).
	c, err := NewContainer(Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		EventBus:  d.Bus,
		UserSvc:   d.UserSvc,
		Registrar: registrar,
		Registry:  registry,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container: c,
		Service:   c.Service,
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
	d.Checker.Register("messaging", func(ctx context.Context) error {
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
