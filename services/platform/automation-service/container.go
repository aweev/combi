package automation

import (
	"context"
	_ "embed"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/limiter"
	"github.com/combi/pkg/log"
)


// Container holds the public-facing services for the Automation domain.
type Container struct {
	Service             automation.Service
	EventIngestService  automation.EventIngestService
	EventWorker         queue.JobProcessor
	ActionWorker        queue.JobProcessor
	ContinuationWorker  queue.JobProcessor
	RecoveryService     *RecoveryService
	SubscriptionManager *SubscriptionManager
	Subscriber          *Subscriber
}

// Dependencies holds all external dependencies for building the services.
type Dependencies struct {
	DB              *sqlx.DB
	Redis           *redis.Client
	Logger          log.Logger
	Registrar       rbac.PermissionRegistrar
	EventsBus       events.Bus
	Dispatcher      queue.Dispatcher
	RateLimiter     limiter.RateLimiter
	NotificationSvc notification.Service
	ImpactSvc       impact.Service
	ProjectSvc      project.Service
	ContentSvc      content.Service
	ReportingSvc    reporting.Service
	Registry        core.Registry
}

// NewContainer is the factory function for the entire automation service.
func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Manifest (Permissions & Jobs)
	RegisterPermissions(deps.Registrar)
	deps.Registry.RegisterService(BuildManifest())

	// 2. Infra
	repo := postgres.NewAutomationRepository(deps.DB, deps.Logger)
	evaluator := NewExprEvaluator() // Used the high-perf one
	statsBuffer := NewStatsBuffer(deps.Redis, repo, deps.Logger)

	// 3. Executor
	executor := NewActionExecutor(deps.Logger)

	// Register Universal Notification Handlers
	executor.RegisterHandler(NewNotificationHandler(deps.NotificationSvc, ""))
	executor.RegisterHandler(NewNotificationHandler(deps.NotificationSvc, notification.ChannelEmail))
	executor.RegisterHandler(NewNotificationHandler(deps.NotificationSvc, notification.ChannelSMS))
	executor.RegisterHandler(NewNotificationHandler(deps.NotificationSvc, notification.ChannelPush))

	// Register Other Handlers
	safeClient := NewSafeHTTPClient(30 * time.Second)
	executor.RegisterHandler(NewWebhookHandler(safeClient))
	executor.RegisterHandler(NewLogHandler(deps.Logger))
	executor.RegisterHandler(NewDelayHandler())

	// Register dynamic Domain Action Handlers
	executor.RegisterHandler(NewUpdateImpactMetricHandler(deps.ImpactSvc, deps.Logger))
	executor.RegisterHandler(NewCreateTaskHandler(deps.ProjectSvc, deps.Logger))
	executor.RegisterHandler(NewPostNewsFeedHandler(deps.ContentSvc, deps.Logger))
	executor.RegisterHandler(NewGenerateAIReportHandler(deps.ReportingSvc, deps.Logger))
	executor.RegisterHandler(NewSendPersonalNoteHandler(deps.NotificationSvc, deps.Logger))

	// 4. Ingest Service
	ingestSvc := NewIngestService(deps.Dispatcher)

	// 5. Subscription Manager (Dynamic Topics) & Subscriber
	subscriber := NewSubscriber(ingestSvc, deps.Logger)
	eventHandler := subscriber.handleAny // For dynamic subs

	subManager := NewSubscriptionManager(deps.EventsBus, repo, eventHandler, deps.Logger)

	// 6. Service
	svc := NewService(
		repo,
		evaluator,
		executor,
		deps.Logger,
		deps.EventsBus,
		deps.Dispatcher,
		deps.RateLimiter,
		statsBuffer,
		subManager,
	)

	// 7. Workers
	eventWorker := NewEventWorker(svc, deps.Logger)
	actionWorker := NewActionWorker(svc, deps.Logger)
	continuationWorker := NewContinuationWorker(svc, deps.Logger)

	// 8. Maintenance / Recovery
	recoverySvc := NewRecoveryService(repo, svc, deps.Logger)

	// 9. Start Background Processes (Stats Buffer)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			if err := statsBuffer.FlushSync(context.Background()); err != nil {
				deps.Logger.Error("Failed to flush stats buffer", err)
			}
		}
	}()

	return &Container{
		Service:             svc,
		EventIngestService:  ingestSvc,
		EventWorker:         eventWorker,
		ActionWorker:        actionWorker,
		ContinuationWorker:  continuationWorker,
		RecoveryService:     recoverySvc,
		SubscriptionManager: subManager,
		Subscriber:          subscriber,
	}, nil
}

// RegisterSubscribers implements events.EventSubscriber
func (c *Container) RegisterSubscribers(bus events.Bus) {
	// Delegate to the subscriber struct which has the list of core events
	c.Subscriber.RegisterSubscribers(bus)

	// Note: Dynamic subscriptions for user-defined triggers are handled
	// separately by the SubscriptionManager background process.
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "automation-service",
		DisplayName: "Automation & Workflows",
		Description: "Manage dynamic rules, event triggers, and automated workflows.",
		Icon:        "bolt",
		Events: []core.EventDefinition{
			{
				Key:         core.EventRuleTriggered,
				Label:       "Rule Triggered",
				Description: "Emitted when a rule successfully completes execution.",
				Schema:      map[string]string{"rule_id": "uuid", "execution_id": "uuid"},
			},
			{
				Key:         core.EventRuleCreated,
				Label:       "Rule Created",
				Description: "Emitted when a new automation rule is created.",
				Schema:      map[string]string{"rule_id": "uuid", "rule_name": "string"},
			},
			{
				Key:         core.EventRuleUpdated,
				Label:       "Rule Updated",
				Description: "Emitted when an existing rule configuration is modified.",
				Schema:      map[string]string{"rule_id": "uuid", "rule_name": "string"},
			},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         "automation:maintenance:prune",
				Label:       "Prune History",
				Description: "Delete execution logs older than retention period.",
				InputSchema: map[string]string{"older_than_days": "int"},
			},
			{
				Key:         "automation:maintenance:sync_subs",
				Label:       "Sync Subscriptions",
				Description: "Force re-sync of event bus subscriptions based on active rules.",
				InputSchema: map[string]string{},
			},
		},
		Permissions: []core.PermDefinition{
			{
				Key:         automation.PermAutomationRuleRead,
				Label:       "View Rules",
				Category:    "Automation",
				Description: "View automation rules and execution history.",
			},
			{
				Key:         automation.PermAutomationRuleManage,
				Label:       "Manage Rules",
				Category:    "Automation",
				Description: "Create, edit, and delete automation rules.",
				Dangerous:   true,
			},
			{
				Key:         automation.PermAutomationSystemOperate,
				Label:       "System Operations",
				Category:    "Automation",
				Description: "Retry executions and manage system settings.",
				Dangerous:   true,
			},
		},
	}
}

func RegisterPermissions(registrar rbac.PermissionRegistrar) {
	manifest := BuildManifest()
	for _, p := range manifest.Permissions {
		registrar.Register(rbac.PermissionDefinition{
			Name:        p.Key,
			Description: p.Description,
			Category:    p.Category,
		})
	}
}
