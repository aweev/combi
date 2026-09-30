package analytics_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	analytics_service "github.com/combi/analytics"
	"github.com/combi/pkg/testutil"
)

func TestAnalyticsWorkers(t *testing.T) {
	// Shared Test DB Setup
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	ctx := context.Background()

	// Ensure pgcrypto is enabled for tests
	_, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	require.NoError(t, err)

	// --- 1. Test Partition Worker ---
	t.Run("PartitionWorker creates future tables", func(t *testing.T) {
		worker := analytics_service.NewPartitionWorker(db, logger)

		job := &queue.Job{ID: uuid.New(), Type: analytics.JobMaintainPartitions}
		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
		assert.Contains(t, string(res), "success")

		futureDate := time.Now().AddDate(0, 2, 0)
		expectedTableName := fmt.Sprintf("analytics_events_y%dm%02d", futureDate.Year(), futureDate.Month())

		var exists bool
		query := `SELECT EXISTS (SELECT FROM pg_tables WHERE tablename = $1)`
		err = db.GetContext(ctx, &exists, query, expectedTableName)
		require.NoError(t, err)
		assert.True(t, exists, "Partition table %s should exist", expectedTableName)
	})

	// --- 2. Test Aggregation Worker ---
	t.Run("AggregationWorker populates rollup tables", func(t *testing.T) {
		repo := postgres.NewAnalyticsRepository(db, logger)
		worker := analytics_service.NewAggregationWorker(repo, logger)

		yesterday := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
		uID := uuid.New()

		events := []analytics.AnalyticsEvent{
			{ID: uuid.New(), UserID: &uID, SessionID: "s1", EventType: "login", CreatedAt: yesterday.Add(1 * time.Hour)},
			{ID: uuid.New(), UserID: &uID, SessionID: "s1", EventType: "login", CreatedAt: yesterday.Add(2 * time.Hour)},
		}
		require.NoError(t, repo.InsertBatch(ctx, events))

		payload, _ := json.Marshal(map[string]string{
			"target_date": yesterday.Format("2006-01-02"),
		})
		job := &queue.Job{ID: uuid.New(), Type: analytics.JobAnalyticsAggregate, Payload: payload}

		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
		assert.Contains(t, string(res), "success")

		var dailyCount int
		err = db.GetContext(ctx, &dailyCount, "SELECT count FROM analytics_daily_users WHERE date = $1", yesterday)
		require.NoError(t, err)
		assert.Equal(t, 1, dailyCount)
	})

	// --- 3. Test Anonymization Worker ---
	t.Run("AnonymizationWorker scrubs PII", func(t *testing.T) {
		worker := analytics_service.NewAnonymizationWorker(db, logger, "my-secret-pepper")

		oldID := uuid.New()
		newID := uuid.New()
		uID := uuid.New()

		// Manually insert data to simulate history
		_, err := db.ExecContext(ctx, `
			INSERT INTO analytics_events (id, user_id, session_id, event_type, ip_address, created_at)
			VALUES 
				($1, $2, 'session-old', 'click', '192.168.1.1', NOW() - INTERVAL '40 days'),
				($3, $2, 'session-new', 'click', '192.168.1.1', NOW())
		`, oldID, uID, newID)
		require.NoError(t, err)

		// Run Job
		payload, _ := json.Marshal(map[string]int{"older_than_days": 30})
		job := &queue.Job{ID: uuid.New(), Type: analytics.JobAnonymizeOldData, Payload: payload}

		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
		assert.Contains(t, string(res), "success")

		// Check Old Event (Should be anonymized)
		var oldRow struct {
			IP string     `db:"ip_address"`
			ID *uuid.UUID `db:"user_id"`
			S  string     `db:"session_id"`
		}
		err = db.GetContext(ctx, &oldRow, "SELECT ip_address, user_id, session_id FROM analytics_events WHERE id=$1", oldID)
		require.NoError(t, err)

		assert.Equal(t, "0.0.0.0", oldRow.IP, "IP should be zeroed")
		assert.Nil(t, oldRow.ID, "UserID should be NULL")
		assert.NotEqual(t, "session-old", oldRow.S, "SessionID should be hashed")
		assert.Len(t, oldRow.S, 64, "SessionID hash should be 64 hex chars (sha256)")

		// Check New Event (Should be untouched)
		// FIX: Added struct tag `db:"ip_address"` so sqlx knows how to map it
		var newRow struct {
			IP string `db:"ip_address"`
		}
		err = db.GetContext(ctx, &newRow, "SELECT ip_address FROM analytics_events WHERE id=$1", newID)
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.1", newRow.IP)
	})
}
