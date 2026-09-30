package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	storage_service "github.com/combi/storage"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// storageServiceTestSuite holds all mocked dependencies for the main Storage Service.
type storageServiceTestSuite struct {
	repo          *storage_mocks.MockRepository
	provider      *storage_mocks.MockProvider
	providerFct   *storage_mocks.MockProviderFactory
	dispatcher    *queue_mocks.MockDispatcher
	auditRecorder *mocks.MockEventRecorder
	rbacService   *rbac_mocks.MockAuthorizationService
	service       storage.Service
	config        storage.ServiceConfig
}

// setupService initializes a new test suite with fresh mocks.
func setupService(t *testing.T) storageServiceTestSuite {
	var ts storageServiceTestSuite
	ts.repo = storage_mocks.NewMockRepository(t)
	ts.provider = storage_mocks.NewMockProvider(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.auditRecorder = mocks.NewMockEventRecorder(t)
	ts.rbacService = rbac_mocks.NewMockAuthorizationService(t)
	ts.providerFct = storage_mocks.NewMockProviderFactory(t)

	// FIX: Use realistic sizes (Bytes).
	// 5MB Upload Test requires > 5MB Limit.
	ts.config = storage.ServiceConfig{
		DefaultQuota:      100 * 1024 * 1024, // 100 MB
		UploadURLExpiry:   15 * time.Minute,
		DownloadURLExpiry: 60 * time.Minute,
		MaxUploadSize:     50 * 1024 * 1024, // 50 MB
		ProcessingQueue:   "media",
	}

	deps := storage_service.Dependencies{
		Repo:          ts.repo,
		ProviderFct:   ts.providerFct,
		Dispatcher:    ts.dispatcher,
		AuditRecorder: ts.auditRecorder,
		RBACService:   ts.rbacService,
		Logger:        &log.NoOpLogger{},
	}
	ts.service = storage_service.NewService(deps, ts.config)

	return ts
}

// Helper to create a context with a test actor.
func contextWithActor(actorID uuid.UUID) context.Context {
	actor := &auth.Actor{AccountID: uuid.New(), UserID: actorID}
	return auth.NewContextWithActor(context.Background(), actor)
}

func TestService_InitiateUpload_Robust(t *testing.T) {
	actorID := uuid.New()
	ctx := contextWithActor(actorID)

	// Common objects
	mockBucket := &storage.StorageBucket{ID: uuid.New(), Name: "test-bucket", Provider: storage.ProviderS3}
	req := storage.InitiateUploadRequest{
		OwnerID:      actorID,
		OwnerType:    "user",
		OriginalName: "report.pdf",
		MimeType:     "application/pdf",
		Size:         1024 * 1024 * 5, // 5MB
		Visibility:   storage.VisibilityPrivate,
	}

	t.Run("Happy Path: New User, Default Quota", func(t *testing.T) {
		ts := setupService(t)

		mockQuota := &storage.StorageQuota{Limit: ts.config.DefaultQuota, Used: 0}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)

			// 1. Quota Check
			txRepo.EXPECT().GetOrCreateQuota(ctx, actorID, "user", ts.config.DefaultQuota).Return(mockQuota, nil).Once()

			// 2. Provider Selection
			ts.providerFct.EXPECT().GetDefaultProvider(ctx).Return(ts.provider, mockBucket, nil).Once()

			// 3. Create File Record
			txRepo.EXPECT().CreateFile(ctx, mock.MatchedBy(func(f *storage.File) bool {
				return f.OwnerID == actorID && f.Status == storage.StatusPending && f.BucketID == mockBucket.ID
			})).Return(nil).Once()

			// 4. Create Upload Session
			txRepo.EXPECT().CreateUpload(ctx, mock.MatchedBy(func(u *storage.Upload) bool {
				return u.TotalSize == req.Size && u.Provider == mockBucket.Provider
			})).Return(nil).Once()

			// 5. Generate URL
			ts.provider.EXPECT().GeneratePresignedUploadURL(
				ctx,
				mockBucket.Name,
				mock.Anything,
				req.MimeType,
				req.Size,
				ts.config.UploadURLExpiry,
			).Return(&storage.SignedURL{URL: "https://signed-url"}, nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		resp, err := ts.service.InitiateUpload(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "https://signed-url", resp.SignedURL.URL)
		assert.NotEqual(t, uuid.Nil, resp.UploadID)
		assert.NotEqual(t, uuid.Nil, resp.FileID)
	})

	t.Run("Failure: Quota Exceeded", func(t *testing.T) {
		ts := setupService(t)
		mockQuota := &storage.StorageQuota{Limit: 1024 * 1024 * 10, Used: 1024 * 1024 * 6}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetOrCreateQuota(ctx, actorID, "user", ts.config.DefaultQuota).Return(mockQuota, nil).Once()

			err := fn(txRepo)
			assert.ErrorIs(t, err, storage.ErrQuotaExceeded)
		}).Return(storage.ErrQuotaExceeded).Once()

		_, err := ts.service.InitiateUpload(ctx, req)

		assert.ErrorIs(t, err, storage.ErrQuotaExceeded)
		ts.auditRecorder.AssertNotCalled(t, "Record")
	})

	t.Run("Failure: Provider Error (URL Generation)", func(t *testing.T) {
		ts := setupService(t)
		mockQuota := &storage.StorageQuota{Limit: ts.config.DefaultQuota, Used: 0}
		providerErr := errors.New("aws region unavailable")

		// FIX: We expect the service to wrap the provider error.
		// Since we are mocking WithTx, we simulate returning the WRAPPED error.
		expectedWrappedErr := storage.NewProviderError("GeneratePresignedUploadURL", "failed to create upload URL", providerErr)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetOrCreateQuota(ctx, mock.Anything, mock.Anything, mock.Anything).Return(mockQuota, nil)
			ts.providerFct.EXPECT().GetDefaultProvider(ctx).Return(ts.provider, mockBucket, nil)
			txRepo.EXPECT().CreateFile(ctx, mock.Anything).Return(nil)
			txRepo.EXPECT().CreateUpload(ctx, mock.Anything).Return(nil)

			// Provider fails
			ts.provider.EXPECT().GeneratePresignedUploadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, providerErr).Once()

			err := fn(txRepo)
			// Verify inside the closure that the error was indeed wrapped
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create upload URL")
		}).Return(expectedWrappedErr).Once() // Simulate WithTx propagating the wrapped error

		_, err := ts.service.InitiateUpload(ctx, req)
		// Now checking against the wrapped error message passes
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create upload URL")
	})

	t.Run("Failure: DB Insert Failed", func(t *testing.T) {
		ts := setupService(t)
		dbErr := errors.New("connection lost")

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetOrCreateQuota(ctx, mock.Anything, mock.Anything, mock.Anything).Return(&storage.StorageQuota{Limit: ts.config.DefaultQuota}, nil)
			ts.providerFct.EXPECT().GetDefaultProvider(ctx).Return(ts.provider, mockBucket, nil)

			// CreateFile fails
			txRepo.EXPECT().CreateFile(ctx, mock.Anything).Return(dbErr).Once()

			err := fn(txRepo)
			assert.ErrorIs(t, err, dbErr)
		}).Return(dbErr).Once()

		_, err := ts.service.InitiateUpload(ctx, req)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("Validation: Config Constraints", func(t *testing.T) {
		// 1. Max Size
		ts := setupService(t)
		hugeReq := req
		hugeReq.Size = ts.config.MaxUploadSize + 1

		_, err := ts.service.InitiateUpload(ctx, hugeReq)
		assert.ErrorIs(t, err, storage.ErrFileTooLarge)

		// 2. Mime Type (if restricted)
		ts.config.AllowedMimeTypePats = []string{"image/*"}
		deps := storage_service.Dependencies{
			Repo: ts.repo, ProviderFct: ts.providerFct, Dispatcher: ts.dispatcher,
			AuditRecorder: ts.auditRecorder, RBACService: ts.rbacService, Logger: &log.NoOpLogger{},
		}
		ts.service = storage_service.NewService(deps, ts.config)

		pdfReq := req // application/pdf
		_, err = ts.service.InitiateUpload(ctx, pdfReq)
		assert.ErrorIs(t, err, storage.ErrInvalidMimeType)
	})
}

