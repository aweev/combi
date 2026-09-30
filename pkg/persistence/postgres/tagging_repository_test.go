package postgres_test

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/tagging"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaggingRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewTaggingRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Tag_CRUD", func(t *testing.T) {
		id := uuid.New()
		tag := &tagging.Tag{
			ID:          id,
			Name:        "Go Language",
			Slug:        "go",
			Type:        tagging.TypeSkill,
			Description: "Programming",
			Color:       "#00ADD8",
			IsApproved:  true,
		}

		// Create
		require.NoError(t, repo.CreateTag(ctx, tag))

		// Duplicate Slug check (same type)
		assert.ErrorIs(t, repo.CreateTag(ctx, &tagging.Tag{ID: uuid.New(), Name: "X", Slug: "go", Type: tagging.TypeSkill}), tagging.ErrTagAlreadyExists)

		// Duplicate Slug allowed for different type
		require.NoError(t, repo.CreateTag(ctx, &tagging.Tag{ID: uuid.New(), Name: "Go Game", Slug: "go", Type: tagging.TypeInterest}))

		// Get By ID
		got, err := repo.GetTagByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "Go Language", got.Name)

		// Get By Slug
		gotSlug, err := repo.GetTagBySlug(ctx, "go", tagging.TypeSkill)
		require.NoError(t, err)
		assert.Equal(t, id, gotSlug.ID)

		// Search
		results, err := repo.SearchTags(ctx, "Lang", nil, 10)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, id, results[0].ID)

		// Update
		tag.Count = 100
		require.NoError(t, repo.UpdateTag(ctx, tag))

		updated, _ := repo.GetTagByID(ctx, id)
		assert.Equal(t, 100, updated.Count)
	})

	t.Run("Tagging_Operations", func(t *testing.T) {
		// Setup Tags
		tag1ID := uuid.New()
		tag2ID := uuid.New()
		repo.CreateTag(ctx, &tagging.Tag{ID: tag1ID, Name: "A", Slug: "a", Type: "T"})
		repo.CreateTag(ctx, &tagging.Tag{ID: tag2ID, Name: "B", Slug: "b", Type: "T"})

		entityID := uuid.New()
		entityType := "USER"

		// 1. Tag Entity
		added, err := repo.AddTagging(ctx, &tagging.Tagging{TagID: tag1ID, EntityType: entityType, EntityID: entityID})
		require.NoError(t, err)
		assert.True(t, added)

		// 2. Add Same Tag (Idempotent)
		added, err = repo.AddTagging(ctx, &tagging.Tagging{TagID: tag1ID, EntityType: entityType, EntityID: entityID})
		require.NoError(t, err)
		assert.False(t, added)

		// 3. Add Second Tag
		_, err = repo.AddTagging(ctx, &tagging.Tagging{TagID: tag2ID, EntityType: entityType, EntityID: entityID})
		require.NoError(t, err)

		// 4. Get Tags for Entity
		tags, err := repo.GetTagsForEntity(ctx, entityType, entityID)
		require.NoError(t, err)
		assert.Len(t, tags, 2)

		// 5. Get Entities for Tag
		ids, count, err := repo.GetEntityIDsByTag(ctx, tag1ID, entityType, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, entityID, ids[0])

		// 6. Remove Tagging
		require.NoError(t, repo.RemoveTagging(ctx, tag1ID, entityType, entityID))
		tags, _ = repo.GetTagsForEntity(ctx, entityType, entityID)
		assert.Len(t, tags, 1)
		assert.Equal(t, tag2ID, tags[0].ID)
	})

	t.Run("Merge_Tags", func(t *testing.T) {
		// Scenario: Merge "Golang"(Old) into "Go"(New)
		oldTagID := uuid.New()
		newTagID := uuid.New()
		repo.CreateTag(ctx, &tagging.Tag{ID: oldTagID, Name: "Golang", Slug: "golang", Type: "S"})
		repo.CreateTag(ctx, &tagging.Tag{ID: newTagID, Name: "Go", Slug: "go", Type: "S"})

		// User A: Has "Golang" only
		uA := uuid.New()
		repo.AddTagging(ctx, &tagging.Tagging{TagID: oldTagID, EntityType: "U", EntityID: uA})

		// User B: Has "Golang" AND "Go" (Conflict case)
		uB := uuid.New()
		repo.AddTagging(ctx, &tagging.Tagging{TagID: oldTagID, EntityType: "U", EntityID: uB})
		repo.AddTagging(ctx, &tagging.Tagging{TagID: newTagID, EntityType: "U", EntityID: uB})

		// Perform Merge
		err := repo.ReassignTaggings(ctx, oldTagID, newTagID)
		require.NoError(t, err)

		// Verify User A now has "Go" (Reassigned)
		tagsA, _ := repo.GetTagsForEntity(ctx, "U", uA)
		require.Len(t, tagsA, 1)
		assert.Equal(t, newTagID, tagsA[0].ID)

		// Verify User B has "Go" (Duplicate removed, existing kept)
		tagsB, _ := repo.GetTagsForEntity(ctx, "U", uB)
		require.Len(t, tagsB, 1)
		assert.Equal(t, newTagID, tagsB[0].ID)
	})

	t.Run("Count_Update", func(t *testing.T) {
		tid := uuid.New()
		repo.CreateTag(ctx, &tagging.Tag{ID: tid, Name: "C", Slug: "c", Type: "T", Count: 10})

		require.NoError(t, repo.UpdateTagCount(ctx, tid, 1))
		t1, _ := repo.GetTagByID(ctx, tid)
		assert.Equal(t, 11, t1.Count)

		require.NoError(t, repo.UpdateTagCount(ctx, tid, -5))
		t2, _ := repo.GetTagByID(ctx, tid)
		assert.Equal(t, 6, t2.Count)
	})
}
