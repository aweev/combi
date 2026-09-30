package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	JobRepository
	FailedJobRepository
	ScheduleRepository
	DependencyRepository
	StatsRepository
	WorkerMetricsRepository
	MaintenanceRepository
	OutboxRepository
	JobEnqueuer
}

type JobRepository interface {
	CreateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id uuid.UUID) (*Job, error)
	UpdateJob(ctx context.Context, job *Job) error
	SearchJobs(ctx context.Context, filters JobSearchFilters) ([]*Job, int, error)
	DeleteJobsByQueue(ctx context.Context, queueName string) (int64, error)
}

type FailedJobRepository interface {
	CreateFailedJob(ctx context.Context, failedJob *FailedJob) error
	GetFailedJob(ctx context.Context, id uuid.UUID) (*FailedJob, error)
	ListFailedJobs(ctx context.Context, filters FailedJobFilters) ([]*FailedJob, int, error)
	DeleteFailedJob(ctx context.Context, id uuid.UUID) error
	DeleteFailedJobsByQueue(ctx context.Context, queueName string) (int64, error)
	BulkDeleteFailedJobs(ctx context.Context, filters FailedJobFilters) (affectedCount int64, err error)
	BulkRetryFailedJobs(ctx context.Context, filters FailedJobFilters) (newJobs []*Job, err error)
}

type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule *ScheduledJob) error
	UpdateSchedule(ctx context.Context, schedule *ScheduledJob) error
	DeleteSchedule(ctx context.Context, id uuid.UUID) error
	GetSchedule(ctx context.Context, id uuid.UUID) (*ScheduledJob, error)
	ListSchedules(ctx context.Context, limit, offset int) ([]*ScheduledJob, int, error)
	GetActiveSchedules(ctx context.Context) ([]*ScheduledJob, error)
}

type DependencyRepository interface {
	CreateJobWithDependencies(ctx context.Context, job *Job, dependencyIDs []uuid.UUID) error
	CheckAndUnblockDependents(ctx context.Context, completedJobID uuid.UUID) ([]*Job, error)
	GetJobDependencies(ctx context.Context, jobID uuid.UUID) ([]*Job, error)
	GetJobDependents(ctx context.Context, jobID uuid.UUID) ([]*Job, error)
}

type StatsRepository interface {
	GetAggregatedStats(ctx context.Context) (*DashboardStats, error)
	GetQueueStats(ctx context.Context) ([]QueueStats, error)
}

type WorkerMetricsRepository interface {
	RecordHeartbeat(ctx context.Context, metrics *WorkerMetrics) error
	GetActiveWorkers(ctx context.Context, heartbeatTimeout time.Duration) ([]*WorkerMetrics, error)
}

type StateRepository interface {
	PauseQueue(ctx context.Context, queueName string) error
	ResumeQueue(ctx context.Context, queueName string) error
	IsQueuePaused(ctx context.Context, queueName string) (bool, error)
}

type MaintenanceRepository interface {
	DeleteJobsOlderThan(ctx context.Context, status JobStatus, t time.Time) (int64, error)
	DeleteFailedJobsOlderThan(ctx context.Context, t time.Time) (int64, error)
	MaintainPartitions(ctx context.Context) error
}

type OutboxRepository interface {
	FetchAndLockOutboxEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)
	UpdateOutboxEvent(ctx context.Context, event *OutboxEvent) error
}

// ScheduleChangeNotifier is an OUTBOUND PORT for the Scheduler service.
// It defines a contract for receiving real-time notifications about schedule changes.
type ScheduleChangeNotifier interface {
	// Listen starts the background listening process. It must be safe to call multiple times.
	Listen(ctx context.Context) error
	// Notifications returns a read-only channel that emits the payload of each notification.
	Notifications() <-chan string
	// Close gracefully shuts down the listener and its underlying connection.
	Close() error
}

type JobQueue interface {
	Enqueue(ctx context.Context, jobID uuid.UUID, queueName string, priority int) error
	Dequeue(ctx context.Context, queues []string, blockTimeout time.Duration) (*Message, error)
	Acknowledge(ctx context.Context, msg *Message) error

	ListQueues(ctx context.Context) ([]string, error)
	ListStreamsForQueue(ctx context.Context, queueName string) ([]string, error)
	PurgeQueue(ctx context.Context, queueName string) (int, error)
	ClaimStalledMessages(ctx context.Context, streamKey string, minIdleTime time.Duration, consumerID string) ([]StalledMessage, error)
	MoveToDLQ(ctx context.Context, msg *Message, originalPayload json.RawMessage) error

	ListDLQMessages(ctx context.Context, limit int, cursor string) ([]DLQMessage, string, error)
	GetDLQMessage(ctx context.Context, messageID string) (*DLQMessage, error)
	DeleteDLQMessage(ctx context.Context, messageID string) error
	RequeueDLQMessage(ctx context.Context, messageID string) error
}

// JobEnqueuer is a simplified interface for placing a job directly into persistence (e.g., outbox).
// This is used by critical infrastructure like the Audit Recorder to break circular dependencies.
type JobEnqueuer interface {
	EnqueueJob(ctx context.Context, job *Job) error
}

// StalledMessage is a DTO representing a message claimed by the Reaper.
type StalledMessage struct {
	StreamKey     string
	MessageHandle string
	DeliveryCount int64 // How many times this message has been delivered
	IdleTime      time.Duration
	Payload       map[string]interface{} // The raw payload from the broker
}
