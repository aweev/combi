// internal/persistence/postgres/localization_repository_test.go
package postgres_test

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalizationRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewLocalizationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Locales_Seeded", func(t *testing.T) {
		locales, err := repo.GetLocales(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(locales), 2) // en-US and de-DE

		en, err := repo.GetLocale(ctx, localization.LocaleEnUS)
		require.NoError(t, err)
		assert.True(t, en.IsDefault)
	})

	t.Run("Key_CRUD", func(t *testing.T) {
		key := &localization.TranslationKey{
			ID:          uuid.New(),
			Namespace:   "auth",
			Key:         "login_button",
			Description: "Label for login",
		}

		// 1. Create
		require.NoError(t, repo.CreateKey(ctx, key))

		// 2. Duplicate Check
		assert.ErrorIs(t, repo.CreateKey(ctx, key), localization.ErrDuplicateKey)

		// 3. Get
		got, err := repo.GetKey(ctx, "auth", "login_button")
		require.NoError(t, err)
		assert.Equal(t, key.ID, got.ID)

		// 4. List
		list, total, err := repo.ListKeys(ctx, "auth", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, key.Key, list[0].Key)
	})

	t.Run("Value_Upsert_And_Bundle", func(t *testing.T) {
		// Setup Key
		key := &localization.TranslationKey{
			ID:        uuid.New(),
			Namespace: "home",
			Key:       "welcome",
		}
		require.NoError(t, repo.CreateKey(ctx, key))

		// 1. Add Value for EN
		valEN := &localization.TranslationValue{
			KeyID:  key.ID,
			Locale: localization.LocaleEnUS,
			Value:  "Welcome",
		}
		require.NoError(t, repo.UpsertValue(ctx, valEN))

		// 2. Add Value for DE
		valDE := &localization.TranslationValue{
			KeyID:  key.ID,
			Locale: localization.LocaleDeDE,
			Value:  "Willkommen",
		}
		require.NoError(t, repo.UpsertValue(ctx, valDE))

		// 3. Fetch Bundle for EN
		bundle, err := repo.GetValuesForNamespace(ctx, localization.LocaleEnUS, "home")
		require.NoError(t, err)
		assert.Equal(t, "Welcome", bundle["welcome"])

		// 4. Fetch Bundle for DE
		bundleDE, err := repo.GetValuesForNamespace(ctx, localization.LocaleDeDE, "home")
		require.NoError(t, err)
		assert.Equal(t, "Willkommen", bundleDE["welcome"])

		// 5. Update Value
		valEN.Value = "Welcome Back"
		require.NoError(t, repo.UpsertValue(ctx, valEN))

		updated, err := repo.GetValue(ctx, key.ID, localization.LocaleEnUS)
		require.NoError(t, err)
		assert.Equal(t, "Welcome Back", updated.Value)
	})

	t.Run("MissingKeys", func(t *testing.T) {
		// Create a key
		k := &localization.TranslationKey{ID: uuid.New(), Namespace: "test", Key: "missing"}
		require.NoError(t, repo.CreateKey(ctx, k))

		// Only add EN value
		val := &localization.TranslationValue{KeyID: k.ID, Locale: localization.LocaleEnUS, Value: "hi"}
		require.NoError(t, repo.UpsertValue(ctx, val))

		// Check missing for DE
		missing, err := repo.GetKeysMissingValue(ctx, localization.LocaleDeDE)
		require.NoError(t, err)

		found := false
		for _, m := range missing {
			if m.ID == k.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Key should be missing in DE")
	})
}
