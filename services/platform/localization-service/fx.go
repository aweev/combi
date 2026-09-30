// localization-service — localization service as an fx library module.
//
// Ported from api/internal/services/localization (Phase 3, Tier 1) onto the
// combi chassis: persistence lives in pkg/persistence/postgres
// (LocalizationRepository), domain types in pkg/domain/localization. This
// module has no main package; a host binary (cmd/platform-server) mounts it
// via Module below.
package localization

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the localization service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, cache.Service (platform utils).
//
// Optional dependencies — rbac.AuthorizationService, audit.EventRecorder —
// arrive nil (dig `optional:"true"`) until their modules are mounted; the
// service layer stores them for request-time use. Registrar/registry fall
// back to discards so the module also validates standalone.
var Module = fx.Module(
	"localization",
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
	Cache  cache.Service

	Authz rbac.AuthorizationService `optional:"true"`
	Audit audit.EventRecorder       `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service localization.Service
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
	c, err := NewContainer(Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		Cache:     d.Cache,
		Authz:     d.Authz,
		Audit:     d.Audit,
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
	d.Checker.Register("localization", func(ctx context.Context) error {
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
