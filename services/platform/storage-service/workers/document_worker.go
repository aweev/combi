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

// Statically verify that DocumentWorker implements the JobProcessor interface.
var _ queue.JobProcessor = (*DocumentWorker)(nil)

// DocumentWorker consumes `storage:process_asset` jobs.
type DocumentWorker struct {
	svc    storage.WorkerService
	logger log.Logger
}

func NewDocumentWorker(svc storage.WorkerService, logger log.Logger) *DocumentWorker {
	return &DocumentWorker{
		svc:    svc,
		logger: logger.With(log.String("worker", string(storage.JobStorageProcessAsset))),
	}
}

// JobType returns the canonical job type this worker handles.
func (w *DocumentWorker) JobType() core.JobType {
	return storage.JobStorageProcessAsset
}

type documentPayload struct {
	FileID     uuid.UUID `json:"file_id"`
	Operations []string  `json:"operations"`
}

// ProcessJob unpacks the job payload and calls the underlying worker service.
func (w *DocumentWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p documentPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid document processing payload: %w", err))
	}
	if p.FileID == uuid.Nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("payload missing required 'file_id'"))
	}

	err := w.svc.ProcessDocument(ctx, p.FileID, p.Operations)
	if err != nil {
		w.logger.Warn("Document processing job failed", log.ErrorAttr(err), log.String("job_id", job.ID.String()))
		return nil, err
	}
	return nil, nil
}