func TestService_InitiateUpload(t *testing.T) {
	actorID := uuid.New()
	ctx := contextWithActor(actorID)
	mockBucket := &storage.StorageBucket{ID: uuid.New(), Name: "default-bucket", Provider: storage.ProviderS3}

	req := storage.InitiateUploadRequest{
		OwnerID:      actorID,
		OwnerType:    "user",
		OriginalName: "test.jpg",
		MimeType:     "image/jpeg",
		Size:         100,
		Visibility:   storage.VisibilityPrivate,
	}

	t.Run("should succeed for a valid request", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		mockQuota := &storage.StorageQuota{Limit: 1000, Used: 200}
		mockSignedURL := &storage.SignedURL{URL: "https://s3.signed.url/upload", Method: "PUT"}

		// Mock the transaction block
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetOrCreateQuota(ctx, actorID, "user", ts.config.DefaultQuota).Return(mockQuota, nil).Once()
			// The Provider Factory is called inside the transaction
			ts.providerFct.EXPECT().GetDefaultProvider(ctx).Return(ts.provider, mockBucket, nil).Once()
			txRepo.EXPECT().CreateFile(ctx, mock.AnythingOfType("*storage.File")).Return(nil).Once()
			txRepo.EXPECT().CreateUpload(ctx, mock.AnythingOfType("*storage.Upload")).Return(nil).Once()
			ts.provider.EXPECT().GeneratePresignedUploadURL(ctx, mockBucket.Name, mock.Anything, req.MimeType, req.Size, ts.config.UploadURLExpiry).Return(mockSignedURL, nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		resp, err := ts.service.InitiateUpload(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "https://s3.signed.url/upload", resp.SignedURL.URL)
	})

	t.Run("should fail if quota is exceeded", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		mockQuota := &storage.StorageQuota{Limit: 1000, Used: 950}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetOrCreateQuota(ctx, actorID, "user", ts.config.DefaultQuota).Return(mockQuota, nil).Once()

			err := fn(txRepo)
			assert.ErrorIs(t, err, storage.ErrQuotaExceeded)
		}).Return(storage.ErrQuotaExceeded).Once()

		_, err := ts.service.InitiateUpload(ctx, req)
		assert.ErrorIs(t, err, storage.ErrQuotaExceeded)
	})

	t.Run("should fail if file is too large", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		largeReq := req
		// FIX: Make this dynamic relative to config to ensure failure
		largeReq.Size = ts.config.MaxUploadSize + 1

		_, err := ts.service.InitiateUpload(ctx, largeReq)
		assert.ErrorIs(t, err, storage.ErrFileTooLarge)
	})

	t.Run("should fail if mime type is not allowed", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		ts.config.AllowedMimeTypePats = []string{"image/png", "image/gif"}

		// Remake the service with the new config
		deps := storage_service.Dependencies{Repo: ts.repo, ProviderFct: ts.providerFct, Dispatcher: ts.dispatcher, AuditRecorder: ts.auditRecorder, RBACService: ts.rbacService, Logger: &log.NoOpLogger{}}
		ts.service = storage_service.NewService(deps, ts.config)

		_, err := ts.service.InitiateUpload(ctx, req)
		assert.ErrorIs(t, err, storage.ErrInvalidMimeType)
	})
}

