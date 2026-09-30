// impact-service — impact & ROI as an fx library module.
//
// Ported onto the combi chassis: persistence lives in pkg/persistence/postgres
// (ImpactRepository), domain types in pkg/domain/impact. This module has no
// main package; a host binary mounts it via Module below.
package impact

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the impact service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, events.Bus (fxcore).
//
// The container takes no optional domain services — only the registrar and
// registry, which fall back to discards so the module also validates
// standalone. NOTE: NewContainer spawns an async seedDefaults goroutine;
// under fx.ValidateApp constructors never run, under a real host it needs a
// live DB (same as the pre-fx wiring).
var Module = fx.Module(
	"impact",
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

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service impact.Service

	// Workers: RefreshWorker is stored as a queue.JobProcessor in the
	// Container, so it is re-emitted as the interface; EventWorker is a
	// concrete *EventWorker; Workers is the dispatch slice.
	RefreshWorker queue.JobProcessor
	EventWorker   *EventWorker
	Workers       []queue.JobProcessor
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
	// NewContainer takes a single Dependencies value with Registrar/Registry
	// nested inside.
	c, err := NewContainer(Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		Registrar: registrar,
		Registry:  registry,
		EventBus:  d.Bus,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:     c,
		Service:       c.Service,
		RefreshWorker: c.RefreshWorker,
		EventWorker:   c.EventWorker,
		Workers:       c.Workers,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("impact", func(ctx context.Context) error {
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
