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

// Statically verify implementation.
var _ queue.JobProcessor = (*DeleteFileWorker)(nil)

// DeleteFileWorker consumes `storage:delete_file` jobs for permanent deletion.
type DeleteFileWorker struct {
	storageSvc storage.Service
	logger     log.Logger
}

func NewDeleteFileWorker(storageSvc storage.Service, logger log.Logger) *DeleteFileWorker {
	return &DeleteFileWorker{
		storageSvc: storageSvc,
		logger:     logger.With(log.String("worker", string(storage.JobStorageDeleteFile))),
	}
}

func (w *DeleteFileWorker) JobType() core.JobType {
	return storage.JobStorageDeleteFile
}

// ProcessJob is the entry point.
func (w *DeleteFileWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid delete_file payload: %w", err))
	}
	fileID, err := uuid.Parse(payload.FileID)
	if err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid file_id in payload: %w", err))
	}

	// The worker's job is simply to call the service method.
	// The service handles all logic, including authorization (though in this case it's a system action) and auditing.
	err = w.storageSvc.DeleteFile(ctx, fileID, true) // permanent = true
	if err != nil {
		w.logger.Error("Failed to permanently delete file", err, log.String("file_id", fileID.String()))
		return nil, err // Return original error to allow for retries
	}

	w.logger.Info("Successfully deleted file via anonymization job", log.String("file_id", fileID.String()))
	return nil, nil
}
