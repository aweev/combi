// internal/services/storage/admin_service_test.go
package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	storage_service "github.com/combi/storage"
)

type adminServiceTestSuite struct {
	repo          *storage_mocks.MockRepository
	dispatcher    *queue_mocks.MockDispatcher
	auditRecorder *mocks.MockEventRecorder
	rbacService   *rbac_mocks.MockAuthorizationService
	service       storage.AdminService
}

func setupAdminService(t *testing.T) adminServiceTestSuite {
	var ts adminServiceTestSuite
	ts.repo = storage_mocks.NewMockRepository(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.auditRecorder = mocks.NewMockEventRecorder(t)
	ts.rbacService = rbac_mocks.NewMockAuthorizationService(t)

	deps := storage_service.Dependencies{
		Repo:          ts.repo,
		Dispatcher:    ts.dispatcher,
		AuditRecorder: ts.auditRecorder,
		RBACService:   ts.rbacService,
		Logger:        &log.NoOpLogger{},
	}
	cfg := storage.ServiceConfig{DefaultQuota: 1000}

	ts.service = storage_service.NewAdminService(deps, cfg)
	return ts
}

func contextWithAdmin(t *testing.T) context.Context {
	actor := &auth.Actor{
		AccountID: uuid.New(),
		UserID:    uuid.New(),
	}
	return auth.NewContextWithActor(context.Background(), actor)
}

func TestAdminService_CreateBucket(t *testing.T) {
	ctx := contextWithAdmin(t)
	req := storage.CreateBucketRequest{
		Name:     "new-bucket",
		Provider: storage.ProviderS3,
		Region:   "us-east-1",
	}

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetBucketByName(ctx, "new-bucket").Return(nil, storage.ErrBucketNotFound).Once()
		ts.repo.EXPECT().CreateBucket(ctx, mock.MatchedBy(func(b *storage.StorageBucket) bool {
			return b.Name == "new-bucket" && b.Provider == storage.ProviderS3
		})).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		bucket, err := ts.service.CreateBucket(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "new-bucket", bucket.Name)
	})

	t.Run("fail if bucket exists", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetBucketByName(ctx, "new-bucket").Return(&storage.StorageBucket{}, nil).Once()

		_, err := ts.service.CreateBucket(ctx, req)
		assert.ErrorIs(t, err, storage.ErrBucketAlreadyExists)
	})
}

func TestAdminService_UpdateBucket(t *testing.T) {
	ctx := contextWithAdmin(t)
	bucketID := uuid.New()
	bucket := &storage.StorageBucket{ID: bucketID, Name: "test-bucket", IsActive: true, IsDefault: false}

	t.Run("update active status", func(t *testing.T) {
		ts := setupAdminService(t)
		isActive := false
		req := storage.UpdateBucketRequest{IsActive: &isActive}

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetBucket(ctx, bucketID).Return(bucket, nil).Once()
			txRepo.EXPECT().UpdateBucket(ctx, mock.MatchedBy(func(b *storage.StorageBucket) bool {
				return b.IsActive == false
			})).Return(nil).Once()
			fn(txRepo)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.UpdateBucket(ctx, bucketID, req)
		require.NoError(t, err)
	})

	t.Run("set as default unsets previous default", func(t *testing.T) {
		ts := setupAdminService(t)
		isDefault := true
		req := storage.UpdateBucketRequest{IsDefault: &isDefault}
		oldDefault := &storage.StorageBucket{ID: uuid.New(), IsDefault: true}

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetBucket(ctx, bucketID).Return(bucket, nil).Once()
			txRepo.EXPECT().GetDefaultBucket(ctx).Return(oldDefault, nil).Once()

			// Expect update on old default to false
			txRepo.EXPECT().UpdateBucket(ctx, mock.MatchedBy(func(b *storage.StorageBucket) bool {
				return b.ID == oldDefault.ID && b.IsDefault == false
			})).Return(nil).Once()

			// Expect update on new default to true
			txRepo.EXPECT().UpdateBucket(ctx, mock.MatchedBy(func(b *storage.StorageBucket) bool {
				return b.ID == bucket.ID && b.IsDefault == true
			})).Return(nil).Once()

			fn(txRepo)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.UpdateBucket(ctx, bucketID, req)
		require.NoError(t, err)
	})
}

func TestAdminService_DeleteBucket(t *testing.T) {
	ctx := contextWithAdmin(t)
	bucketID := uuid.New()
	bucket := &storage.StorageBucket{ID: bucketID, Name: "test", IsDefault: false}

	t.Run("success if empty", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetBucket(ctx, bucketID).Return(bucket, nil).Once()
		// Check for files
		ts.repo.EXPECT().ListFiles(ctx, mock.MatchedBy(func(f storage.FileFilter) bool {
			return f.BucketID != nil && *f.BucketID == bucketID
		})).Return([]*storage.File{}, 0, nil).Once()

		ts.repo.EXPECT().DeleteBucket(ctx, bucketID).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.DeleteBucket(ctx, bucketID)
		require.NoError(t, err)
	})

	t.Run("fail if not empty", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetBucket(ctx, bucketID).Return(bucket, nil).Once()
		ts.repo.EXPECT().ListFiles(ctx, mock.Anything).Return([]*storage.File{{}}, 1, nil).Once()

		err := ts.service.DeleteBucket(ctx, bucketID)
		assert.ErrorIs(t, err, storage.ErrBucketNotEmpty)
	})
}

func TestAdminService_SetStorageQuota(t *testing.T) {
	ctx := contextWithAdmin(t)
	ownerID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)
		quota := &storage.StorageQuota{ID: uuid.New(), OwnerID: ownerID}

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetOrCreateQuota(ctx, ownerID, "user", int64(1000)).Return(quota, nil).Once()
		ts.repo.EXPECT().UpdateQuota(ctx, mock.MatchedBy(func(q *storage.StorageQuota) bool {
			return q.Limit == 5000
		})).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.SetStorageQuota(ctx, ownerID, "user", 5000)
		require.NoError(t, err)
	})
}

func TestAdminService_CleanupExpiredUploads(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)
		upload := &storage.Upload{ID: uuid.New(), FileID: uuid.New()}

		ts.repo.EXPECT().FindExpiredUploads(ctx).Return([]*storage.Upload{upload}, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(storage.Repository) error) {
			txRepo := storage_mocks.NewMockRepository(t)
			txRepo.EXPECT().DeleteFile(ctx, upload.FileID).Return(nil).Once()
			txRepo.EXPECT().DeleteUpload(ctx, upload.ID).Return(nil).Once()
			fn(txRepo)
		}).Return(nil).Once()

		count, err := ts.service.CleanupExpiredUploads(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func TestAdminService_CleanupDeletedFiles(t *testing.T) {
	ctx := contextWithAdmin(t)

	t.Run("success dispatches job", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.rbacService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()

		ts.dispatcher.EXPECT().Dispatch(
			ctx,
			storage.JobStorageCleanupDeleted,
			mock.Anything, // Use mock.Anything to handle nil json.RawMessage
			mock.Anything, mock.Anything,
		).Return(&queue.Job{ID: uuid.New()}, nil).Once()

		_, err := ts.service.CleanupDeletedFiles(ctx, 30*24*time.Hour)
		require.NoError(t, err)
	})
}
