// monitoring-service — system monitoring & alerts as an fx library module.
//
// Ported onto the combi chassis: persistence lives in
// pkg/persistence/postgres (MonitoringRepository), domain types in
// pkg/monitoring (AlertAdminService). This module has no main package; a
// host binary (cmd/platform-server) mounts it via Module below.
package monitoring

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pmonitoring "github.com/combi/pkg/monitoring"
)

// Module is the fx module for the monitoring service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, events.Bus (fxcore).
//
// Optional dependencies — audit.EventRecorder — arrive nil
// (dig `optional:"true"`) until their modules are mounted; the service
// layer stores them for request-time use. Registry falls back to a discard
// so the module also validates standalone. (NewContainer takes no
// Registrar.)
var Module = fx.Module(
	"monitoring",
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
	Config *config.Config
	Bus    events.Bus

	Audit audit.EventRecorder `optional:"true"`

	Registry core.Registry `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service pmonitoring.AlertAdminService
	Worker  queue.JobProcessor
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registry := d.Registry
	if registry == nil {
		registry = fxnoop.NoopRegistry{}
	}
	// NOTE: Audit arrives nil (dig `optional:"true"`) until the audit
	// module is mounted (nil-safe stopgap convention). PrometheusURL
	// derives from host config; empty string when unconfigured.
	// NewContainer returns *Container with no error.
	c := NewContainer(Dependencies{
		DB:            d.DB,
		Logger:        d.Logger,
		EventBus:      d.Bus,
		AuditRecorder: d.Audit,
		PrometheusURL: d.Config.Monitoring.PrometheusURL,
		Registry:      registry,
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
	d.Checker.Register("monitoring", func(ctx context.Context) error {
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
