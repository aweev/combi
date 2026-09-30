package monitoring

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 404 Not Found
	ErrAlertRuleNotFound    = apperror.New(http.StatusNotFound, "ALERT_RULE_NOT_FOUND", "The specified alert rule could not be found.", nil)
	ErrAlertSilenceNotFound = apperror.New(http.StatusNotFound, "ALERT_SILENCE_NOT_FOUND", "The specified alert silence could not be found.", nil)

	// 409 Conflict
	ErrAlertRuleNameTaken = apperror.New(http.StatusConflict, "ALERT_RULE_NAME_TAKEN", "An alert rule with this name already exists.", nil)

	// 500 Internal (or 409 if dynamic registration is user-facing)
	// Since this usually happens during app startup or config, 500 is appropriate as it's a system state error.
	ErrMetricAlreadyRegistered = apperror.New(http.StatusInternalServerError, "METRIC_ALREADY_REGISTERED", "This metric has already been registered in the registry.", nil)
)

const (
	// JobEvaluateAlerts triggers the evaluation logic:
	// 1. Fetch active rules
	// 2. Query Prometheus (via an Adapter)
	// 3. Fire Alert if threshold met
	JobEvaluateAlerts core.JobType = "monitoring:evaluate_alerts"
)

// MetricLabel is a key for a metric's dimension.
type MetricLabel string

// MetricLabels is a map of key-value pairs that define the dimensions of a metric.
type MetricLabels map[MetricLabel]string

// --- Common Labels ---
const (
	LabelMethod    MetricLabel = "method"
	LabelPath      MetricLabel = "path"
	LabelCode      MetricLabel = "code"
	LabelQueue     MetricLabel = "queue"
	LabelJobType   MetricLabel = "job_type"
	LabelStatus    MetricLabel = "status"
	LabelDBName    MetricLabel = "db_name"
	LabelAlertName MetricLabel = "alert_name"
	LabelSeverity  MetricLabel = "severity"
	LabelState     MetricLabel = "state"
	LabelTopic     MetricLabel = "topic"
	LabelTransport MetricLabel = "transport"
	LabelError     MetricLabel = "error_type"
)

// --- HTTP API Metrics ---
const (
	MetricHTTPRequestsTotal          = "ngo_http_requests_total"
	MetricHTTPRequestDurationSeconds = "ngo_http_request_duration_seconds"
)

// --- Queue System Metrics ---
const (
	MetricJobsDispatchedTotal      = "ngo_queue_jobs_dispatched_total"
	MetricJobsProcessedTotal       = "ngo_queue_jobs_processed_total"
	MetricJobDurationSeconds       = "ngo_queue_job_duration_seconds"
	MetricQueueJobs                = "ngo_queue_jobs_gauge"
	MetricReaperJobsReclaimedTotal = "ngo_queue_reaper_jobs_reclaimed_total"
	MetricReaperJobsDLQTotal       = "ngo_queue_reaper_jobs_dlq_total"
	MetricSchedulerRunsTotal       = "ngo_queue_scheduler_runs_total"
	MetricReaperJobsReclaimed      = "ngo_queue_reaper_jobs_reclaimed"
	MetricReaperJobsDLQ            = "ngo_queue_reaper_jobs_dlq"

	MetricReaperJobsClaimedTotal = "ngo_queue_reaper_jobs_claimed_total"
	MetricAlertsEvaluatedTotal   = "ngo_alerting_evaluations_total"
	MetricAlertsSentTotal        = "ngo_alerting_notifications_sent_total"

	MetricMessagesDropped = "realtime_messages_dropped_total"

	// Event bus metrics
	MetricEventsPublishedTotal   = "events_published_total"
	MetricEventsReceivedTotal    = "events_received_total"
	MetricEventsProcessedTotal   = "events_processed_total"
	MetricRedisPublishErrors     = "redis_publish_errors_total"
	MetricRedisDeserializeErrors = "redis_deserialize_errors_total"
)

// Recorder is the core interface for instrumenting application code with metrics.
type Recorder interface {
	IncrementCounter(name string, labels MetricLabels)
	RecordHistogram(name string, value float64, labels MetricLabels)
	HTTPHandler() http.Handler
}