func TestService_CompleteUpload(t *testing.T) {
	uploadID := uuid.New()
	fileID := uuid.New()
	ctx := context.Background()

	mockUpload := &storage.Upload{ID: uploadID, FileID: fileID, ExpiresAt: time.Now().Add(time.Hour)}
	mockFile := &storage.File{ID: fileID, Size: 100, Path: "path/to/file.jpg", BucketID: uuid.New(), Category: storage.CategoryImage}
	mockBucket := &storage.StorageBucket{ID: mockFile.BucketID, Name: "default-bucket"}

	t.Run("should succeed for a valid, existing upload", func(t *testing.T) {
		ts := setupService(t) // Isolate test

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetUpload(ctx, uploadID).Return(mockUpload, nil).Once()
			txRepo.EXPECT().GetFile(ctx, fileID).Return(mockFile, nil).Once()
			txRepo.EXPECT().GetBucket(ctx, mockFile.BucketID).Return(mockBucket, nil).Once()
			ts.providerFct.EXPECT().GetProvider(ctx, mockBucket).Return(ts.provider, nil).Once()
			ts.provider.EXPECT().Exists(ctx, mockBucket.Name, mockFile.Path).Return(true, nil).Once()
			txRepo.EXPECT().UpdateFile(ctx, mock.AnythingOfType("*storage.File")).Return(nil).Once()
			txRepo.EXPECT().IncrementUsage(ctx, mockFile.OwnerID, mockFile.OwnerType, mockFile.Size).Return(nil).Once()
			txRepo.EXPECT().DeleteUpload(ctx, uploadID).Return(nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.dispatcher.EXPECT().Dispatch(mock.Anything, storage.JobStorageProcessImage, mock.Anything, mock.AnythingOfType("queue.DispatchOption"), mock.AnythingOfType("queue.DispatchOption")).Return(&queue.Job{}, nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		file, err := ts.service.CompleteUpload(ctx, uploadID)
		require.NoError(t, err)
		assert.Equal(t, storage.StatusProcessing, file.Status)
	})

	t.Run("should fail if underlying file does not exist on provider", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetUpload(ctx, uploadID).Return(mockUpload, nil).Once()
			txRepo.EXPECT().GetFile(ctx, fileID).Return(mockFile, nil).Once()
			txRepo.EXPECT().GetBucket(ctx, mockFile.BucketID).Return(mockBucket, nil).Once()
			ts.providerFct.EXPECT().GetProvider(ctx, mockBucket).Return(ts.provider, nil).Once()
			ts.provider.EXPECT().Exists(ctx, mockBucket.Name, mockFile.Path).Return(false, nil).Once()

			err := fn(txRepo)
			assert.ErrorIs(t, err, storage.ErrUploadFailed)
		}).Return(storage.ErrUploadFailed).Once()

		_, err := ts.service.CompleteUpload(ctx, uploadID)
		assert.ErrorIs(t, err, storage.ErrUploadFailed)
	})
}

