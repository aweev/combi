package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/configuration"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRepository_Settings(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewConfigRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("UpsertAndGet", func(t *testing.T) {
		setting := &configuration.SystemSetting{
			Key:         "test.feature_x",
			Value:       "true",
			Type:        configuration.TypeBoolean,
			Description: "Test Feature",
			IsPublic:    false,
		}

		// 1. Create (Upsert)
		require.NoError(t, repo.UpdateSetting(ctx, setting))

		// 2. Get
		got, err := repo.GetSetting(ctx, setting.Key)
		require.NoError(t, err)
		assert.Equal(t, setting.Value, got.Value)
		assert.Equal(t, setting.Type, got.Type)

		// 3. Update (Upsert)
		setting.Value = "false"
		setting.IsPublic = true
		require.NoError(t, repo.UpdateSetting(ctx, setting))

		gotUpdated, err := repo.GetSetting(ctx, setting.Key)
		require.NoError(t, err)
		assert.Equal(t, "false", gotUpdated.Value)
		assert.True(t, gotUpdated.IsPublic)
	})

	t.Run("ListSettings", func(t *testing.T) {
		list, err := repo.ListSettings(ctx)
		require.NoError(t, err)
		// Should contain seeded settings + the one created above
		assert.NotEmpty(t, list)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := repo.GetSetting(ctx, "non.existent")
		assert.ErrorIs(t, err, configuration.ErrSettingNotFound)
	})
}

func TestConfigRepository_Themes(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewConfigRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t1 := &configuration.Theme{
		ID:        uuid.New(),
		Name:      "Summer Vibe",
		IsSystem:  false,
		Variables: map[string]string{"primary": "orange"},
		CreatedAt: time.Now(),
	}

	t.Run("CreateAndList", func(t *testing.T) {
		require.NoError(t, repo.UpsertTheme(ctx, t1))

		list, err := repo.ListThemes(ctx)
		require.NoError(t, err)
		// Should find t1 + default system theme
		found := false
		for _, th := range list {
			if th.ID == t1.ID {
				found = true
				assert.Equal(t, "orange", th.Variables["primary"])
			}
		}
		assert.True(t, found)
	})

	t.Run("GetTheme", func(t *testing.T) {
		got, err := repo.GetTheme(ctx, t1.ID)
		require.NoError(t, err)
		assert.Equal(t, t1.Name, got.Name)
	})

	t.Run("GetSeasonalTheme", func(t *testing.T) {
		now := time.Now().UTC()
		future := now.Add(1 * time.Hour)
		past := now.Add(-1 * time.Hour)

		seasonal := &configuration.Theme{
			ID:          uuid.New(),
			Name:        "Winter Special",
			IsSystem:    false,
			Variables:   map[string]string{"bg": "white"},
			ActiveFrom:  &past,
			ActiveUntil: &future,
			CreatedAt:   now,
		}
		require.NoError(t, repo.UpsertTheme(ctx, seasonal))

		got, err := repo.GetSeasonalTheme(ctx, now)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, seasonal.ID, got.ID)
	})

	t.Run("DeleteTheme", func(t *testing.T) {
		require.NoError(t, repo.DeleteTheme(ctx, t1.ID))
		_, err := repo.GetTheme(ctx, t1.ID)
		assert.ErrorIs(t, err, configuration.ErrThemeNotFound)
	})

	t.Run("CannotDeleteSystemTheme", func(t *testing.T) {
		// Find default system theme ID from list
		list, _ := repo.ListThemes(ctx)
		var sysID uuid.UUID
		for _, th := range list {
			if th.IsSystem {
				sysID = th.ID
				break
			}
		}

		// Attempt delete
		err := repo.DeleteTheme(ctx, sysID)
		assert.ErrorIs(t, err, configuration.ErrThemeNotFound) // repo returns NotFound if query affects 0 rows
	})
}

func TestConfigRepository_Brands(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewConfigRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Find a valid theme ID first
	themes, _ := repo.ListThemes(ctx)
	validThemeID := themes[0].ID

	orgID := uuid.New()
	brand := configuration.Brand{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		Name:           "Acme Corp",
		ThemeID:        validThemeID,
		CustomOverrides: map[string]string{
			"font": "Roboto",
		},
	}

	t.Run("UpsertAndGet", func(t *testing.T) {
		require.NoError(t, repo.UpsertBrand(ctx, brand))

		// Get by Org ID
		got, err := repo.GetBrand(ctx, &orgID, nil)
		require.NoError(t, err)
		assert.Equal(t, brand.ID, got.ID)
		assert.Equal(t, "Roboto", got.CustomOverrides["font"])
	})

	t.Run("GetSystemBrand", func(t *testing.T) {
		got, err := repo.GetSystemBrand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "NGO Platform", got.Name) // Seeded in migration
	})

	t.Run("GetBrand_NotFound", func(t *testing.T) {
		randomOrg := uuid.New()
		_, err := repo.GetBrand(ctx, &randomOrg, nil)
		assert.ErrorIs(t, err, configuration.ErrBrandNotFound)
	})

	t.Run("DeleteBrand", func(t *testing.T) {
		require.NoError(t, repo.DeleteBrand(ctx, brand.ID))
		_, err := repo.GetBrand(ctx, &orgID, nil)
		assert.ErrorIs(t, err, configuration.ErrBrandNotFound)
	})
}
