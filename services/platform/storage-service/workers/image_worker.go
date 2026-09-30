package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Statically verify that ImageWorker implements the JobProcessor interface.
var _ queue.JobProcessor = (*ImageWorker)(nil)

// ImageWorker consumes `storage:process_image` jobs.
type ImageWorker struct {
	svc    storage.WorkerService
	logger log.Logger
}

func NewImageWorker(svc storage.WorkerService, logger log.Logger) *ImageWorker {
	return &ImageWorker{
		svc:    svc,
		logger: logger.With(log.String("worker", string(storage.JobStorageProcessImage))),
	}
}

// JobType returns the canonical job type this worker handles.
func (w *ImageWorker) JobType() core.JobType {
	return storage.JobStorageProcessImage
}

type imagePayload struct {
	FileID   uuid.UUID         `json:"file_id"`
	Variants map[string]string `json:"variants"`
}

// ProcessJob unpacks the job payload and calls the underlying worker service.
func (w *ImageWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p imagePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid image processing payload: %w", err))
	}
	if p.FileID == uuid.Nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("payload missing required 'file_id'"))
	}

	// Delegate the complex logic to the worker service
	err := w.svc.ProcessImage(ctx, p.FileID, p.Variants)
	if err != nil {
		w.logger.Warn("Image processing job failed", log.ErrorAttr(err), log.String("job_id", job.ID.String()))
		return nil, err // The service decides if the error is retryable
	}
	return nil, nil
}
