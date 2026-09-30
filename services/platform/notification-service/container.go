package notification

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/combi/notification/processor"
)


// Container holds the public API of the Notification Service.
type Container struct {
	Service       notification.Service
	AdminService  notification.AdminService
	Workers       []queue.JobProcessor
	EventListener *EventListener
	DomainSub     *DomainSubscriber

	processor processor.Processor
}

type Dependencies struct {
	DB              *sqlx.DB
	Logger          log.Logger
	Config          config.EmailConfig
	Audit           audit.EventRecorder
	EventBus        events.Bus
	Dispatcher      queue.Dispatcher
	Cache           cache.Service
	Crypto          security.CryptoService
	Registrar       rbac.PermissionRegistrar
	Registry        core.Registry
	LocalizationSvc localization.Service
	ProviderFactory integration.ProviderFactory
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	// 2. Init Repository
	repo := postgres.NewNotificationRepository(deps.DB, deps.Logger)

	// 3. Init Helpers
	renderer := NewRenderer(repo, deps.Cache, deps.Logger, deps.LocalizationSvc)
	prefChecker := NewPreferenceChecker(repo, deps.Cache, deps.Logger)

	// 4. Init Processor (The Core Logic for Sending)
	proc := processor.NewProcessor(processor.Config{
		Repo:             repo,
		Renderer:         renderer,
		Prefs:            prefChecker,
		Factory:          deps.ProviderFactory,
		Logger:           deps.Logger,
		DefaultFromEmail: deps.Config.Default,
		Dispatcher:       deps.Dispatcher,
		Crypto:           deps.Crypto,
		EventBus:         deps.EventBus,
	})

	// 5. Init Services
	svc := NewService(repo, deps.Dispatcher, "default")
	adminSvc := NewAdminService(repo, deps.Dispatcher, deps.Audit, deps.Crypto, deps.Logger)

	// 6. Init Workers
	sendWorker := NewWorker(proc)
	scheduledWorker := NewScheduledWorker(repo, deps.Dispatcher, deps.Logger)
	cleanupWorker := NewCleanupWorker(repo, deps.Logger)

	// 7. Init Event Listener (Webhooks -> Status Updates)
	listener := NewEventListener(repo, deps.Logger)
	domainSub := NewDomainSubscriber(svc, deps.Logger)

	return &Container{
		Service:       svc,
		AdminService:  adminSvc,
		Workers:       []queue.JobProcessor{sendWorker, scheduledWorker, cleanupWorker},
		EventListener: listener,
		DomainSub:     domainSub,
		processor:     proc,
	}, nil
}

func (c *Container) SetUserLocaleFetcher(fetcher notification.UserLocaleFetcher) {
	if c.processor != nil {
		c.processor.SetUserLocaleFetcher(fetcher)
	}
}

// RegisterSubscribers implements events.EventSubscriber interface
func (c *Container) RegisterSubscribers(bus events.Bus) {
	bus.Subscribe(string(core.EventEmailBounced), c.EventListener.HandleEmailBounced)
	bus.Subscribe(string(core.EventSMSFailed), c.EventListener.HandleSMSFailed)
	bus.Subscribe(string(core.EventAccountCreated), c.DomainSub.OnAccountCreated)
	bus.Subscribe(string(core.EventMemberInvited), c.DomainSub.OnOrganizationInvite)
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	const category = "Notifications"
	registrar.Register(rbac.PermissionDefinition{Name: notification.PermNotificationAdminRead, Description: "View notification logs and templates.", Category: category})
	registrar.Register(rbac.PermissionDefinition{Name: notification.PermNotificationTemplateManage, Description: "Create and edit notification templates.", Category: category})
	registrar.Register(rbac.PermissionDefinition{Name: notification.PermNotificationProviderManage, Description: "Configure notification providers.", Category: category})

}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "notification-service",
		DisplayName: "Notification Engine",
		Description: "Multi-channel messaging (Email, SMS, Push) with templates.",
		Icon:        "send",
		Version:     "1.0.0",
		Tier:        "capability",
		Owner:       "platform",
		Portable:    true,
		Interfaces: []core.InterfaceDefinition{
			{
				Protocol:    "grpc",
				Package:     "platform.notification.v1",
				SpecPath:    "api/proto/notification/v1/notification.proto",
				Stability:   "stable",
				Description: "Preferred inter-service API for sending, scheduling, and checking notification status.",
				Consumers:   []string{"auth-service", "organization-service", "workflow-service", "monitoring-service"},
			},
			{
				Protocol:    "rest",
				Endpoint:    "/api/v1/admin/notifications",
				SpecPath:    "internal/services/notification/api/openapi.yaml",
				Stability:   "stable",
				Description: "Template, provider, preference, and delivery log administration.",
			},
		},
		Dependencies: []core.DependencyDefinition{
			{Name: "postgres", Type: "postgres", Required: true, ConfigKey: "database", Description: "Templates, preferences, provider config, delivery logs, and schedules."},
			{Name: "queue", Type: "queue", Required: true, ConfigKey: "queue", Description: "Retryable send, scheduled delivery, and cleanup jobs."},
			{Name: "event-bus", Type: "event-bus", Required: true, ConfigKey: "eventBus", Description: "Delivery, retry, bounce, and failure events."},
			{Name: "integration", Type: "service", Required: true, Description: "Provider adapters for email, SMS, push, and webhooks."},
			{Name: "localization", Type: "service", Required: false, Description: "Localized templates and user locale resolution."},
		},
		Telemetry: core.TelemetryDefinition{
			MetricsPrefix: "notification",
			TraceName:     "notification-service",
			LogFields:     []string{"notification_id", "channel", "provider", "template_key", "recipient_hash"},
			Dashboards:    []string{"notification-delivery", "notification-provider-health", "notification-retries"},
			Alerts:        []string{"notification_provider_failure_rate_high", "notification_delivery_lag", "notification_retry_exhausted"},
		},
		Events: []core.EventDefinition{
			{Key: core.EventEmailBounced, Label: "Email Bounced"},
			{Key: core.EventNotificationRetried, Label: "Notification Retried"},
		},
		Jobs: []core.JobDefinition{
			{Key: notification.JobSendNotification, Label: "Send Notification", InputSchema: map[string]string{"notification_id": "uuid"}},
			{Key: notification.JobCleanupOld, Label: "Cleanup Logs", InputSchema: map[string]string{"older_than_days": "int"}},
		},
		Permissions: []core.PermDefinition{
			{Key: notification.PermNotificationTemplateManage, Label: "Manage Templates", Category: "Communication"},
			{Key: notification.PermNotificationAdminRead, Label: "View Logs", Category: "Communication"},
		},
	}
}
