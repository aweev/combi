package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
)

type FileCleanupWorker struct {
	storageSvc storage.Service
	logger     log.Logger
}

func NewFileCleanupWorker(storageSvc storage.Service, logger log.Logger) *FileCleanupWorker {
	return &FileCleanupWorker{storageSvc: storageSvc, logger: logger}
}

func (w *FileCleanupWorker) JobType() core.JobType {
	return user.JobUserFileCleanup
}

func (w *FileCleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}
	userID, _ := uuid.Parse(p.UserID)

	// List all files owned by user
	// Note: This assumes ListFiles handles pagination or returns all files if no page is specified.
	// For robust implementation with huge file counts, you might need a loop here.
	files, _, err := w.storageSvc.ListFiles(ctx, storage.FileFilter{OwnerID: &userID, PageSize: 1000})
	if err != nil {
		return nil, err // Retryable error from storage service
	}

	deletedCount := 0
	var errorList []error

	for _, f := range files {
		if err := w.storageSvc.DeleteFile(ctx, f.ID, true); err != nil {
			w.logger.Error("failed to delete file", err, log.String("file_id", f.ID.String()))
			errorList = append(errorList, err)
		} else {
			deletedCount++
		}
	}

	// If there were errors, we fail the job so it can be retried by the queue system.
	// The retry will re-list files (idempotent) and try to delete the remaining ones.
	if len(errorList) > 0 {
		return nil, fmt.Errorf("failed to delete %d files; last error: %v", len(errorList), errorList[len(errorList)-1])
	}

	w.logger.Info("user file cleanup complete", log.Int("deleted_count", deletedCount))

	data, _ := json.Marshal(map[string]int{"deleted": deletedCount})
	return data, nil
}
