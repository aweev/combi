// internal/services/impact/container.go
package impact

import (
	"context"
	_ "embed"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service       impact.Service
	RefreshWorker queue.JobProcessor
	EventWorker   *EventWorker
	Workers       []queue.JobProcessor
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
	EventBus  events.Bus
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Initialize Infrastructure & Repository
	repo := postgres.NewImpactRepository(deps.DB, deps.Logger)

	// 2. Initialize Core Domain Service
	svc := NewService(repo, deps.EventBus, deps.Logger)

	// 3. Initialize Workers
	refreshWorker := NewRefreshWorker(svc, deps.Logger)
	eventWorker := NewEventWorker(svc, repo, deps.Logger)

	// 4. System Registration
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	// 5. Database Seeding (Async to prevent blocking app startup)
	// We seed the metrics required for the Public Summary/Homepage
	go seedDefaults(svc, repo, deps.Logger)

	return &Container{
		Service:       svc,
		RefreshWorker: refreshWorker,
		EventWorker:   eventWorker,
		Workers:       []queue.JobProcessor{refreshWorker},
	}, nil
}

// RegisterSubscribers maps external system events to impact metrics
func (c *Container) RegisterSubscribers(bus events.Bus) {
	topics := []string{
		"donation.succeeded",     // Updates funding metrics
		"event.ended",            // Updates lives impacted
		"volunteer.hours_logged", // Updates volunteer metrics
		"project.activated",      // Updates active project count
	}

	for _, topic := range topics {
		bus.Subscribe(topic, c.EventWorker.HandleEvent)
	}
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	registrar.Register(rbac.PermissionDefinition{
		Name:        impact.PermManageConfig,
		Description: "Configure metric types, conversion mappings, and strategic goals.",
		Category:    "Impact",
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        impact.PermReadReports,
		Description: "Access impact dashboards and partner ROI reports.",
		Category:    "Impact",
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        impact.PermWriteEntry,
		Description: "Manually record impact data points for metrics that cannot be automated.",
		Category:    "Impact",
	})
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "impact-service",
		DisplayName: "Impact & ROI",
		Description: "Quantifies social value, tracks strategic goals, and generates attribution reports for partners.",
		Icon:        "chart-line",

		Events: []core.EventDefinition{
			{Key: core.EventImpactMetricUpdated, Label: "Metric Updated", Description: "Fired when a metric value changes."},
			{Key: core.EventGoalReached, Label: "Goal Achieved", Description: "Fired when a target goal is met."},
		},

		Jobs: []core.JobDefinition{
			{
				Key:         impact.JobRefreshGoals,
				Label:       "Goal Progress Refresh",
				Description: "Re-calculates progress for all active goals based on current ledger values.",
			},
		},

		Permissions: []core.PermDefinition{
			{Key: impact.PermManageConfig, Label: "Manage Config", Category: "Impact"},
			{Key: impact.PermReadReports, Label: "View Reports", Category: "Impact"},
			{Key: impact.PermWriteEntry, Label: "Data Entry", Category: "Impact"},
		},
	}
}

// seedDefaults ensures the metrics required by the Public Summary (Homepage) exist.
func seedDefaults(svc impact.Service, repo impact.Repository, logger log.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Helper to Get or Create Metric safely
	ensureMetric := func(def impact.MetricDefinition) *impact.MetricDefinition {
		existing, err := repo.GetMetricByKey(ctx, def.Key)
		if err == nil && existing != nil {
			return existing
		}

		created, err := svc.CreateMetric(ctx, def)
		if err != nil {
			logger.Error("Failed to seed impact metric", err, log.String("key", def.Key))
			return nil
		}
		return created
	}

	// 1. Standard Metrics for the Homepage Hero / Public Summary
	// These keys MUST match what service.GetPublicSummary() expects
	metricsToSeed := []impact.MetricDefinition{
		{
			Key:             "donations_total",
			Name:            "Total Funding",
			Unit:            "EUR",
			Type:            impact.MetricTypeCumulative,
			IsFundingSource: true,
		},
		{
			Key:  "lives_impacted",
			Name: "Lives Impacted",
			Unit: "people",
			Type: impact.MetricTypeCumulative,
		},
		{
			Key:  "active_projects",
			Name: "Active Projects",
			Unit: "projects",
			Type: impact.MetricTypeGauge,
		},
		{
			Key:  "countries_active",
			Name: "Operational Countries",
			Unit: "countries",
			Type: impact.MetricTypeGauge,
		},
	}

	results := make(map[string]*impact.MetricDefinition)
	for _, m := range metricsToSeed {
		if created := ensureMetric(m); created != nil {
			results[m.Key] = created
		}
	}

	// 2. Define Event-to-Impact Mappings
	// This automates the "Lives Impacted" metric when events end
	if livesMetric, ok := results["lives_impacted"]; ok {
		_ = repo.CreateMapping(ctx, impact.EventMapping{
			EventType:  "event.ended",
			MetricID:   livesMetric.ID,
			ValuePath:  "attendee_count", // Look for this key in event metadata
			Multiplier: 1.0,
		})
	}

	// This automates "Total Funding" when donations succeed
	if fundsMetric, ok := results["donations_total"]; ok {
		_ = repo.CreateMapping(ctx, impact.EventMapping{
			EventType:  "donation.succeeded",
			MetricID:   fundsMetric.ID,
			ValuePath:  "amount",
			Multiplier: 1.0,
		})
	}

	logger.Info("Impact seeding completed successfully")
}