func TestService_GetDownloadURL(t *testing.T) {
	actorID := uuid.New()
	fileID := uuid.New()
	ctx := contextWithActor(actorID)

	mockFile := &storage.File{ID: fileID, OwnerID: actorID, Status: storage.StatusAvailable, Visibility: storage.VisibilityPrivate, BucketID: uuid.New(), Path: "path/to/file"}
	mockBucket := &storage.StorageBucket{ID: mockFile.BucketID, Name: "private-bucket"}
	mockSignedURL := &storage.SignedURL{URL: "https://s3.signed.url/download"}

	t.Run("should succeed for owner of a private file", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(mockFile, nil).Once()
		ts.repo.EXPECT().GetBucket(ctx, mockFile.BucketID).Return(mockBucket, nil).Once()
		ts.providerFct.EXPECT().GetProvider(ctx, mockBucket).Return(ts.provider, nil).Once()
		ts.provider.EXPECT().GeneratePresignedDownloadURL(ctx, mockBucket.Name, mockFile.Path, ts.config.DownloadURLExpiry).Return(mockSignedURL, nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		url, err := ts.service.GetDownloadURL(ctx, fileID, 0)
		require.NoError(t, err)
		assert.Equal(t, "https://s3.signed.url/download", url.URL)
	})

	t.Run("should fail for a file that is not available", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		processingFile := *mockFile
		processingFile.Status = storage.StatusProcessing
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(&processingFile, nil).Once()

		_, err := ts.service.GetDownloadURL(ctx, fileID, 0)
		assert.ErrorIs(t, err, storage.ErrFileNotAvailable)
	})

	t.Run("should deny access to non-owner of a private file without RBAC permission", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		otherUserCtx := contextWithActor(uuid.New())

		ts.repo.EXPECT().GetFile(otherUserCtx, fileID).Return(mockFile, nil).Once()
		ts.rbacService.EXPECT().Authorize(otherUserCtx, mock.Anything).Return(false, nil).Once()

		_, err := ts.service.GetDownloadURL(otherUserCtx, fileID, 0)
		assert.ErrorIs(t, err, storage.ErrAccessDenied)
	})
}

