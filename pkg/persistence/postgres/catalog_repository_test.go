package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewCatalogRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 1. Categories (Lifecycle & Relationships)
	// -------------------------------------------------------------------------
	t.Run("Category_Lifecycle", func(t *testing.T) {
		// Create Parent
		parent := &catalog.Category{
			ID:   uuid.New(),
			Name: "Education",
			Slug: "education",
		}
		require.NoError(t, repo.CreateCategory(ctx, parent))

		// Create Child
		childID := uuid.New()
		desc := "Online courses"
		child := &catalog.Category{
			ID:          childID,
			ParentID:    &parent.ID,
			Name:        "Courses",
			Slug:        "education-courses",
			Description: &desc,
		}
		require.NoError(t, repo.CreateCategory(ctx, child))

		// Duplicate Slug
		assert.ErrorIs(t, repo.CreateCategory(ctx, &catalog.Category{ID: uuid.New(), Name: "X", Slug: "education"}), catalog.ErrCategoryAlreadyExists)

		// Get
		got, err := repo.GetCategory(ctx, childID)
		require.NoError(t, err)
		assert.Equal(t, parent.ID, *got.ParentID)
		assert.Equal(t, desc, *got.Description)

		// List
		all, err := repo.ListCategories(ctx)
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	// -------------------------------------------------------------------------
	// 2. Items (CRUD & JSONB)
	// -------------------------------------------------------------------------
	t.Run("Item_CRUD", func(t *testing.T) {
		itemID := uuid.New()
		props := json.RawMessage(`{"duration": "2 weeks"}`)
		tags := json.RawMessage(`["beginner", "tech"]`)

		item := &catalog.Item{
			ID:         itemID,
			Type:       "PROGRAM",
			Name:       "Go Bootcamp",
			Slug:       "go-bootcamp",
			Status:     catalog.StatusAvailable,
			Properties: props,
			Tags:       tags,
		}

		// Create
		require.NoError(t, repo.CreateItem(ctx, item))

		// Get by Slug
		got, err := repo.GetItemBySlug(ctx, "go-bootcamp")
		require.NoError(t, err)
		assert.JSONEq(t, string(props), string(got.Properties))
		assert.JSONEq(t, string(tags), string(got.Tags))

		// Update
		got.Name = "Go Advanced"
		got.Status = catalog.StatusArchived
		require.NoError(t, repo.UpdateItem(ctx, got))

		// Verify Update
		updated, _ := repo.GetItem(ctx, itemID)
		assert.Equal(t, "Go Advanced", updated.Name)
		assert.Equal(t, catalog.StatusArchived, updated.Status)
	})

	// -------------------------------------------------------------------------
	// 3. Item Categorization & Filtering
	// -------------------------------------------------------------------------
	t.Run("Item_Categories", func(t *testing.T) {
		catID := uuid.New()
		require.NoError(t, repo.CreateCategory(ctx, &catalog.Category{ID: catID, Name: "Tech", Slug: "tech"}))

		itemID := uuid.New()
		require.NoError(t, repo.CreateItem(ctx, &catalog.Item{ID: itemID, Name: "A", Slug: "a", Type: "P", Status: catalog.StatusAvailable}))

		// Add Relationship
		require.NoError(t, repo.AddItemToCategory(ctx, itemID, catID))

		// Count Items in Category
		count, err := repo.CountItemsInCategory(ctx, catID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// List Filtered
		items, _, err := repo.ListItems(ctx, catalog.ItemFilter{CategoryID: &catID, Limit: 10})
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, itemID, items[0].ID)

		// Prevent Deletion of Category in use
		err = repo.DeleteCategory(ctx, catID)
		assert.ErrorIs(t, err, catalog.ErrCategoryInUse)

		// Remove relationship
		require.NoError(t, repo.RemoveAllCategoriesFromItem(ctx, itemID))
		count, _ = repo.CountItemsInCategory(ctx, catID)
		assert.Equal(t, 0, count)
	})

	// -------------------------------------------------------------------------
	// 4. Variants
	// -------------------------------------------------------------------------
	t.Run("Variants", func(t *testing.T) {
		itemID := uuid.New()
		repo.CreateItem(ctx, &catalog.Item{ID: itemID, Name: "T-Shirt", Slug: "tshirt", Type: "PRODUCT", Status: catalog.StatusAvailable})

		// Create Variant (using helper created in repo file)
		stock := 50
		v := &catalog.Variant{
			ID:       uuid.New(),
			ItemID:   itemID,
			Name:     "Large",
			Price:    2500,
			Currency: "USD",
			Stock:    &stock,
			IsActive: true,
		}
		require.NoError(t, repo.CreateVariant(ctx, v))

		// List
		variants, err := repo.ListVariants(ctx, itemID)
		require.NoError(t, err)
		assert.Len(t, variants, 1)
		assert.Equal(t, "Large", variants[0].Name)
	})

	// -------------------------------------------------------------------------
	// 5. Applications (Business Rules)
	// -------------------------------------------------------------------------
	t.Run("Applications", func(t *testing.T) {
		itemID := uuid.New()
		repo.CreateItem(ctx, &catalog.Item{ID: itemID, Name: "Job", Slug: "job", Type: "JOB", Status: catalog.StatusAvailable})

		userID := uuid.New()
		app := &catalog.Application{
			ID:      uuid.New(),
			UserID:  userID,
			ItemID:  itemID,
			Status:  catalog.AppStatusPending,
			Answers: json.RawMessage(`{"experience": "5 years"}`),
		}

		// Apply
		require.NoError(t, repo.CreateApplication(ctx, app))

		// Double Apply (Unique Constraint)
		err := repo.CreateApplication(ctx, &catalog.Application{ID: uuid.New(), UserID: userID, ItemID: itemID, Status: catalog.AppStatusPending})
		assert.ErrorIs(t, err, catalog.ErrAlreadyApplied)

		// Get By User/Item
		got, err := repo.GetApplicationByUserAndItem(ctx, userID, itemID)
		require.NoError(t, err)
		assert.Equal(t, app.ID, got.ID)

		// Update
		got.Status = catalog.AppStatusAccepted
		require.NoError(t, repo.UpdateApplication(ctx, got))

		updated, _ := repo.GetApplication(ctx, got.ID)
		assert.Equal(t, catalog.AppStatusAccepted, updated.Status)
	})
}
