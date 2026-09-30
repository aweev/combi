package integration

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/combi/integration/workers"
)


// Container holds all public-facing services for the Integration domain.
type Container struct {
	Service         *Service
	Workers         []queue.JobProcessor
	ProviderFactory integration.ProviderFactory
}

// Dependencies holds external inputs.
type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	Dispatcher    queue.Dispatcher
	CryptoService security.CryptoService
	EventBus      events.Bus
	Registry      core.Registry
}

func NewContainer(deps Dependencies, registrar rbac.PermissionRegistrar) (*Container, error) {
	// 1. Register Permissions
	registerPermissions(registrar)
	deps.Registry.RegisterService(buildManifest())

	// 2. Initialize Repository (Postgres)
	repo := postgres.NewIntegrationRepository(deps.DB, deps.Logger)

	// 3. Initialize Provider Factory
	// UPDATED: Now takes Repo and Crypto to support GetPaymentGateway logic
	providerFactory := NewProviderFactory(deps.Logger, repo, deps.CryptoService)

	// 4. Initialize Service
	service := NewService(ServiceDependencies{
		Repo:            repo,
		CryptoService:   deps.CryptoService,
		ProviderFactory: providerFactory,
		AuditRecorder:   deps.AuditRecorder,
		Dispatcher:      deps.Dispatcher,
		Logger:          deps.Logger,
	})

	// 5. Initialize Workers
	syncWorker := workers.NewSyncWorker(repo, service, deps.AuditRecorder, deps.Logger)
	webhookWorker := workers.NewWebhookWorker(repo, providerFactory, deps.EventBus, deps.CryptoService, deps.Logger)

	workers := []queue.JobProcessor{
		syncWorker,
		webhookWorker,
	}

	return &Container{
		Service:         service,
		Workers:         workers,
		ProviderFactory: providerFactory,
	}, nil
}

func registerPermissions(r rbac.PermissionRegistrar) {
	r.Register(rbac.PermissionDefinition{Name: integration.PermIntegrationRead, Description: "View integration configurations", Category: "Integration"})
	r.Register(rbac.PermissionDefinition{Name: integration.PermIntegrationManage, Description: "Create/Update integrations", Category: "Integration"})
	r.Register(rbac.PermissionDefinition{Name: integration.PermIntegrationTest, Description: "Test connection to providers", Category: "Integration"})
	r.Register(rbac.PermissionDefinition{Name: integration.PermWebhookRead, Description: "View received webhooks", Category: "Integration"})
	r.Register(rbac.PermissionDefinition{Name: integration.PermWebhookReplay, Description: "Replay webhook events", Category: "Integration"})
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "integration-service",
		DisplayName: "External Integrations",
		Description: "Connectors for Stripe, Twilio, etc.",
		Events: []core.EventDefinition{
			{Key: core.EventWebhookReceived, Label: "Webhook Received", Description: "Raw webhook ingested."},
			{Key: core.EventIntegrationConnectionTested, Label: "Connection Test", Description: "Admin verified provider credentials."},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         integration.JobIntegrationRunSync,
				Label:       "Sync Provider Status",
				Description: "Check health of all active integrations.",
				InputSchema: map[string]string{},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: integration.PermIntegrationManage, Label: "Manage Integrations", Category: "Configuration"},
		},
	}
}
