package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImpactRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewImpactRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// --- 1. Metrics ---
	t.Run("Metrics CRUD", func(t *testing.T) {
		metricID := uuid.New()
		def := &impact.MetricDefinition{
			ID:          metricID,
			Key:         "trees_planted",
			Name:        "Trees Planted",
			Unit:        "trees",
			Type:        impact.MetricTypeCumulative,
			Description: "Reforestation efforts",
			CreatedAt:   time.Now().UTC(),
		}

		// Create
		err := repo.CreateMetric(ctx, def)
		require.NoError(t, err)

		// Get By Key
		got, err := repo.GetMetricByKey(ctx, "trees_planted")
		require.NoError(t, err)
		assert.Equal(t, def.ID, got.ID)
		assert.Equal(t, "Trees Planted", got.Name)

		// List
		list, err := repo.ListMetrics(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	// --- 2. Goals ---
	t.Run("Goals Management", func(t *testing.T) {
		// Prereq: Create Metric
		metricID := uuid.New()
		_ = repo.CreateMetric(ctx, &impact.MetricDefinition{
			ID: metricID, Key: "meals_served", Name: "Meals", Unit: "meals", Type: "CUMULATIVE", CreatedAt: time.Now(),
		})

		goalID := uuid.New()
		goal := &impact.Goal{
			ID:          goalID,
			MetricID:    metricID,
			Name:        "2024 Hunger Relief",
			TargetValue: 10000,
			StartDate:   time.Now().Add(-24 * time.Hour),
			EndDate:     time.Now().Add(24 * time.Hour), // Active
			UpdatedAt:   time.Now(),
		}

		// Create
		err := repo.CreateGoal(ctx, goal)
		require.NoError(t, err)

		// List Active
		activeGoals, err := repo.ListGoals(ctx, true)
		require.NoError(t, err)
		assert.Len(t, activeGoals, 1)
		assert.Equal(t, goalID, activeGoals[0].ID)

		// Update Progress
		err = repo.UpdateGoalProgress(ctx, goalID, 500)
		require.NoError(t, err)

		// Verify Update
		updatedGoals, _ := repo.ListGoals(ctx, false)
		assert.Equal(t, 500.0, updatedGoals[0].CurrentValue)
	})

	// --- 3. Entries & Aggregation ---
	t.Run("Impact Aggregation", func(t *testing.T) {
		// Setup
		metricID := uuid.New()
		org1ID := uuid.New()
		org2ID := uuid.New()

		_ = repo.CreateMetric(ctx, &impact.MetricDefinition{
			ID: metricID, Key: "lives_touched", Name: "Lives", Unit: "people", Type: "CUMULATIVE", CreatedAt: time.Now(),
		})

		baseTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)

		// Insert Entries
		entries := []impact.ImpactEntry{
			{ // Entry 1: Org 1, Jan 1
				ID: uuid.New(), MetricID: metricID, Value: 10,
				OccurredAt: baseTime, CreatedAt: time.Now(),
				OrganizationID: &org1ID, SourceService: "test", SourceID: "1",
			},
			{ // Entry 2: Org 1, Jan 2
				ID: uuid.New(), MetricID: metricID, Value: 20,
				OccurredAt: baseTime.Add(24 * time.Hour), CreatedAt: time.Now(),
				OrganizationID: &org1ID, SourceService: "test", SourceID: "2",
			},
			{ // Entry 3: Org 2, Jan 1
				ID: uuid.New(), MetricID: metricID, Value: 5,
				OccurredAt: baseTime, CreatedAt: time.Now(),
				OrganizationID: &org2ID, SourceService: "test", SourceID: "3",
			},
			{ // Entry 4: No Org (General Public), Jan 1
				ID: uuid.New(), MetricID: metricID, Value: 100,
				OccurredAt: baseTime, CreatedAt: time.Now(),
				SourceService: "test", SourceID: "4",
			},
		}

		for _, e := range entries {
			require.NoError(t, repo.InsertEntry(ctx, &e))
		}

		// Test A: Total Sum (All Time)
		total, err := repo.SumMetric(ctx, metricID, baseTime.Add(-1*time.Hour), baseTime.Add(48*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 135.0, total) // 10 + 20 + 5 + 100

		// Test B: Sum for Org 1 (All Time)
		org1Total, err := repo.SumMetricForOrg(ctx, metricID, org1ID, baseTime.Add(-1*time.Hour), baseTime.Add(48*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 30.0, org1Total) // 10 + 20

		// Test C: Date Filtering (Jan 1 Only)
		jan1Total, err := repo.SumMetric(ctx, metricID, baseTime, baseTime.Add(12*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 115.0, jan1Total) // 10 + 5 + 100 (excludes Jan 2 entry of 20)

		// Test D: Empty Range
		emptyTotal, err := repo.SumMetric(ctx, metricID, baseTime.Add(-100*time.Hour), baseTime.Add(-90*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0.0, emptyTotal)
	})
}
