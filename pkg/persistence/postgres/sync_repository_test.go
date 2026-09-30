package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewSyncRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("EnsureDevice_Lifecycle", func(t *testing.T) {
		userID := uuid.New()
		deviceID := "android-pixel-5-xyz"

		// 1. New Device Registration
		err := repo.EnsureDevice(ctx, userID, deviceID)
		require.NoError(t, err, "First call should register device")

		// Verify in DB
		// Define the struct once to match the SELECT query columns
		type DeviceRow struct {
			IsRevoked  bool      `db:"is_revoked"`
			LastSyncAt time.Time `db:"last_sync_at"`
		}

		query := `SELECT is_revoked, last_sync_at FROM sync_devices WHERE user_id = $1 AND device_id = $2`

		var row DeviceRow
		err = db.GetContext(ctx, &row, query, userID, deviceID)
		require.NoError(t, err)
		assert.False(t, row.IsRevoked)
		assert.False(t, row.LastSyncAt.IsZero())

		firstSyncTime := row.LastSyncAt

		// Small sleep to ensure timestamp difference
		time.Sleep(100 * time.Millisecond)

		// 2. Existing Device Update
		err = repo.EnsureDevice(ctx, userID, deviceID)
		require.NoError(t, err, "Second call should update timestamp")

		// Verify Timestamp Updated
		var rowUpdated DeviceRow
		err = db.GetContext(ctx, &rowUpdated, query, userID, deviceID)
		require.NoError(t, err)
		assert.True(t, rowUpdated.LastSyncAt.After(firstSyncTime), "last_sync_at should be updated")

		// 3. Revocation Check
		_, err = db.ExecContext(ctx, `UPDATE sync_devices SET is_revoked = TRUE WHERE user_id = $1 AND device_id = $2`, userID, deviceID)
		require.NoError(t, err)

		err = repo.EnsureDevice(ctx, userID, deviceID)
		assert.Error(t, err, "Should fail on revoked device")
		assert.Contains(t, err.Error(), "revoked")
	})

	t.Run("LogSession", func(t *testing.T) {
		userID := uuid.New()
		deviceID := "iphone-12-abc"

		// 1. Log a Success Push
		err := repo.LogSession(ctx, userID, deviceID, "PUSH", "SUCCESS", 5, nil)
		require.NoError(t, err)

		// 2. Log a Failed Pull
		errMsg := "connection timeout"
		err = repo.LogSession(ctx, userID, deviceID, "PULL", "ERROR", 0, &errMsg)
		require.NoError(t, err)

		// Verify Logs
		type LogRow struct {
			SyncType string  `db:"sync_type"`
			Status   string  `db:"status"`
			OpsCount int     `db:"ops_count"`
			ErrorMsg *string `db:"error_message"`
		}

		var rows []LogRow
		err = db.SelectContext(ctx, &rows, `SELECT sync_type, status, ops_count, error_message FROM sync_logs WHERE user_id = $1 ORDER BY occurred_at ASC`, userID)
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, "PUSH", rows[0].SyncType)
		assert.Equal(t, "SUCCESS", rows[0].Status)
		assert.Equal(t, "PULL", rows[1].SyncType)
		assert.Equal(t, "connection timeout", *rows[1].ErrorMsg)
	})
}
