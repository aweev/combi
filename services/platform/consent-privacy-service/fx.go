// consent-privacy-service — consent + privacy services as one fx library module.
//
// Ports api/internal/services/consent and api/internal/services/privacy
// (Phase 3, Tier 2; merged per plan.md) onto the combi chassis: persistence
// lives in pkg/persistence/postgres (ConsentRepository, PrivacyRepository),
// domain types in pkg/domain/consent and pkg/domain/privacy. The two
// services stay in their own subpackages (consent/, privacy/) — merged only
// at the fx wiring layer here. This module has no main package; a host
// binary (cmd/platform-server) mounts it via Module below.
package consentprivacy

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	consentsvc "github.com/combi/consent/consent"
	privacysvc "github.com/combi/consent/privacy"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/consent"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the consent-privacy service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger.
//
// Optional dependencies — audit recorder, event bus — arrive nil (dig
// `optional:"true"`) until mounted. Registrar/registry fall back to discards
// so the module also validates standalone.
var Module = fx.Module(
	"consentprivacy",
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

	Audit audit.EventRecorder `optional:"true"`
	Bus   events.Bus          `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	ConsentContainer *consentsvc.Container
	PrivacyContainer *privacysvc.Container

	ConsentService consent.Service
	PrivacyManager privacy.Manager
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
	consentC, err := consentsvc.NewContainer(consentsvc.Dependencies{
		DB:        d.DB,
		Logger:    d.Logger,
		Registrar: registrar,
		Audit:     d.Audit,
		Registry:  registry,
		EventBus:  d.Bus,
	})
	if err != nil {
		return servicesOut{}, err
	}
	privacyC, err := privacysvc.NewContainer(privacysvc.Dependencies{
		DB:             d.DB,
		Logger:         d.Logger,
		AnonymizerSvcs: nil,
		Registry:       registry,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		ConsentContainer: consentC,
		PrivacyContainer: privacyC,
		ConsentService:   consentC.Service,
		PrivacyManager:   privacyC.Manager,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC      fx.Lifecycle
	Checker *health.Checker
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("consentprivacy", func(ctx context.Context) error {
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
