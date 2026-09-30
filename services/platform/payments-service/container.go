package payments

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/payments/workers"
)


type Dependencies struct {
	Repo                       financial.Repository
	Logger                     log.Logger
	AuditRecorder              audit.EventRecorder
	Dispatcher                 queue.Dispatcher
	EventBus                   events.Bus
	IntegrationProviderFactory integration.ProviderFactory
	Registry                   core.Registry // <--- ADDED THIS FIELD
}

type Container struct {
	Service          financial.Service
	AdminService     financial.AdminService
	ReportingFetcher reporting.DataFetcher
	workers          []queue.JobProcessor
}

func NewContainer(deps Dependencies, registrar rbac.PermissionRegistrar) (*Container, error) {
	if deps.Repo == nil || deps.IntegrationProviderFactory == nil || deps.Registry == nil {
		return nil, fmt.Errorf("missing dependencies for financial service (Repo, ProviderFactory, or Registry)")
	}
	// 1. Get Gateway
	paymentGateway, err := deps.IntegrationProviderFactory.GetPaymentGateway(context.Background())
	if err != nil {
		deps.Logger.Warn("No active payment gateway found. Financial Admin Service will be limited.", log.ErrorAttr(err))
	} else {
		deps.Logger.Info("Loaded payment gateway", log.String("provider", paymentGateway.ProviderName()))
	}

	// 2. Register Permissions & Manifest
	// Now deps.Registry exists
	deps.Registry.RegisterService(core.ServiceManifest{
		ServiceName: "financial-service",
		DisplayName: "Financial",
		Description: "Payments, Subscriptions, and Ledger.",
		Icon:        "dollar-sign",
		Events: []core.EventDefinition{
			{Key: core.EventPaymentSucceeded, Label: "Payment Succeeded"},
			{Key: core.EventPaymentFailed, Label: "Payment Failed"},
			{Key: core.EventDonationReceived, Label: "Donation Received"},
			{Key: core.EventSubscriptionCreated, Label: "Subscription Created"},
			{Key: core.EventSubscriptionRenewed, Label: "Subscription Renewed"},
		},
		Permissions: []core.PermDefinition{
			{Key: financial.PermFinancialRead, Label: "Read Financial Data", Category: "Financial"},
			{Key: financial.PermFinancialManage, Label: "Manage Finances", Category: "Financial"},
			{Key: financial.PermFinancialRefund, Label: "Issue Refunds", Category: "Financial"},
			{Key: financial.PermFinancialLedgerRead, Label: "Read Ledger", Category: "Financial"},
		},
	})

	// Register individual permissions for RBAC checks
	registrar.Register(rbac.PermissionDefinition{Name: financial.PermFinancialRead, Description: "Read financial data", Category: "Financial"})
	registrar.Register(rbac.PermissionDefinition{Name: financial.PermFinancialManage, Description: "Manage finances", Category: "Financial"})
	registrar.Register(rbac.PermissionDefinition{Name: financial.PermFinancialRefund, Description: "Issue refunds", Category: "Financial"})
	registrar.Register(rbac.PermissionDefinition{Name: financial.PermFinancialLedgerRead, Description: "Read ledger", Category: "Financial"})

	// 3. Init Services
	ledger := NewLedgerService(deps.Logger)

	service := NewService(
		deps.Repo,
		deps.IntegrationProviderFactory,
		ledger,
		deps.AuditRecorder,
		deps.Dispatcher,
		deps.EventBus,
		deps.Logger,
	)

	adminService := NewAdminService(
		deps.Repo,
		paymentGateway,
		ledger,
		deps.AuditRecorder,
		deps.Dispatcher,
		deps.EventBus,
		deps.Logger,
	)

	// 4. Init Workers
	renewalWorker := workers.NewRenewalWorker(service, deps.Repo, deps.Logger)
	refundWorker := workers.NewRefundWorker(adminService, deps.Logger)

	// 5. Reporting Fetcher
	reportingFetcher := NewReportingFetcher(adminService)

	return &Container{
		Service:          service,
		AdminService:     adminService,
		ReportingFetcher: reportingFetcher,
		workers:          []queue.JobProcessor{renewalWorker, refundWorker},
	}, nil
}

func (c *Container) GetWorkers() []queue.JobProcessor {
	return c.workers
}
