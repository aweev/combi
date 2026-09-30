// internal/persistence/postgres/content_repository_test.go
package postgres_test

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewContentRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	var spaceID, modelID uuid.UUID

	t.Run("Setup_Structure", func(t *testing.T) {
		// Create Space
		s := &content.Space{
			ID:            uuid.New(),
			Key:           "blog",
			Name:          "Blog",
			Locales:       []string{"en-US"},
			DefaultLocale: "en-US",
		}
		require.NoError(t, repo.CreateSpace(ctx, s))
		spaceID = s.ID

		// Create Model
		m := &content.Model{
			ID:      uuid.New(),
			SpaceID: spaceID,
			APIKey:  "post",
			Name:    "Post",
			Fields: []content.Field{
				{ID: "title", Name: "Title", Type: content.FieldTypeText},
			},
		}
		require.NoError(t, repo.CreateModel(ctx, m))
		modelID = m.ID
	})

	t.Run("Entry_Lifecycle", func(t *testing.T) {
		user := uuid.New()
		fields := map[string]map[string]any{
			"en-US": {"title": "Hello World"},
		}

		// 1. Create Entry
		e := &content.Entry{
			ID:        uuid.New(),
			ModelID:   modelID,
			SpaceID:   spaceID,
			Status:    content.StatusDraft,
			Fields:    fields,
			CreatedBy: user,
			UpdatedBy: user,
		}
		require.NoError(t, repo.CreateEntry(ctx, e))

		// 2. Create Version
		v1 := &content.Version{
			EntryID:   e.ID,
			Version:   1,
			Fields:    fields,
			CreatedBy: user,
		}
		require.NoError(t, repo.CreateVersion(ctx, v1))

		// 3. Update Entry
		e.Fields["en-US"]["title"] = "Hello V2"
		e.Version = 2
		e.Status = content.StatusPublished
		require.NoError(t, repo.UpdateEntry(ctx, e))

		// 4. Get Latest Version
		got, err := repo.GetEntry(ctx, e.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, got.Version)
		assert.Equal(t, "Hello V2", got.Fields["en-US"]["title"])

		// 5. Check History
		history, err := repo.ListVersionsForEntry(ctx, e.ID)
		require.NoError(t, err)
		assert.Len(t, history, 1) // Only created v1 explicitly
		assert.Equal(t, 1, history[0].Version)
	})
}