func TestService_DeleteFile(t *testing.T) {
	actorID := uuid.New()
	fileID := uuid.New()
	ctx := contextWithActor(actorID)
	bucketID := uuid.New()

	// Create a base struct to copy from to ensure clean state per test
	baseFile := storage.File{
		ID:        fileID,
		OwnerID:   actorID,
		Path:      "path/to/file.jpg",
		BucketID:  bucketID,
		Size:      123,
		OwnerType: "user",
		Status:    storage.StatusAvailable,
	}

	t.Run("should soft delete a file successfully", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		file := baseFile      // Copy

		ts.repo.EXPECT().GetFile(ctx, fileID).Return(&file, nil).Once()
		ts.repo.EXPECT().UpdateFile(ctx, mock.MatchedBy(func(f *storage.File) bool {
			return f.Status == storage.StatusDeleted && f.DeletedAt != nil
		})).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.DeleteFile(ctx, fileID, false)
		require.NoError(t, err)
	})

	t.Run("should permanently delete a file successfully", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		file := baseFile      // Copy
		mockBucket := &storage.StorageBucket{ID: file.BucketID, Name: "the-bucket"}

		// 1. Expect the initial GetFile call on the main repo.
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(&file, nil).Once()

		// 2. Expect the WithTx call.
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			// This function will be called by the service. Inside it, we set expectations
			// on a NEW mock that represents the transaction.
			txRepo := storage_mocks.NewMockRepository(t)

			// These calls happen inside the transaction, so we expect them on txRepo.
			txRepo.EXPECT().GetBucket(ctx, file.BucketID).Return(mockBucket, nil).Once()
			// Since file is NOT soft deleted (fresh copy), this SHOULD be called
			txRepo.EXPECT().DecrementUsage(ctx, file.OwnerID, file.OwnerType, file.Size).Return(nil).Once()
			txRepo.EXPECT().DeleteFile(ctx, fileID).Return(nil).Once()

			// The ProviderFactory is called from within the transaction block in the service code.
			ts.providerFct.EXPECT().GetProvider(ctx, mockBucket).Return(ts.provider, nil).Once()
			// The provider's Delete method is also called within the transaction block.
			ts.provider.EXPECT().Delete(ctx, mockBucket.Name, file.Path).Return(nil).Once()

			// Execute the function passed by the service, which will trigger the txRepo expectations.
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		// 3. Expect the final audit call.
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// 4. Run the method under test.
		err := ts.service.DeleteFile(ctx, fileID, true)
		require.NoError(t, err)
	})

	t.Run("should NOT decrement usage if file was already soft-deleted", func(t *testing.T) {
		ts := setupService(t)
		deletedAt := time.Now()
		softDeletedFile := &storage.File{
			ID:        fileID,
			OwnerID:   actorID,
			Path:      "path/to/file.jpg",
			BucketID:  bucketID,
			Size:      123,
			OwnerType: "user",
			Status:    storage.StatusDeleted, // Mark as soft-deleted
			DeletedAt: &deletedAt,
		}
		mockBucket := &storage.StorageBucket{ID: softDeletedFile.BucketID, Name: "the-bucket"}

		ts.repo.EXPECT().GetFile(ctx, fileID).Return(softDeletedFile, nil).Once()
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetBucket(ctx, softDeletedFile.BucketID).Return(mockBucket, nil).Once()
			ts.providerFct.EXPECT().GetProvider(ctx, mockBucket).Return(ts.provider, nil).Once()
			ts.provider.EXPECT().Delete(ctx, mockBucket.Name, softDeletedFile.Path).Return(nil).Once()

			// CRITICAL: NO expectation for DecrementUsage is set here.

			txRepo.EXPECT().DeleteFile(ctx, fileID).Return(nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.DeleteFile(ctx, fileID, true)
		require.NoError(t, err)
	})

	t.Run("should fail to delete if not owner", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		otherUsersFile := &storage.File{ID: fileID, OwnerID: uuid.New()}
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(otherUsersFile, nil).Once()

		err := ts.service.DeleteFile(ctx, fileID, false)
		assert.ErrorIs(t, err, storage.ErrAccessDenied)
	})
}

