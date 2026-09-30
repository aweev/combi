// campaign-service — fundraising campaigns as an fx library module.
//
// Ported onto the combi chassis: persistence lives in pkg/persistence/postgres
// (CampaignRepository), domain types in pkg/domain/campaign. This module has
// no main package; a host binary mounts it via Module below.
package campaign

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the campaign service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, events.Bus (fxcore).
//
// Optional dependencies — audit.EventRecorder — arrive nil
// (dig `optional:"true"`) until their modules are mounted; the service layer
// stores them for request-time use (nil-safe stopgap). Registrar/registry
// fall back to discards so the module also validates standalone.
var Module = fx.Module(
	"campaign",
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

	Audit audit.EventRecorder `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service      campaign.Service
	AdminService campaign.AdminService
	Subscriber   *Subscriber
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
	// nested inside (unlike project-service which takes them as extra args).
	c, err := NewContainer(Dependencies{
		DB:            d.DB,
		Logger:        d.Logger,
		EventBus:      d.Bus,
		Registrar:     registrar,
		Registry:      registry,
		AuditRecorder: d.Audit,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:    c,
		Service:      c.Service,
		AdminService: c.AdminService,
		Subscriber:   c.Subscriber,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("campaign", func(ctx context.Context) error {
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
