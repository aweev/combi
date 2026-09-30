package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Statically verify that CleanupWorker implements the JobProcessor interface.
var _ queue.JobProcessor = (*CleanupWorker)(nil)

// CleanupWorker consumes `storage:cleanup_deleted` jobs for permanent deletion.
type CleanupWorker struct {
	repo        storage.Repository
	providerFct storage.ProviderFactory
	logger      log.Logger
}

// A constant for the batch size makes the logic clearer.
const cleanupBatchSize = 50

func NewCleanupWorker(repo storage.Repository, providerFct storage.ProviderFactory, logger log.Logger) *CleanupWorker {
	return &CleanupWorker{
		repo:        repo,
		providerFct: providerFct,
		logger:      logger.With(log.String("worker", string(storage.JobStorageCleanupDeleted))),
	}
}

// JobType returns the canonical job type this worker handles.
func (w *CleanupWorker) JobType() core.JobType {
	return storage.JobStorageCleanupDeleted
}

type cleanupPayload struct {
	OlderThanDays int `json:"older_than_days"`
}

type resultPayload struct {
	PermanentlyDeletedCount int64 `json:"permanently_deleted_count"`
}

// ProcessJob is the entry point called by the worker pool.
func (w *CleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p cleanupPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid cleanup_deleted payload: %w", err))
	}
	if p.OlderThanDays <= 0 {
		p.OlderThanDays = 30
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -p.OlderThanDays)
	files, err := w.repo.FindSoftDeletedBefore(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to find soft-deleted files: %w", err)
	}
	if len(files) == 0 {
		result, _ := json.Marshal(resultPayload{PermanentlyDeletedCount: 0})
		return result, nil
	}

	var deletedCount int64
	// --- Start of refactored code ---
	// Replace toolkit.BatchProcessor with a standard for loop.
	for i := 0; i < len(files); i += cleanupBatchSize {
		end := i + cleanupBatchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]

		// The logic from the original callback is now directly inside the loop.
		for _, file := range batch {
			if err := w.permanentlyDeleteFile(ctx, file); err != nil {
				w.logger.Error("Failed to permanently delete file", err, log.String("file_id", file.ID.String()))
			} else {
				deletedCount++
			}
		}
	}
	// --- End of refactored code ---

	resultBytes, _ := json.Marshal(resultPayload{PermanentlyDeletedCount: deletedCount})
	return resultBytes, nil
}

func (w *CleanupWorker) permanentlyDeleteFile(ctx context.Context, file *storage.File) error {
	return w.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		bucket, err := txRepo.GetBucket(ctx, file.BucketID)
		if err != nil {
			if errors.Is(err, storage.ErrBucketNotFound) {
				w.logger.Warn("Bucket not found for file during cleanup; proceeding with DB deletion only.", log.String("file_id", file.ID.String()))
			} else {
				return err
			}
		} else {
			provider, err := w.providerFct.GetProvider(ctx, bucket)
			if err != nil {
				return err
			}
			if err := provider.Delete(ctx, bucket.Name, file.Path); err != nil {
				w.logger.Warn("Failed to delete file from provider during cleanup", log.ErrorAttr(err), log.String("file_id", file.ID.String()))
			}
		}
		if err := txRepo.DecrementUsage(ctx, file.OwnerID, file.OwnerType, file.Size); err != nil {
			w.logger.Error("Failed to decrement quota during permanent delete", err, log.String("file_id", file.ID.String()))
		}
		return txRepo.DeleteFile(ctx, file.ID)
	})
}
