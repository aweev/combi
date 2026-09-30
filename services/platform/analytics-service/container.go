package analytics

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service             analytics.Service
	AnonymizationWorker queue.JobProcessor
	AggregationWorker   queue.JobProcessor
	PartitionWorker     queue.JobProcessor
	Subscriber          *Subscriber
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
	Config    config.AutomationConfig
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	// 2. Infra
	repo := postgres.NewAnalyticsRepository(deps.DB, deps.Logger)
	svc := NewService(repo, deps.Logger)

	// 3. Workers
	// Secret key for hashing PII during anonymization
	anonymizer := NewAnonymizationWorker(deps.DB, deps.Logger, deps.Config.AnonymizationSecret)
	aggregator := NewAggregationWorker(repo, deps.Logger)
	partitioner := NewPartitionWorker(deps.DB, deps.Logger)

	// 4. Subscriber
	sub := NewSubscriber(svc, deps.Logger)

	return &Container{
		Service:             svc,
		AnonymizationWorker: anonymizer,
		AggregationWorker:   aggregator,
		PartitionWorker:     partitioner,
		Subscriber:          sub,
	}, nil
}

// RegisterSubscribers wires the high-volume topics we care about.
func (c *Container) RegisterSubscribers(bus events.Bus) {
	topics := []string{
		core.EventUserCreated.String(),
		core.EventUserLoggedIn.String(),
		core.EventDonationReceived.String(),
		core.EventContentPublished.String(),
		// We can add "form.submission", "opportunity.created" etc. as needed
	}

	for _, t := range topics {
		bus.Subscribe(t, c.Subscriber.Handle)
	}
}

func registerPermissions(r rbac.PermissionRegistrar) {
	r.Register(rbac.PermissionDefinition{
		Name:        analytics.PermViewDashboard,
		Description: "Can view system analytics dashboards.",
		Category:    "Analytics",
	})
	r.Register(rbac.PermissionDefinition{
		Name:        analytics.PermExportData,
		Description: "Can export raw analytics data.",
		Category:    "Analytics",
	})
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "analytics-service",
		DisplayName: "Data Intelligence",
		Description: "Privacy-first tracking, anonymization, and reporting.",
		Version:     "1.0.0",
		Tier:        "capability",
		Owner:       "platform",
		Portable:    true,
		Interfaces: []core.InterfaceDefinition{
			{
				Protocol:    "grpc",
				Package:     "platform.analytics.v1",
				SpecPath:    "api/proto/analytics/v1/analytics.proto",
				Stability:   "stable",
				Description: "Preferred inter-service API for event ingestion and report queries.",
				Consumers:   []string{"reporting-service", "monitoring-service", "automation-service"},
			},
			{
				Protocol:    "rest",
				Endpoint:    "/api/v1/analytics",
				SpecPath:    "internal/services/analytics/api/openapi.yaml",
				Stability:   "stable",
				Description: "Public ingestion and admin reporting endpoints.",
			},
			{
				Protocol:    "graphql",
				Endpoint:    "/api/v1/graphql",
				Stability:   "stable",
				Description: "Read-optimized analytics fields for dashboard composition.",
			},
		},
		Dependencies: []core.DependencyDefinition{
			{Name: "postgres", Type: "postgres", Required: true, ConfigKey: "database", Description: "Partitioned raw events and rollup tables."},
			{Name: "queue", Type: "queue", Required: true, ConfigKey: "queue", Description: "Aggregation, anonymization, and partition maintenance jobs."},
			{Name: "event-bus", Type: "event-bus", Required: true, ConfigKey: "eventBus", Description: "Domain event subscriber for automatic tracking."},
			{Name: "redis", Type: "redis", Required: false, ConfigKey: "redis", Description: "Optional cache for expensive dashboard queries."},
		},
		Telemetry: core.TelemetryDefinition{
			MetricsPrefix: "analytics",
			TraceName:     "analytics-service",
			LogFields:     []string{"event_type", "org_id", "session_id", "batch_size"},
			Dashboards:    []string{"analytics-ingestion", "analytics-rollups", "analytics-privacy"},
			Alerts:        []string{"analytics_ingest_dropped_events", "analytics_rollup_lag", "analytics_anonymization_failed"},
		},
		Events: []core.EventDefinition{}, // Pure consumer
		Jobs: []core.JobDefinition{
			{
				Key:         analytics.JobAnalyticsAggregate,
				Label:       "Refresh Dashboards",
				Description: "Re-calculates daily statistics.",
			},
			{
				Key:         analytics.JobAnonymizeOldData,
				Label:       "Purge PII",
				Description: "Runs GDPR anonymization script.",
			},
			{
				Key:         analytics.JobMaintainPartitions,
				Label:       "Maintenance: Partitions",
				Description: "Ensures database partitions exist.",
			},
		},
		Permissions: []core.PermDefinition{
			{Key: analytics.PermViewDashboard, Label: "View Dashboards", Category: "Reporting"},
			{Key: analytics.PermExportData, Label: "Export Data", Category: "Reporting"},
		},
	}
}
