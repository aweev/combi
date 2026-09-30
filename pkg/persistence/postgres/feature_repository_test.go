// internal/persistence/postgres/feature_repository_test.go
package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestFeatureRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewFeatureRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Helper to create a complex flag
	createFlag := func(key string) feature.Flag {
		return feature.Flag{
			Key:                key,
			Name:               "Test Flag " + key,
			Description:        "A flag for testing JSON persistence",
			Type:               feature.TypeBoolean,
			IsActive:           true,
			UpdatedBy:          uuid.New(),
			UpdatedAt:          time.Now(),
			DefaultVariationID: "var-true",
			OffVariationID:     "var-false",
			Variations: []feature.Variation{
				{
					ID:    "var-true",
					Name:  "True",
					Value: json.RawMessage("true"),
				},
				{
					ID:    "var-false",
					Name:  "False",
					Value: json.RawMessage("false"),
				},
			},
			TargetingRules: []feature.TargetingRule{
				{
					ID:       "rule-1",
					Priority: 1,
					Conditions: []feature.Condition{
						{
							ContextKey: "email",
							Operator:   "CONTAINS",
							Value:      json.RawMessage(`"@ngo-platform.org"`),
						},
					},
					Distribution: feature.Rollout{
						Variations: []feature.WeightedVariation{
							{VariationID: "var-true", Weight: 100},
						},
					},
				},
			},
		}
	}

	t.Run("UpsertAndGet_ComplexJSON", func(t *testing.T) {
		flag := createFlag("feature.test_json")

		// 1. Create
		require.NoError(t, repo.Upsert(ctx, flag))

		// 2. Get
		got, err := repo.Get(ctx, flag.Key)
		require.NoError(t, err)

		assert.Equal(t, flag.Name, got.Name)
		assert.Len(t, got.Variations, 2)
		assert.Len(t, got.TargetingRules, 1)

		// Verify deep JSON structure
		assert.Equal(t, "email", got.TargetingRules[0].Conditions[0].ContextKey)
		assert.Equal(t, feature.OpContains, got.TargetingRules[0].Conditions[0].Operator)
	})

	t.Run("UpdateExisting", func(t *testing.T) {
		flag := createFlag("feature.update_test")
		require.NoError(t, repo.Upsert(ctx, flag))

		// Modify
		flag.IsActive = false
		flag.TargetingRules = []feature.TargetingRule{} // Clear rules

		require.NoError(t, repo.Upsert(ctx, flag))

		updated, err := repo.Get(ctx, flag.Key)
		require.NoError(t, err)
		assert.False(t, updated.IsActive)
		assert.Empty(t, updated.TargetingRules)
	})

	t.Run("ListAll", func(t *testing.T) {
		// Clean start (if using shared DB, key names prevent collision)
		f1 := createFlag("list.1")
		f2 := createFlag("list.2")
		require.NoError(t, repo.Upsert(ctx, f1))
		require.NoError(t, repo.Upsert(ctx, f2))

		list, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 2)
	})

	t.Run("Delete", func(t *testing.T) {
		flag := createFlag("feature.delete_test")
		require.NoError(t, repo.Upsert(ctx, flag))

		require.NoError(t, repo.Delete(ctx, flag.Key))

		_, err := repo.Get(ctx, flag.Key)
		assert.ErrorIs(t, err, feature.ErrFlagNotFound)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := repo.Get(ctx, "non_existent")
		assert.ErrorIs(t, err, feature.ErrFlagNotFound)
	})
}
