package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivacyRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewPrivacyRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Shared UserID for basic tests, but be careful with state
	userID := uuid.New()

	t.Run("SaveRequest_Create", func(t *testing.T) {
		req := &privacy.Request{
			ID:          uuid.New(),
			UserID:      userID,
			Type:        privacy.TypeErasure,
			Status:      privacy.StatusPending,
			RequestedAt: time.Now().UTC().Truncate(time.Microsecond),
			Reason:      "gdpr",
		}

		err := repo.SaveRequest(ctx, req)
		require.NoError(t, err)

		got, err := repo.GetRequest(ctx, req.ID)
		require.NoError(t, err)
		assert.Equal(t, req.ID, got.ID)
		assert.Equal(t, req.UserID, got.UserID)
		assert.Equal(t, req.Status, got.Status)
	})

	t.Run("SaveRequest_Update", func(t *testing.T) {
		req := &privacy.Request{
			ID:          uuid.New(),
			UserID:      userID,
			Type:        privacy.TypeExport,
			Status:      privacy.StatusProcessing,
			RequestedAt: time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.SaveRequest(ctx, req))

		// Update
		req.Status = privacy.StatusCompleted
		now := time.Now().UTC().Truncate(time.Microsecond)
		req.CompletedAt = &now
		url := "https://s3..."
		req.ResultLocation = &url
		req.ErasureDetails = &privacy.ErasureReport{RecordsDeleted: 10}

		err := repo.SaveRequest(ctx, req)
		require.NoError(t, err)

		got, err := repo.GetRequest(ctx, req.ID)
		require.NoError(t, err)
		assert.Equal(t, privacy.StatusCompleted, got.Status)
		assert.Equal(t, int64(10), got.ErasureDetails.RecordsDeleted)
		assert.Equal(t, url, *got.ResultLocation)
	})

	t.Run("GetPendingRequest", func(t *testing.T) {
		// Use a FRESH UserID to ensure isolation from previous tests
		uniqueUser := uuid.New()

		// Create Pending
		p := &privacy.Request{
			ID:          uuid.New(),
			UserID:      uniqueUser,
			Type:        privacy.TypeErasure,
			Status:      privacy.StatusPending,
			RequestedAt: time.Now(),
		}
		require.NoError(t, repo.SaveRequest(ctx, p))

		got, err := repo.GetPendingRequest(ctx, uniqueUser, privacy.TypeErasure)
		require.NoError(t, err)
		assert.Equal(t, p.ID, got.ID)

		// Create Completed (Should be ignored)
		c := &privacy.Request{
			ID:          uuid.New(),
			UserID:      uniqueUser,
			Type:        privacy.TypeErasure,
			Status:      privacy.StatusCompleted,
			RequestedAt: time.Now(),
		}
		require.NoError(t, repo.SaveRequest(ctx, c))

		// Verify we still get pending
		got2, err := repo.GetPendingRequest(ctx, uniqueUser, privacy.TypeErasure)
		require.NoError(t, err)
		assert.Equal(t, p.ID, got2.ID)
	})
}
