package impact

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// MetricType defines how values are aggregated.
type MetricType string

const (
	MetricTypeCumulative MetricType   = "CUMULATIVE" // Sum (e.g., Donations, Meals Served)
	MetricTypeGauge      MetricType   = "GAUGE"      // Last Value / Avg (e.g., Active Volunteers)
	JobRefreshGoals      core.JobType = "impact:refresh_goals"
)

const (
	PermManageConfig core.PermissionName = "impact:config:manage"
	PermWriteEntry   core.PermissionName = "impact:entry:write"
	PermReadReports  core.PermissionName = "impact:report:read"
)

// MetricDefinition defines WHAT we measure.
type MetricDefinition struct {
	ID              uuid.UUID  `db:"id"`
	Key             string     `db:"key"` // unique slug: "lives_touched"
	Name            string     `db:"name"`
	Unit            string     `db:"unit"` // "people", "kg", "USD"
	Type            MetricType `db:"type"`
	IsFundingSource bool       `db:"is_funding_source"`
	Description     string     `db:"description"`
	CreatedAt       time.Time  `db:"created_at"`
}

type EventMapping struct {
	ID         uuid.UUID `db:"id"`
	EventType  string    `db:"event_type"`
	MetricID   uuid.UUID `db:"metric_id"`
	ValuePath  string    `db:"value_path"` // e.g. "attendee_count"
	Multiplier float64   `db:"multiplier"`
}

// ImpactEntry is the ledger of value creation.
type ImpactEntry struct {
	ID         uuid.UUID `db:"id"`
	MetricID   uuid.UUID `db:"metric_id"`
	Value      float64   `db:"value"`
	OccurredAt time.Time `db:"occurred_at"`

	// Audit / Traceability
	SourceService string `db:"source_service"` // "event-service", "manual"
	SourceID      string `db:"source_id"`      // Event UUID, Project UUID

	// Attribution (Optional)
	ProjectID      *uuid.UUID `db:"project_id"`
	OrganizationID *uuid.UUID `db:"organization_id"` // For Corporate Partner attribution

	Metadata  map[string]any `db:"metadata"` // JSONB
	CreatedAt time.Time      `db:"created_at"`
}

// Goal is a target for a metric within a timeframe.
type Goal struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	MetricID    uuid.UUID `db:"metric_id"`
	TargetValue float64   `db:"target_value"`
	StartDate   time.Time `db:"start_date"`
	EndDate     time.Time `db:"end_date"`

	// Cached Calculation (updated via background worker)
	CurrentValue float64   `db:"current_value"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (g *Goal) Status() string {
	now := time.Now().UTC()
	if now.After(g.EndDate) {
		if g.CurrentValue >= g.TargetValue {
			return "COMPLETED"
		}
		return "MISSED"
	}

	// Simple projection logic
	totalDuration := g.EndDate.Sub(g.StartDate)
	elapsed := now.Sub(g.StartDate)
	expectedProgress := float64(elapsed) / float64(totalDuration)
	actualProgress := g.CurrentValue / g.TargetValue

	if actualProgress >= expectedProgress {
		return "ON_TRACK"
	}
	if actualProgress >= (expectedProgress * 0.8) {
		return "AT_RISK"
	}
	return "BEHIND"
}

// ROIReport represents the value delivered to a partner.
type ROIReport struct {
	PartnerName string
	TotalGiven  float64
	Currency    string
	Metrics     []ReportMetric
}

type ReportMetric struct {
	Name       string
	TotalValue float64
	Attributed float64 // The portion claimed by this partner
	Unit       string
}

type Service interface {
	// Configuration
	CreateMetric(ctx context.Context, def MetricDefinition) (*MetricDefinition, error)
	ListMetrics(ctx context.Context) ([]MetricDefinition, error)
	GetMetricByKey(ctx context.Context, key string) (*MetricDefinition, error)

	// Goals
	CreateGoal(ctx context.Context, goal Goal) (*Goal, error)
	CreateGoalFromRequest(ctx context.Context, params CreateGoalParams) (*Goal, error)
	GetGoals(ctx context.Context, activeOnly bool) ([]Goal, error)
	RefreshGoalProgress(ctx context.Context) error

	// Data
	RecordImpact(ctx context.Context, entry ImpactEntry) error
	RecordManualEntry(ctx context.Context, params ManualEntryParams) error

	// Analytics
	GenerateROI(ctx context.Context, orgID uuid.UUID, year int) (*ROIReport, error)
	GetAggregatedValue(ctx context.Context, metricKey string, start, end time.Time) (float64, error)

	// Public
	GetPublicSummary(ctx context.Context) (*PublicSummary, error)
}

type PublicSummary struct {
	LivesImpacted   int       `json:"lives_impacted"`
	ActiveProjects  int       `json:"active_projects"`
	CountriesActive int       `json:"countries_active"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Repository interface {
	// Metrics
	CreateMetric(ctx context.Context, def *MetricDefinition) error
	GetMetricByKey(ctx context.Context, key string) (*MetricDefinition, error)
	GetMetricByID(ctx context.Context, id uuid.UUID) (*MetricDefinition, error)
	ListMetrics(ctx context.Context) ([]MetricDefinition, error)

	// Goals
	CreateGoal(ctx context.Context, goal *Goal) error
	ListGoals(ctx context.Context, activeOnly bool) ([]Goal, error)
	UpdateGoalProgress(ctx context.Context, goalID uuid.UUID, currentVal float64) error

	// Entries
	InsertEntry(ctx context.Context, entry *ImpactEntry) error
	GetFundingMetric(ctx context.Context) (*MetricDefinition, error)

	CreateMapping(ctx context.Context, m EventMapping) error
	GetMappingsForEvent(ctx context.Context, eventType string) ([]EventMapping, error)

	// Analytics
	SumMetric(ctx context.Context, metricID uuid.UUID, start, end time.Time) (float64, error)
	SumMetricForOrg(ctx context.Context, metricID, orgID uuid.UUID, start, end time.Time) (float64, error)
}

type CreateGoalParams struct {
	Name        string
	MetricKey   string
	TargetValue float64
	StartDate   time.Time
	EndDate     time.Time
}

type ManualEntryParams struct {
	MetricKey  string
	Value      float64
	OccurredAt time.Time
	ActorID    uuid.UUID
	Metadata   map[string]any
}

// MetricsUpdatedPayload contains the new total for a specific metric.
// Used to push live updates to the frontend dashboard.
type MetricsUpdatedPayload struct {
	MetricKey    string    `json:"metric_key"`  // "lives_touched"
	MetricName   string    `json:"metric_name"` // "Lives Touched"
	NewValue     float64   `json:"new_value"`
	Unit         string    `json:"unit"`
	CalculatedAt time.Time `json:"calculated_at"`
}
