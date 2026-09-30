package analytics

import (
	"context"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 400 Bad Request
	ErrInvalidEvent = apperror.New(http.StatusBadRequest, "INVALID_EVENT", "The analytics event is malformed.", nil)
)

const (
	// Used by the Seeder
	JobAnalyticsAggregate core.JobType = "analytics:aggregate"

	// JobFlushAnalyticsBatch takes a list of events from Redis and inserts them into the DB.
	JobFlushAnalyticsBatch core.JobType = "analytics:flush_batch"

	// JobAnonymizeOldData runs weekly to hash IPs and delete raw logs older than X days (GDPR).
	JobAnonymizeOldData core.JobType = "analytics:anonymize"

	// JobMaintainPartitions runs once a day to ensure the partitions are up-to-date.
	JobMaintainPartitions core.JobType = "analytics:maintain_partitions"

	// Permissions
	PermViewDashboard = core.PermissionName("analytics:view_dashboard")
	PermExportData    = core.PermissionName("analytics:export_data")
)

// AnalyticsEvent - Enhanced with parsed fields
type AnalyticsEvent struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    *uuid.UUID `json:"user_id" db:"user_id"`
	SessionID string     `json:"session_id" db:"session_id"`
	EventType string     `json:"event_type" db:"event_type"`
	Path      string     `json:"path" db:"path"`
	Referrer  string     `json:"referrer" db:"referrer"`

	// Raw Technical Data
	UserAgent string `json:"user_agent" db:"user_agent"`
	IPAddress string `json:"ip_address" db:"ip_address"`

	// Enriched Data (The Magic)
	DeviceType string `json:"device_type" db:"device_type"` // mobile, desktop, tablet, bot
	Country    string `json:"country" db:"country"`         // ISO Code (DE, US)

	Properties map[string]any `json:"properties" db:"properties"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
}

// JobEvent captures internal background job metrics for the analytics pipeline.
type JobEvent struct {
	JobID          uuid.UUID `json:"job_id"`
	JobType        string    `json:"job_type"`
	Queue          string    `json:"queue"`
	FinalStatus    string    `json:"final_status"` // STARTED, COMPLETED, FAILED
	Attempts       int       `json:"attempts"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	AvailableAt    time.Time `json:"available_at"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	WaitDurationMs int64     `json:"wait_duration_ms"` // StartedAt - AvailableAt
	RunDurationMs  int64     `json:"run_duration_ms"`  // FinishedAt - StartedAt
	WorkerID       *string   `json:"worker_id,omitempty"`
	WorkerHost     *string   `json:"worker_host,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
}

// Metric represents a single data point (Time Series)
type Metric struct {
	ID         string            `db:"id"`
	EventName  string            `db:"event_name"` // e.g., "donation.received"
	Value      float64           `db:"value"`      // e.g., 50.00
	Dimensions map[string]string `db:"dimensions"` // JSONB: {"country": "DE", "campaign": "winter23"}
	Timestamp  time.Time         `db:"timestamp"`
}

// Aggregate represents a rolled-up view (for fast charts)
type Aggregate struct {
	Period time.Time `json:"period"` // Daily bucket
	Metric string    `json:"metric"` // "donation_total"
	Value  float64   `json:"value"`
}

type TimeSeriesPoint struct {
	Time  time.Time `json:"time" db:"bucket"`
	Value float64   `json:"value" db:"count"`
}

// FunnelStep represents one stage in a conversion process.
type FunnelStep struct {
	Name      string `json:"name"`
	EventType string `json:"event_type"`
	Count     int    `json:"count"`
	Dropoff   int    `json:"dropoff"`
}

type Service interface {
	// Ingestion
	TrackEvent(ctx context.Context, e AnalyticsEvent) error

	// Links a previous anonymous session to a user
	Alias(ctx context.Context, sessionID string, userID uuid.UUID) error

	// Reporting
	GetDailyActiveUsers(ctx context.Context, start, end time.Time) ([]TimeSeriesPoint, error)
	GetPageViews(ctx context.Context, path string, start, end time.Time) (int, error)
	GetConversionFunnel(ctx context.Context, steps []string, start, end time.Time) ([]FunnelStep, error)
	GetTopFeatures(ctx context.Context, limit int) ([]string, error)

	// Lifecycle
	Shutdown(ctx context.Context) error
}

// Repository defines the persistence contract.
// This is often implemented by a specialized OLAP DB (ClickHouse/TimescaleDB)
// but can start with Postgres.
type Repository interface {
	InsertEvent(ctx context.Context, e *AnalyticsEvent) error
	InsertBatch(ctx context.Context, events []AnalyticsEvent) error

	LinkAnonymousSession(ctx context.Context, sessionID string, userID uuid.UUID) error

	// Aggregations
	CountUniqueUsers(ctx context.Context, start, end time.Time) ([]TimeSeriesPoint, error)
	CountEventsByType(ctx context.Context, eventType string, start, end time.Time) (int, error)
	GetFunnelCounts(ctx context.Context, steps []string, start, end time.Time) (map[string]int, error)
	GetTopFeatures(ctx context.Context, limit int) ([]string, error)
	AggregateStats(ctx context.Context, date time.Time) error
}
