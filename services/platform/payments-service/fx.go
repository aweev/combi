// payments-service — payments/financial service as an fx library module.
//
// Ported from api/internal/services/financial (Phase 3) onto the combi
// chassis: persistence lives in pkg/persistence/postgres
// (FinancialRepository), domain types in pkg/domain/financial. This module
// has no main package; a host binary (cmd/platform-server) mounts it via
// Module below.
package payments

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
)

// Module is the fx module for the payments service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger, events.Bus (fxcore), plus audit.EventRecorder
// (auditlog module — REQUIRED, not optional: the service layer calls
// Record/Publish unguarded, so a nil recorder would panic at runtime;
// failing loudly at graph validation is the honest contract).
//
// Optional dependencies — queue.Dispatcher, integration.ProviderFactory —
// arrive nil (dig `optional:"true"`) until their modules are mounted.
// Registrar/registry fall back to discards so the module also validates
// standalone.
var Module = fx.Module(
	"payments",
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
	Audit  audit.EventRecorder

	Dispatcher queue.Dispatcher            `optional:"true"`
	ProvFac    integration.ProviderFactory `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service      financial.Service
	AdminService financial.AdminService
	// Grouped (not bare): feeds the future reporting-service aggregator
	// via the "reportFetchers" group — see auditlog.servicesOut. A bare
	// provide would collide with auditlog's feeder in the same host.
	ReportingFetcher reporting.DataFetcher `group:"reportFetchers"`
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
	factory := d.ProvFac
	if factory == nil {
		factory = discardProviderFactory{}
	}
	c, err := NewContainer(Dependencies{
		Repo:                       pPostgres.NewFinancialRepository(d.DB, d.Logger),
		Logger:                     d.Logger,
		AuditRecorder:              d.Audit,
		Dispatcher:                 d.Dispatcher,
		EventBus:                   d.Bus,
		IntegrationProviderFactory: factory,
		Registry:                   registry,
	}, registrar)
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:        c,
		Service:          c.Service,
		AdminService:     c.AdminService,
		ReportingFetcher: c.ReportingFetcher,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("payments", func(ctx context.Context) error {
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

// discardProviderFactory stands in for the integration-service provider
// factory until that module is ported. Every method reports "not ported";
// GetPaymentGateway's error path is already handled gracefully by
// NewContainer (warn log, limited admin service) — the same path api takes
// with no active gateway.
type discardProviderFactory struct{}

func (discardProviderFactory) GetPaymentGateway(ctx context.Context) (financial.PaymentGateway, error) {
	return nil, errors.New("no payment gateway configured: integration-service not yet ported")
}

func (discardProviderFactory) BuildPaymentGateway(provider core.ProviderType, creds map[string]string) (financial.PaymentGateway, error) {
	return nil, errors.New("integration-service not yet ported")
}

func (discardProviderFactory) BuildEmailProvider(provider core.ProviderType, creds map[string]string) (notification.EmailProvider, error) {
	return nil, errors.New("integration-service not yet ported")
}

func (discardProviderFactory) BuildSMSProvider(provider core.ProviderType, creds map[string]string) (notification.SMSProvider, error) {
	return nil, errors.New("integration-service not yet ported")
}

func (discardProviderFactory) BuildPushProvider(typ core.ProviderType, config map[string]string) (notification.PushProvider, error) {
	return nil, errors.New("integration-service not yet ported")
}

func (discardProviderFactory) BuildWebPushProvider(typ core.ProviderType, config map[string]string) (notification.WebPushProvider, error) {
	return nil, errors.New("integration-service not yet ported")
}

func (discardProviderFactory) GetWebhookParser(config *integration.Integration, creds map[string]string) (integration.WebhookParser, error) {
	return nil, errors.New("integration-service not yet ported")
}
