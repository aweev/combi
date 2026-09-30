package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/notification/processor"
)

var _ queue.JobProcessor = (*Worker)(nil)

// Worker is the queue worker for the `notification:send` job type.
// It acts as a simple bridge, delegating all complex logic to the Processor.
type Worker struct {
	processor processor.Processor
}

func NewWorker(processor processor.Processor) *Worker {
	return &Worker{processor: processor}
}

// JobType returns the canonical job type this worker handles.
func (w *Worker) JobType() core.JobType {
	return notification.JobSendNotification
}

// ProcessJob unpacks the notification ID and passes it to the processor.
func (w *Worker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p struct {
		NotificationID string `json:"notification_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload: %w", err))
	}
	if _, err := uuid.Parse(p.NotificationID); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid notification_id in payload: %w", err))
	}

	// Delegate the entire complex pipeline to the processor.
	// The processor's returned error (retryable or not) is passed directly to the queue system.
	err := w.processor.Process(ctx, p.NotificationID)

	return nil, err
}
