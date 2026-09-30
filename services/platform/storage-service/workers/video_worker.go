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

// Statically verify that VideoWorker implements the JobProcessor interface.
var _ queue.JobProcessor = (*VideoWorker)(nil)

// VideoWorker consumes `storage:process_video` jobs.
type VideoWorker struct {
	svc    storage.WorkerService
	logger log.Logger
}

func NewVideoWorker(svc storage.WorkerService, logger log.Logger) *VideoWorker {
	return &VideoWorker{
		svc:    svc,
		logger: logger.With(log.String("worker", string(storage.JobStorageProcessVideo))),
	}
}

// JobType returns the canonical job type this worker handles.
func (w *VideoWorker) JobType() core.JobType {
	return storage.JobStorageProcessVideo
}

type videoPayload struct {
	FileID    uuid.UUID `json:"file_id"`
	Formats   []string  `json:"formats"`
	Qualities []string  `json:"qualities"`
}

// ProcessJob unpacks the job payload and calls the underlying worker service.
func (w *VideoWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p videoPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid video processing payload: %w", err))
	}
	if p.FileID == uuid.Nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("payload missing required 'file_id'"))
	}

	err := w.svc.ProcessVideo(ctx, p.FileID, p.Formats, p.Qualities)
	if err != nil {
		w.logger.Warn("Video processing job failed", log.ErrorAttr(err), log.String("job_id", job.ID.String()))
		return nil, err
	}
	return nil, nil
}
