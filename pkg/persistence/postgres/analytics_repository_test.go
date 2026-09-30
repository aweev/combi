package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAnalyticsRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Helper to create event
	createEvent := func(eventType string, userID *uuid.UUID, t time.Time) analytics.AnalyticsEvent {
		return analytics.AnalyticsEvent{
			ID:         uuid.New(),
			UserID:     userID,
			SessionID:  "session-" + uuid.NewString(),
			EventType:  eventType,
			CreatedAt:  t,
			Properties: map[string]any{"test": true},
		}
	}

	t.Run("Realtime Query (Raw Data)", func(t *testing.T) {
		// FIX: Use a fixed anchor time (xx:30) to ensure all events (-10m, -5m)
		// fall within the SAME hour bucket (xx:00 - xx:59).
		// Using time.Now() causes flakiness if the test runs near the top of the hour.
		now := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)

		u1 := uuid.New()
		u2 := uuid.New()

		events := []analytics.AnalyticsEvent{
			createEvent("page_view", &u1, now.Add(-10*time.Minute)), // 12:20 -> Bucket 12:00
			createEvent("page_view", &u1, now.Add(-5*time.Minute)),  // 12:25 -> Bucket 12:00
			createEvent("page_view", &u2, now.Add(-1*time.Minute)),  // 12:29 -> Bucket 12:00
		}
		require.NoError(t, repo.InsertBatch(ctx, events))

		// Query Range covers the specific hour
		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		points, err := repo.CountUniqueUsers(ctx, start, end)
		require.NoError(t, err)

		// Since all events are in 12:00 bucket:
		// Bucket 12:00 contains {u1, u2} -> Count 2
		// Total sum should be 2.
		totalUnique := 0.0
		for _, p := range points {
			totalUnique += p.Value
		}
		assert.Equal(t, 2.0, totalUnique)
	})

	t.Run("Hourly Aggregation and Query", func(t *testing.T) {
		// Scenario: Data from 2 days ago.
		targetDate := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Hour)
		u1 := uuid.New()

		// Insert raw data for that day
		events := []analytics.AnalyticsEvent{
			createEvent("login", &u1, targetDate.Add(10*time.Minute)), // +10m
			createEvent("login", &u1, targetDate.Add(20*time.Minute)), // +20m
		}
		require.NoError(t, repo.InsertBatch(ctx, events))

		// RUN AGGREGATION
		err := repo.AggregateStats(ctx, targetDate)
		require.NoError(t, err)

		// Verify Hourly Table Content (Direct SQL check)
		var hourlyCount int
		err = db.GetContext(ctx, &hourlyCount, "SELECT COUNT(*) FROM analytics_hourly_users")
		require.NoError(t, err)
		// We expect at least 1 row (the hour we just inserted)
		assert.GreaterOrEqual(t, hourlyCount, 1, "Should have rows in hourly rollup table")

		// Query via Repository
		// Range > 24h triggers Hourly Logic
		start := targetDate.Add(-2 * time.Hour)
		end := targetDate.Add(30 * time.Hour)

		points, err := repo.CountUniqueUsers(ctx, start, end)
		require.NoError(t, err)

		assert.NotEmpty(t, points)
		foundVal := false
		for _, p := range points {
			if p.Value > 0 {
				foundVal = true
			}
		}
		assert.True(t, foundVal, "Should find user counts in hourly rollups")
	})

	t.Run("Daily Aggregation and Query", func(t *testing.T) {
		// Scenario: Data from 40 days ago triggers Daily Logic.
		targetDate := time.Now().UTC().Add(-40 * 24 * time.Hour).Truncate(24 * time.Hour)
		u1 := uuid.New()
		u2 := uuid.New()

		events := []analytics.AnalyticsEvent{
			createEvent("signup", &u1, targetDate.Add(10*time.Hour)),
			createEvent("signup", &u2, targetDate.Add(11*time.Hour)),
		}
		require.NoError(t, repo.InsertBatch(ctx, events))

		// RUN AGGREGATION
		err := repo.AggregateStats(ctx, targetDate)
		require.NoError(t, err)

		// Verify Daily Table Content
		var dailyCount int
		err = db.GetContext(ctx, &dailyCount, "SELECT count FROM analytics_daily_users WHERE date = $1", targetDate.Format("2006-01-02"))
		require.NoError(t, err)
		assert.Equal(t, 2, dailyCount, "Daily rollup should show 2 users")

		// Query via Repository
		// Range > 32 days triggers Daily Logic
		start := targetDate.Add(-1 * 24 * time.Hour)
		end := time.Now().UTC()

		points, err := repo.CountUniqueUsers(ctx, start, end)
		require.NoError(t, err)

		assert.NotEmpty(t, points)
		// Check that we have a point for that specific day with value 2
		foundDay := false
		for _, p := range points {
			if p.Time.Format("2006-01-02") == targetDate.Format("2006-01-02") && p.Value == 2 {
				foundDay = true
			}
		}
		assert.True(t, foundDay, "Should find daily rollup data via Repository")
	})

	t.Run("Funnel Counts (Raw Data)", func(t *testing.T) {
		now := time.Now().UTC()
		sessionID := "funnel-session"

		events := []analytics.AnalyticsEvent{
			{ID: uuid.New(), SessionID: sessionID, EventType: "view_home", CreatedAt: now.Add(-10 * time.Minute)},
			{ID: uuid.New(), SessionID: sessionID, EventType: "add_cart", CreatedAt: now.Add(-5 * time.Minute)},
		}
		require.NoError(t, repo.InsertBatch(ctx, events))

		steps := []string{"view_home", "add_cart", "checkout"}
		counts, err := repo.GetFunnelCounts(ctx, steps, now.Add(-1*time.Hour), now.Add(1*time.Hour))
		require.NoError(t, err)

		assert.Equal(t, 1, counts["view_home"])
		assert.Equal(t, 1, counts["add_cart"])
		assert.Equal(t, 0, counts["checkout"])
	})
}