// --- Alerting Models ---
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

type AlertState string

const (
	StateInactive AlertState = "inactive"
	StatePending  AlertState = "pending"
	StateFiring   AlertState = "firing"
)

// AlertRule is the persistent definition of a condition that should trigger an alert.
type AlertRule struct {
	ID          uuid.UUID
	Name        string
	Description string
	IsEnabled   bool
	Severity    AlertSeverity
	PromQLQuery string
	ForDuration time.Duration
	Labels      map[string]interface{}
	Annotations map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Alert is a runtime instance of a triggered AlertRule.
type Alert struct {
	Fingerprint string
	RuleID      uuid.UUID
	State       AlertState
	Labels      map[string]interface{}
	Annotations map[string]string
	StartsAt    time.Time
	Value       string
}

// GenerateFingerprint creates a stable, unique identifier for an alert instance.
func (a *Alert) GenerateFingerprint() {
	stableLabels := make(map[string]interface{})
	for k, v := range a.Labels {
		if !strings.HasPrefix(k, "__") {
			stableLabels[k] = v
		}
	}
	keys := make([]string, 0, len(stableLabels))
	for k := range stableLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(a.RuleID.String())
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(`stableLabels[k]`)
	}
	hash := sha256.Sum256([]byte(b.String()))
	a.Fingerprint = hex.EncodeToString(hash[:])
}

// AlertSilence defines a period during which alerts are suppressed.
type AlertSilence struct {
	ID        uuid.UUID
	Matchers  map[string]interface{}
	StartsAt  time.Time
	EndsAt    time.Time
	Comment   string
	CreatedBy string
	CreatedAt time.Time
}

// Service is a composite interface for the entire monitoring system.
// It is used at the top level (e.g., in main.go) for initialization.
type Service interface {
	Recorder // Embeds the Recorder interface.
	RegisterCounterVec(name, help string, labelNames []string) error
	RegisterGaugeVec(name, help string, labelNames []string) error
	RegisterHistogramVec(name, help string, labelNames []string) error
	RegisterDBStatsCollector(db *sql.DB, dbName string) error
	RegisterDefaultMetrics() error
}

// AlertAdminService defines the contract for managing the alerting control plane.
// This is used by an admin UI or CLI to configure rules, silences, and routing.
type AlertAdminService interface {
	// Alert Rule Management
	CreateAlertRule(ctx context.Context, rule *AlertRule) (*AlertRule, error)
	UpdateAlertRule(ctx context.Context, rule *AlertRule) (*AlertRule, error)
	DeleteAlertRule(ctx context.Context, id uuid.UUID) error
	GetAlertRule(ctx context.Context, id uuid.UUID) (*AlertRule, error)
	ListAlertRules(ctx context.Context) ([]*AlertRule, error)

	// Silence Management
	CreateSilence(ctx context.Context, silence *AlertSilence) (*AlertSilence, error)
	DeleteSilence(ctx context.Context, id uuid.UUID) error
	GetSilence(ctx context.Context, id uuid.UUID) (*AlertSilence, error)
	ListSilences(ctx context.Context, activeOnly bool) ([]*AlertSilence, error)
}

// AlertRuleRepository defines the contract for persisting AlertRule entities.
type AlertRuleRepository interface {
	CreateRule(ctx context.Context, rule *AlertRule) error
	UpdateRule(ctx context.Context, rule *AlertRule) error
	DeleteRule(ctx context.Context, id uuid.UUID) error
	GetRule(ctx context.Context, id uuid.UUID) (*AlertRule, error)
	GetRuleByName(ctx context.Context, name string) (*AlertRule, error)
	ListRules(ctx context.Context) ([]*AlertRule, error)
}

// AlertSilenceRepository defines the contract for persisting AlertSilence entities.
type AlertSilenceRepository interface {
	CreateSilence(ctx context.Context, silence *AlertSilence) error
	DeleteSilence(ctx context.Context, id uuid.UUID) error
	GetSilence(ctx context.Context, id uuid.UUID) (*AlertSilence, error)
	ListActiveSilences(ctx context.Context, at time.Time) ([]*AlertSilence, error)
	ListAllSilences(ctx context.Context) ([]*AlertSilence, error)
}
