// audit-log-service — ported from api/internal/services/audit/store_worker.go.
package auditlog

import (
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// Statically verify that it implements the domain interface.
var _ queue.JobProcessor = (*StoreWorker)(nil)

// StoreWorker is the background job processor for persisting audit events.
// Its sole responsibility is to move an event from the queue to the database.
type StoreWorker struct {
	repo   audit.EventRepository
	logger log.Logger
}

// NewStoreWorker creates a new worker for persisting audit events.
func NewStoreWorker(repo audit.EventRepository, logger log.Logger) *StoreWorker {
	return &StoreWorker{
		repo:   repo,
		logger: logger.With(log.String("component", "audit_store_worker")),
	}
}

// JobType returns the specific job type this worker handles.
func (w *StoreWorker) JobType() core.JobType {
	return audit.JobAuditStoreEvent
}

// ProcessJob unmarshals the audit event from the payload and persists it.
func (w *StoreWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var event audit.Event
	if err := json.Unmarshal(job.Payload, &event); err != nil {
		w.logger.Error("failed to unmarshal audit event from job payload", err, log.String("job_id", job.ID.String()))
		// This is a permanent failure; the payload is malformed.
		return nil, queue.NewNonRetryableError(err)
	}

	// The repository should handle bulk insertion, but for a single job, we pass a slice of one.
	if err := w.repo.CreateBatchEvents(ctx, []*audit.Event{&event}); err != nil {
		w.logger.Error("failed to persist audit event to repository", err, log.String("event_id", event.ID.String()))
		// Database errors are potentially transient, so we return the raw error to allow retries.
		return nil, err
	}

	w.logger.Debug("successfully stored audit event", log.String("event_id", event.ID.String()))
	return nil, nil // No result for this job.
}
