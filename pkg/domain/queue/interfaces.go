// internal/domain/queue/interfaces.go
package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// Dispatcher is the simple, high-level service for creating jobs.
// This is the primary interface that other application domains will depend on.
type Dispatcher interface {
	Dispatch(ctx context.Context, jobType core.JobType, payload json.RawMessage, opts ...DispatchOption) (*Job, error)
}

// Service defines the administrative and operational contract for managing the queue system.
// This is used by UIs, CLIs, and monitoring tools.
type Service interface {
	// Job Management
	CreateJob(ctx context.Context, params CreateJobParams) (*Job, error)
	GetJob(ctx context.Context, jobID uuid.UUID) (*Job, error)
	CancelJob(ctx context.Context, id uuid.UUID) (*Job, error)
	SearchJobs(ctx context.Context, filters JobSearchFilters) ([]*Job, int, error)
	GetJobDependencies(ctx context.Context, jobID uuid.UUID) ([]*Job, error)
	GetJobDependents(ctx context.Context, jobID uuid.UUID) ([]*Job, error)

	// Failed Job Management
	GetFailedJob(ctx context.Context, failedJobID uuid.UUID) (*FailedJob, error)
	ListFailedJobs(ctx context.Context, filters FailedJobFilters) ([]*FailedJob, int, error)
	RetryFailedJob(ctx context.Context, failedJobID uuid.UUID) (*Job, error)
	DeleteFailedJob(ctx context.Context, failedJobID uuid.UUID) error
	BulkRetryFailedJobs(ctx context.Context, filters FailedJobFilters) (*BulkOperationResult, error)
	BulkDeleteFailedJobs(ctx context.Context, filters FailedJobFilters) (*BulkOperationResult, error)

	// Schedule Management
	CreateSchedule(ctx context.Context, schedule *ScheduledJob) (*ScheduledJob, error)
	GetSchedule(ctx context.Context, scheduleID uuid.UUID) (*ScheduledJob, error)
	ListSchedules(ctx context.Context, limit, offset int) ([]*ScheduledJob, int, error)
	UpdateSchedule(ctx context.Context, scheduleID uuid.UUID, params UpdateScheduleParams) (*ScheduledJob, error)
	DeleteSchedule(ctx context.Context, scheduleID uuid.UUID) error
	TriggerSchedule(ctx context.Context, scheduleID uuid.UUID) (*Job, error)

	// System & Queue Operations
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)
	GetQueueStats(ctx context.Context) ([]QueueStats, error)
	ListQueues(ctx context.Context) ([]string, error)
	PauseQueue(ctx context.Context, queueName string) error
	ResumeQueue(ctx context.Context, queueName string) error
	GetActiveWorkers(ctx context.Context, heartbeatTimeout time.Duration) ([]*WorkerMetrics, error)
	PurgeQueue(ctx context.Context, queueName string) (deletedCount int, err error)

	// DLQ Management
	ListDLQMessages(ctx context.Context, limit int, cursor string) ([]DLQMessage, string, error)
	GetDLQMessage(ctx context.Context, messageID string) (*DLQMessage, error)
	DeleteDLQMessage(ctx context.Context, messageID string) error
	RequeueDLQMessage(ctx context.Context, messageID string) error
}

// QueueMaintenanceService handles internal, long-running queue operations.
type QueueMaintenanceService interface {
	CleanupOldJobs(ctx context.Context, olderThan time.Duration) (int64, error)
	CleanupFailedJobs(ctx context.Context, olderThan time.Duration) (int64, error)
}

// --- Infrastructure Interfaces (Runtime Components) ---

// JobProcessor is the interface that all background workers must implement.
type JobProcessor interface {
	JobType() core.JobType
	ProcessJob(ctx context.Context, job *Job) (result json.RawMessage, err error)
}

// WorkerPool is the engine that runs job processors.
type WorkerPool interface {
	RegisterWorker(worker JobProcessor) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Scheduler is the component that handles cron-based job scheduling.
type Scheduler interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type WorkflowManager interface {
	OnJobCompletion(ctx context.Context, completedJobID uuid.UUID)
}
type BackoffStrategy interface {
	Calculate(attempt int) time.Duration
}

// WorkerRegistryReader defines the contract for reading from the worker registry.
// The WorkerPool only needs to read from it.
type WorkerRegistryReader interface {
	Get(jobType core.JobType) (JobProcessor, error)
	RegisteredJobTypes() []core.JobType
	Register(worker JobProcessor) error
}

// JobLifecycleManager defines the contract for handling the outcome of a job's execution.
type JobLifecycleManager interface {
	HandleSuccess(ctx context.Context, job *Job, result json.RawMessage)
	HandleFailure(ctx context.Context, job *Job, processErr error, workerID, workerHost string)
}

// --- DTOs for Service Methods ---

type CreateJobParams struct {
	Type          core.JobType
	Queue         string
	Priority      int
	Payload       json.RawMessage
	MaxAttempts   int
	Timeout       time.Duration
	Tags          []string
	DependencyIDs []uuid.UUID
}

type UpdateScheduleParams struct {
	Name           *string
	Description    *string
	Type           *core.JobType
	Payload        json.RawMessage
	Queue          *string
	CronExpression *string
	Timezone       *string
	IsActive       *bool
}

type JobSearchFilters struct {
	Queue     *string
	Type      *core.JobType
	Status    *JobStatus
	Tags      []string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

type FailedJobFilters struct {
	Queue     *string
	Type      *string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}

type QueueStats struct {
	Name       string `json:"name"`
	Pending    int64  `json:"pending"`
	Processing int64  `json:"processing"`
	Failed     int64  `json:"failed"`
	Delayed    int64  `json:"delayed"`
}

type WorkerMetrics struct {
	ID            string    `json:"id"`
	Host          string    `json:"host"`
	Queues        []string  `json:"queues"`
	Status        string    `json:"status"` // "ACTIVE", "IDLE", "STOPPED"
	JobsProcessed int64     `json:"jobs_processed"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	StartedAt     time.Time `json:"started_at"`
}

type BulkOperationResult struct {
	TotalConsidered int      `json:"total_considered"`
	TotalAffected   int      `json:"total_affected"`
	SuccessCount    int      `json:"success_count"`
	FailureCount    int      `json:"failure_count"`
	DurationMs      int64    `json:"duration_ms"`
	Errors          []string `json:"errors,omitempty"`
}

type DashboardStats struct {
	TotalPending    int64 `json:"total_pending" db:"total_pending"`
	TotalProcessing int64 `json:"total_processing" db:"total_processing"`
	TotalFailed     int64 `json:"total_failed" db:"total_failed"`
	TotalSchedules  int   `json:"total_schedules" db:"total_schedules"`
	ActiveWorkers   int   `json:"active_workers"` // This one is calculated in Go, so no db tag is needed.
}
