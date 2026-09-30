package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Statically verify that CleanupUploadsWorker implements the JobProcessor interface.
var _ queue.JobProcessor = (*CleanupUploadsWorker)(nil)

// CleanupUploadsWorker consumes `storage:cleanup_expired_uploads` jobs.
type CleanupUploadsWorker struct {
	repo   storage.Repository
	logger log.Logger
}

func NewCleanupUploadsWorker(repo storage.Repository, logger log.Logger) *CleanupUploadsWorker {
	return &CleanupUploadsWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", string(storage.JobStorageCleanupExpiredUploads))),
	}
}

// JobType returns the canonical job type this worker handles.
func (w *CleanupUploadsWorker) JobType() core.JobType {
	return storage.JobStorageCleanupExpiredUploads
}

type cleanupUploadsResult struct {
	CleanedCount int64 `json:"cleaned_count"`
}

// ProcessJob is the entry point called by the worker pool.
func (w *CleanupUploadsWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	uploads, err := w.repo.FindExpiredUploads(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find expired uploads: %w", err)
	}
	if len(uploads) == 0 {
		result, _ := json.Marshal(cleanupUploadsResult{CleanedCount: 0})
		return result, nil
	}

	var cleanedCount int64
	for _, upload := range uploads {
		err := w.repo.WithTx(ctx, func(txRepo storage.Repository) error {
			if err := txRepo.DeleteFile(ctx, upload.FileID); err != nil && !errors.Is(err, storage.ErrFileNotFound) {
				w.logger.Error("Failed to delete placeholder file for expired upload", err)
			}
			return txRepo.DeleteUpload(ctx, upload.ID)
		})
		if err != nil {
			w.logger.Error("Failed to clean up expired upload transaction", err, log.String("upload_id", upload.ID.String()))
		} else {
			cleanedCount++
		}
	}

	resultBytes, _ := json.Marshal(cleanupUploadsResult{CleanedCount: cleanedCount})
	return resultBytes, nil
}
