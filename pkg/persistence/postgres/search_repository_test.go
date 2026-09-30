package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresSearchEngine(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	engine := postgres.NewPostgresSearchEngine(db, &log.NoOpLogger{})
	ctx := context.Background()

	// 1. Seed Data
	docs := []search.SearchDocument{
		{
			ID:             "user:1",
			Type:           search.IndexUsers,
			Title:          "Alice Wonderland",
			Content:        "Software Engineer with Python skills based in Berlin.",
			Status:         "active",
			AllowedViewers: []string{"public"},
			CreatedAt:      time.Now(),
		},
		{
			ID:             "user:2",
			Type:           search.IndexUsers,
			Title:          "Bob Builder",
			Content:        "Construction manager and Golang enthusiast.",
			Status:         "active",
			AllowedViewers: []string{"public"},
			CreatedAt:      time.Now(),
		},
		{
			ID:             "doc:1",
			Type:           search.IndexContent,
			Title:          "Secret Project",
			Content:        "Top secret python blueprints.",
			Status:         "draft",
			AllowedViewers: []string{"role:admin"}, // Restricted
			CreatedAt:      time.Now(),
		},
	}

	require.NoError(t, engine.IndexBatch(ctx, docs))

	t.Run("Public Search: Full Text", func(t *testing.T) {
		// "Python" should match Alice (content) and Secret Project (content).
		// But viewer is "public", so Secret Project should be hidden.
		req := search.SearchQuery{
			Term:         "Python",
			ViewerScopes: []string{"public"},
			Limit:        10,
		}

		res, err := engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalHits)
		assert.Equal(t, "Alice Wonderland", res.Hits[0].Title)
		// Check highlight contains "Python" wrapped in em tags
		assert.Contains(t, res.Hits[0].Highlights["content"], "<em>Python</em>")
	})

	t.Run("Admin Search: Security Scope", func(t *testing.T) {
		// Admin searches "Python". Should see both.
		req := search.SearchQuery{
			Term:         "Python",
			ViewerScopes: []string{"public", "role:admin"},
			Limit:        10,
		}

		res, err := engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 2, res.TotalHits)
	})

	t.Run("Filter by Index", func(t *testing.T) {
		// Search "Golang" in "content" index (should be 0, Bob is in "users")
		req := search.SearchQuery{
			Term:         "Golang",
			Index:        search.IndexContent,
			ViewerScopes: []string{"public"},
			Limit:        10,
		}
		res, err := engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 0, res.TotalHits)

		// Now search in "users" (should match)
		req.Index = search.IndexUsers
		res, err = engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalHits)
		assert.Equal(t, "Bob Builder", res.Hits[0].Title)
	})

	t.Run("Update Document", func(t *testing.T) {
		// Bob learns Rust
		updatedDoc := docs[1]
		updatedDoc.Content = "Construction manager and Rust enthusiast."
		require.NoError(t, engine.IndexBatch(ctx, []search.SearchDocument{updatedDoc}))

		// Search "Golang" -> 0 hits
		req := search.SearchQuery{Term: "Golang", ViewerScopes: []string{"public"}, Limit: 10}
		res, err := engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 0, res.TotalHits)

		// Search "Rust" -> 1 hit
		req.Term = "Rust"
		res, err = engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, res.TotalHits)
	})

	t.Run("Delete Batch", func(t *testing.T) {
		require.NoError(t, engine.DeleteBatch(ctx, []string{"user:1", "user:2"}))

		req := search.SearchQuery{Term: "", ViewerScopes: []string{"public"}, Limit: 10}
		res, err := engine.Search(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 0, res.TotalHits) // Secret doc is hidden from public
	})
}