func TestService_RestoreFile(t *testing.T) {
	actorID := uuid.New()
	fileID := uuid.New()
	ctx := contextWithActor(actorID)
	deletedAt := time.Now().Add(-time.Hour)

	mockFile := &storage.File{ID: fileID, OwnerID: actorID, Status: storage.StatusDeleted, DeletedAt: &deletedAt}

	t.Run("should restore a soft-deleted file", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(mockFile, nil).Once()
		ts.repo.EXPECT().UpdateFile(ctx, mock.MatchedBy(func(f *storage.File) bool {
			return f.Status == storage.StatusAvailable && f.DeletedAt == nil
		})).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		file, err := ts.service.RestoreFile(ctx, fileID)
		require.NoError(t, err)
		assert.Equal(t, storage.StatusAvailable, file.Status)
	})

	t.Run("should be idempotent if file is not deleted", func(t *testing.T) {
		ts := setupService(t) // Isolate test
		activeFile := &storage.File{ID: fileID, OwnerID: actorID, Status: storage.StatusAvailable}
		ts.repo.EXPECT().GetFile(ctx, fileID).Return(activeFile, nil).Once()

		file, err := ts.service.RestoreFile(ctx, fileID)
		require.NoError(t, err)
		assert.Equal(t, storage.StatusAvailable, file.Status)
	})
}

func TestService_AnonymizeUser(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	// Define files that will be returned by ListFiles
	files := []*storage.File{
		{ID: uuid.New(), OwnerID: userID, Size: 100, Path: "a", BucketID: uuid.New()},
		{ID: uuid.New(), OwnerID: userID, Size: 200, Path: "b", BucketID: uuid.New()},
	}

	t.Run("should list files and dispatch deletion jobs", func(t *testing.T) {
		ts := setupService(t)

		// 1. Expect ListFiles to return the user's files
		ts.repo.EXPECT().ListFiles(mock.Anything, mock.MatchedBy(func(f storage.FileFilter) bool {
			return f.OwnerID != nil && *f.OwnerID == userID
		})).Return(files, len(files), nil).Once()

		// 2. Expect a Dispatch call for EACH file.
		for _, f := range files {
			ts.dispatcher.EXPECT().Dispatch(
				ctx,
				storage.JobStorageDeleteFile,
				mock.MatchedBy(func(payload json.RawMessage) bool {
					var p map[string]string
					json.Unmarshal(payload, &p)
					return p["file_id"] == f.ID.String()
				}),
				mock.Anything, // Options
			).Return(&queue.Job{}, nil).Once()
		}

		// Act
		report, err := ts.service.AnonymizeUser(ctx, userID)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.Equal(t, int64(2), report.RecordsDeleted) // 2 jobs dispatched
	})

	t.Run("should return error if listing fails", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().ListFiles(mock.Anything, mock.Anything).Return(nil, 0, errors.New("db error")).Once()

		_, err := ts.service.AnonymizeUser(ctx, userID)
		assert.Error(t, err)
	})
}
