// internal/domain/queue/models.go
package queue

import (
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// --- Enums ---

type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"    // Ready to be picked up.
	StatusProcessing JobStatus = "PROCESSING" // Actively being worked on by a worker.
	StatusCompleted  JobStatus = "COMPLETED"  // Successfully finished.
	StatusFailed     JobStatus = "FAILED"     // Permanently failed after all retries.
	StatusCancelled  JobStatus = "CANCELLED"  // Cancelled by a user/admin.
	StatusDelayed    JobStatus = "DELAYED"    // Scheduled for a future time.
	StatusBlocked    JobStatus = "BLOCKED"    // Waiting for dependencies to complete.
)

func (s JobStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// --- Core Models ---

// Job represents a single unit of work to be performed in the background.
type Job struct {
	ID          uuid.UUID       `json:"id"`
	Type        core.JobType    `json:"type"`     // e.g., "notification.send_email", "report.generate_pdf"
	Queue       string          `json:"queue"`    // e.g., "default", "critical", "low"
	Priority    int             `json:"priority"` // Lower is higher priority (e.g., 0 is highest)
	Payload     json.RawMessage `json:"payload"`
	Status      JobStatus       `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	Timeout     time.Duration   `json:"timeout"`            // How long a worker has to process this job
	AvailableAt time.Time       `json:"available_at"`       // When the job can be processed (for delayed jobs)
	Error       *string         `json:"error"`              // The last error message if it failed
	Result      json.RawMessage `json:"result,omitempty"`   // Optional result data upon completion
	Tags        []string        `json:"tags,omitempty"`     // For searching and categorization
	Metadata    json.RawMessage `json:"metadata,omitempty"` // For internal data like dependency tracking
	ParentJobID *uuid.UUID      `json:"parent_job_id,omitempty"`
	WorkflowID  *uuid.UUID      `json:"workflow_id,omitempty"`
	ActorID     *uuid.UUID      `json:"actor_id,omitempty"` // Nullable: system jobs might not have a user actor
	ActorType   string          `json:"actor_type"`         // e.g., "user", "system"
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// FailedJob is an immutable record of a job that has permanently failed.
type FailedJob struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	JobID      uuid.UUID       `json:"job_id" db:"original_job_id"` // <-- ADD THIS DB TAG
	Type       core.JobType    `json:"type" db:"type"`
	Queue      string          `json:"queue" db:"queue"`
	Payload    json.RawMessage `json:"payload" db:"payload"`
	Exception  string          `json:"exception" db:"exception"`
	StackTrace *string         `json:"stack_trace,omitempty" db:"stack_trace"`
	Attempts   int             `json:"attempts" db:"attempts"`
	WorkerID   *string         `json:"worker_id,omitempty" db:"worker_id"`
	WorkerHost *string         `json:"worker_host,omitempty" db:"worker_host"`
	FailedAt   time.Time       `json:"failed_at" db:"failed_at"`
}

// ScheduledJob defines a recurring job based on a cron expression.
type ScheduledJob struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"` // Must be unique
	Description    string          `json:"description"`
	Type           core.JobType    `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	Queue          string          `json:"queue"`
	CronExpression string          `json:"cron_expression"`
	Timezone       string          `json:"timezone"` // e.g., "UTC", "America/New_York"
	IsActive       bool            `json:"is_active"`
	LastError      *string         `json:"last_error,omitempty"`
	LastRunAt      *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt      *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// DLQMessage represents a message that has been moved to the Dead-Letter Queue.
type DLQMessage struct {
	MessageID         string                 `json:"message_id"`
	OriginalStream    string                 `json:"original_stream"`
	OriginalMessageID string                 `json:"original_message_id"`
	MovedAt           time.Time              `json:"moved_at"`
	Payload           map[string]interface{} `json:"payload"`
}

type OutboxEventStatus string

const (
	OutboxStatusPending   OutboxEventStatus = "PENDING"
	OutboxStatusProcessed OutboxEventStatus = "PROCESSED"
	OutboxStatusFailed    OutboxEventStatus = "FAILED"
)

// OutboxEvent is a record stored in the main database to be reliably published to the broker.
// This is created in the same transaction as the primary business logic (e.g., creating a user).
type OutboxEvent struct {
	ID              uuid.UUID         `db:"id"`
	JobID           uuid.UUID         `db:"job_id"`
	QueueName       string            `db:"queue_name"`
	Priority        int               `db:"priority"`
	Status          OutboxEventStatus `db:"status"`
	Attempts        int               `db:"attempts"`
	LastAttemptAt   *time.Time        `db:"last_attempt_at"`
	ProcessingError *string           `db:"processing_error"`
	CreatedAt       time.Time         `db:"created_at"`
	ProcessedAt     *time.Time        `db:"processed_at"`
}

type Message struct {
	// JobID is the canonical identifier for the job payload.
	JobID uuid.UUID

	// Handle is the broker-specific identifier for this message instance (e.g., a Redis Stream ID).
	// It is required for acknowledging the message.
	Handle string

	// Source is the broker-specific identifier for the queue/stream the message came from.
	Source string
}
