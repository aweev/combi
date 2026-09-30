package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageRepository_Buckets(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewStorageRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CRUD", func(t *testing.T) {
		b := &storage.StorageBucket{
			ID:        uuid.New(),
			Name:      "main-bucket",
			Provider:  storage.ProviderS3,
			Region:    "us-east-1",
			IsDefault: true,
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, repo.CreateBucket(ctx, b))

		got, err := repo.GetBucket(ctx, b.ID)
		require.NoError(t, err)
		assert.Equal(t, b.Name, got.Name)

		gotDef, err := repo.GetDefaultBucket(ctx)
		require.NoError(t, err)
		assert.Equal(t, b.ID, gotDef.ID)

		// Update
		b.IsActive = false
		require.NoError(t, repo.UpdateBucket(ctx, b))

		updated, _ := repo.GetBucket(ctx, b.ID)
		assert.False(t, updated.IsActive)
	})
}

func TestStorageRepository_Files(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewStorageRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup Bucket
	bucket := &storage.StorageBucket{ID: uuid.New(), Name: "test", Provider: storage.ProviderLocal, Region: "local", IsActive: true}
	require.NoError(t, repo.CreateBucket(ctx, bucket))

	t.Run("FileLifecycle", func(t *testing.T) {
		f := &storage.File{
			ID:           uuid.New(),
			OwnerID:      uuid.New(),
			OwnerType:    "user",
			BucketID:     bucket.ID,
			OriginalName: "photo.jpg",
			Path:         "u/1/photo.jpg",
			MimeType:     "image/jpeg",
			Size:         1024,
			Category:     storage.CategoryImage,
			Status:       storage.StatusPending,
			Visibility:   storage.VisibilityPrivate,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		require.NoError(t, repo.CreateFile(ctx, f))

		got, err := repo.GetFile(ctx, f.ID)
		require.NoError(t, err)
		assert.Equal(t, f.OriginalName, got.OriginalName)

		// List
		list, count, err := repo.ListFiles(ctx, storage.FileFilter{BucketID: &bucket.ID})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, f.ID, list[0].ID)

		// Update
		f.Status = storage.StatusAvailable
		completed := time.Now().UTC()
		f.UploadCompletedAt = &completed
		require.NoError(t, repo.UpdateFile(ctx, f))

		updated, _ := repo.GetFile(ctx, f.ID)
		assert.Equal(t, storage.StatusAvailable, updated.Status)
		assert.NotNil(t, updated.UploadCompletedAt)
	})
}

func TestStorageRepository_Quotas(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewStorageRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	ownerID := uuid.New()

	t.Run("QuotaManagement", func(t *testing.T) {
		// Get or Create
		q, err := repo.GetOrCreateQuota(ctx, ownerID, "user", 1000)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), q.Limit)
		assert.Equal(t, int64(0), q.Used)

		// Increment
		require.NoError(t, repo.IncrementUsage(ctx, ownerID, "user", 500))
		q, _ = repo.GetOrCreateQuota(ctx, ownerID, "user", 1000)
		assert.Equal(t, int64(500), q.Used)

		// Decrement
		require.NoError(t, repo.DecrementUsage(ctx, ownerID, "user", 200))
		q, _ = repo.GetOrCreateQuota(ctx, ownerID, "user", 1000)
		assert.Equal(t, int64(300), q.Used)
	})
}
