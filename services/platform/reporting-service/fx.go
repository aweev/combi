// reporting-service — pluggable report generation engine as an fx library
// module.
//
// Ported onto the combi chassis: persistence lives in
// pkg/persistence/postgres (ReportingRepository), domain types in
// pkg/domain/reporting. This module has no main package; a host binary
// (cmd/platform-server) mounts it via Module below.
package reporting

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/persistence/postgres"
)

// Module is the fx module for the reporting service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger (platform utils).
//
// Optional dependencies — queue.Dispatcher, storage.Service,
// audit.EventRecorder — arrive nil (dig `optional:"true"`) until their
// modules are mounted; the service layer stores them for request-time use.
// Registry passes through as-is: NewContainer guards it internally, so the
// module also validates standalone. (NewContainer takes no Registrar and
// returns *Container with no error.)
var Module = fx.Module(
	"reporting",
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

	Queue   queue.Dispatcher    `optional:"true"`
	Storage storage.Service     `optional:"true"`
	Audit   audit.EventRecorder `optional:"true"`

	Registry core.Registry `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service reporting.Service
	Worker  queue.JobProcessor
}

func provideServices(d moduleDeps) (servicesOut, error) {
	// NOTE: the host graph has no reporting.Repository provider, so the
	// postgres-backed repo is built here from *sqlx.DB. Queue/Storage/Audit
	// arrive nil (dig `optional:"true"`) until their modules are mounted
	// (nil-safe stopgap convention).
	repo := postgres.NewReportingRepository(d.DB, d.Logger)
	c := NewContainer(Dependencies{
		Repo:     repo,
		Queue:    d.Queue,
		Storage:  d.Storage,
		Audit:    d.Audit,
		Logger:   d.Logger,
		Registry: d.Registry,
	})
	return servicesOut{
		Container: c,
		Service:   c.Service,
		Worker:    c.Worker,
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
	d.Checker.Register("reporting", func(ctx context.Context) error {
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
