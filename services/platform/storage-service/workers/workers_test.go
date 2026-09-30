package workers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/storage/workers"
)

func TestImageWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	fileID := uuid.New()
	variants := map[string]string{"thumb": "100x100"}
	payload, _ := json.Marshal(map[string]interface{}{
		"file_id":  fileID.String(),
		"variants": variants,
	})
	job := &queue.Job{ID: uuid.New(), Payload: payload}

	t.Run("Success", func(t *testing.T) {
		mockSvc := storage_mocks.NewMockWorkerService(t)
		worker := workers.NewImageWorker(mockSvc, &log.NoOpLogger{})

		mockSvc.EXPECT().ProcessImage(ctx, fileID, variants).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("Failure", func(t *testing.T) {
		mockSvc := storage_mocks.NewMockWorkerService(t)
		worker := workers.NewImageWorker(mockSvc, &log.NoOpLogger{})
		procErr := errors.New("processing failed")

		mockSvc.EXPECT().ProcessImage(ctx, fileID, variants).Return(procErr).Once()

		_, err := worker.ProcessJob(ctx, job)
		assert.ErrorIs(t, err, procErr)
	})

	t.Run("Invalid Payload", func(t *testing.T) {
		mockSvc := storage_mocks.NewMockWorkerService(t)
		worker := workers.NewImageWorker(mockSvc, &log.NoOpLogger{})
		invalidJob := &queue.Job{Payload: []byte("bad json")}

		_, err := worker.ProcessJob(ctx, invalidJob)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable))
	})
}

func TestDeleteFileWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	fileID := uuid.New()
	payload, _ := json.Marshal(map[string]string{"file_id": fileID.String()})
	job := &queue.Job{ID: uuid.New(), Payload: payload}

	t.Run("Success", func(t *testing.T) {
		mockSvc := storage_mocks.NewMockService(t) // Note: Uses Service, not WorkerService
		worker := workers.NewDeleteFileWorker(mockSvc, &log.NoOpLogger{})

		mockSvc.EXPECT().DeleteFile(ctx, fileID, true).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})
}

func TestCleanupUploadsWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	job := &queue.Job{ID: uuid.New(), Payload: []byte("{}")}

	t.Run("Success with uploads", func(t *testing.T) {
		repo := storage_mocks.NewMockRepository(t)
		worker := workers.NewCleanupUploadsWorker(repo, &log.NoOpLogger{})

		upload := &storage.Upload{ID: uuid.New(), FileID: uuid.New()}

		// 1. Find
		repo.EXPECT().FindExpiredUploads(ctx).Return([]*storage.Upload{upload}, nil).Once()

		// 2. Cleanup Transaction
		repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().DeleteFile(ctx, upload.FileID).Return(nil).Once()
			txRepo.EXPECT().DeleteUpload(ctx, upload.ID).Return(nil).Once()
			fn(txRepo)
		}).Return(nil).Once()

		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
		assert.Contains(t, string(res), `"cleaned_count":1`)
	})
}
