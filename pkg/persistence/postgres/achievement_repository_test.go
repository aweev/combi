package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAchievementRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAchievementRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// --- 1. Metric Concurrency Test (The ACID Test) ---
	t.Run("IncrementMetric Concurrency", func(t *testing.T) {
		userID := uuid.New()
		metric := "volunteer_hours"

		// 50 goroutines adding 1.0 each = 50.0 total
		concurrency := 50
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				// This simulates "Event Bus" triggering concurrent updates
				_, err := repo.IncrementMetric(ctx, userID, metric, 1.0)
				assert.NoError(t, err)
			}()
		}
		wg.Wait()

		// Verify Total
		metrics, err := repo.GetUserMetrics(ctx, userID)
		require.NoError(t, err)
		assert.InDelta(t, float64(concurrency), metrics[metric], 0.01)
	})

	// --- 2. Rules & Badges ---
	t.Run("Badge and Rules", func(t *testing.T) {
		badgeID := uuid.New()
		err := repo.CreateBadge(ctx, &achievement.Badge{
			ID: badgeID, Slug: "test-badge", Name: "Test",
			Description: "Desc", Type: achievement.BadgeTypeMilestone, Points: 10,
		})
		require.NoError(t, err)

		// Create Rule
		target := 100.0
		metric := "hours"
		err = repo.CreateRule(ctx, &achievement.AchievementRule{
			BadgeID: badgeID, RuleType: achievement.RuleTypeAccumulation,
			EventAction: "event.test", MetricName: &metric, Threshold: &target, IsActive: true,
		})
		require.NoError(t, err)

		// Fetch Rule
		rules, err := repo.GetRulesByTrigger(ctx, "event.test")
		require.NoError(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, badgeID, rules[0].BadgeID)
		assert.Equal(t, 100.0, *rules[0].Threshold)

		// Fetch by Badge
		rulesByBadge, err := repo.GetRulesByBadge(ctx, badgeID)
		require.NoError(t, err)
		require.Len(t, rulesByBadge, 1)
	})

	// --- 3. Leaderboard Logic ---
	t.Run("Leaderboard Calculation", func(t *testing.T) {
		u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
		b10 := uuid.New() // 10 points
		b50 := uuid.New() // 50 points

		// Create Badges
		_ = repo.CreateBadge(ctx, &achievement.Badge{ID: b10, Slug: "b10", Name: "10pts", Description: "d", Type: "M", Points: 10})
		_ = repo.CreateBadge(ctx, &achievement.Badge{ID: b50, Slug: "b50", Name: "50pts", Description: "d", Type: "M", Points: 50})

		// Awards:
		// U1: 10 + 50 = 60
		// U2: 50 = 50
		// U3: 10 = 10

		_ = repo.AwardBadge(ctx, u1, b10)
		_ = repo.AwardBadge(ctx, u1, b50)

		_ = repo.AwardBadge(ctx, u2, b50)

		_ = repo.AwardBadge(ctx, u3, b10)

		// Query Leaderboard
		lb, err := repo.GetGlobalLeaderboard(ctx, 10)
		require.NoError(t, err)

		require.Len(t, lb, 3)

		// Rank 1: U1 (60pts)
		assert.Equal(t, u1, lb[0].UserID)
		assert.Equal(t, 60, lb[0].TotalPoints)
		assert.Equal(t, 1, lb[0].Rank)

		// Rank 2: U2 (50pts)
		assert.Equal(t, u2, lb[1].UserID)
		assert.Equal(t, 50, lb[1].TotalPoints)
		assert.Equal(t, 2, lb[1].Rank)

		// Rank 3: U3 (10pts)
		assert.Equal(t, u3, lb[2].UserID)
		assert.Equal(t, 10, lb[2].TotalPoints)
		assert.Equal(t, 3, lb[2].Rank)
	})

	// --- 4. Idempotency ---
	t.Run("Idempotent Awarding", func(t *testing.T) {
		u := uuid.New()
		b := uuid.New()
		_ = repo.CreateBadge(ctx, &achievement.Badge{ID: b, Slug: "uniq", Name: "U", Description: "d", Type: "M", Points: 1})

		// First Award -> OK
		err := repo.AwardBadge(ctx, u, b)
		require.NoError(t, err)

		// Second Award -> ErrAlreadyAwarded (Not duplicate key error)
		err = repo.AwardBadge(ctx, u, b)
		assert.ErrorIs(t, err, achievement.ErrAlreadyAwarded)
	})
}
