package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/security"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretRepository_Integration(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewSecretRepository(db)
	ctx := context.Background()

	t.Run("CRUD Operations", func(t *testing.T) {
		// 1. Create
		encCtx, _ := json.Marshal(security.EncryptionContext{Nonce: []byte("123")})
		s := &secret.Secret{
			ID:                uuid.New(),
			Name:              "stripe/prod/key",
			EncryptedValue:    []byte("encrypted-data"),
			EncryptionContext: encCtx,
			Version:           1,
			Status:            secret.StatusActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
			Metadata:          map[string]string{"team": "billing"},
		}

		err := repo.Set(ctx, s)
		require.NoError(t, err)

		// 2. Get
		got, err := repo.Get(ctx, "stripe/prod/key")
		require.NoError(t, err)
		assert.Equal(t, s.ID, got.ID)
		assert.Equal(t, "billing", got.Metadata["team"])

		// 3. List
		// Insert another one
		s2 := &secret.Secret{
			ID:                uuid.New(),
			Name:              "stripe/test/key",
			EncryptedValue:    []byte("data"),
			EncryptionContext: encCtx,
			Version:           1,
			Status:            secret.StatusActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
		require.NoError(t, repo.Set(ctx, s2))

		list, err := repo.List(ctx, "stripe/")
		require.NoError(t, err)
		assert.Contains(t, list, "stripe/prod/key")
		assert.Contains(t, list, "stripe/test/key")

		// 4. Update (Upsert)
		s.Version = 2
		s.Metadata["team"] = "finance"
		err = repo.Set(ctx, s)
		require.NoError(t, err)

		updated, err := repo.Get(ctx, s.Name)
		require.NoError(t, err)
		assert.Equal(t, 2, updated.Version)
		assert.Equal(t, "finance", updated.Metadata["team"])

		// 5. Delete
		err = repo.Delete(ctx, s.Name)
		require.NoError(t, err)

		_, err = repo.Get(ctx, s.Name)
		assert.ErrorIs(t, err, secret.ErrSecretNotFound)
	})

	t.Run("FindSecretsDueForRotation", func(t *testing.T) {
		// Clean up DB or use unique names
		now := time.Now().UTC()
		past := now.Add(-1 * time.Hour)
		future := now.Add(1 * time.Hour)

		encCtx, _ := json.Marshal(security.EncryptionContext{})

		// Secret A: Enabled, Due (Should be found)
		sA := &secret.Secret{
			ID:                uuid.New(),
			Name:              "rot/due",
			EncryptedValue:    []byte("x"),
			EncryptionContext: encCtx,
			Version:           1,
			Status:            secret.StatusActive,
			CreatedAt:         now,
			UpdatedAt:         now,
			RotationPolicy: &secret.RotationPolicy{
				Enabled:        true,
				NextRotationAt: past, // In the past
				Frequency:      24 * time.Hour,
			},
		}
		require.NoError(t, repo.Set(ctx, sA))

		// Secret B: Enabled, Not Due (Should NOT be found)
		sB := &secret.Secret{
			ID:                uuid.New(),
			Name:              "rot/future",
			EncryptedValue:    []byte("x"),
			EncryptionContext: encCtx,
			Version:           1,
			Status:            secret.StatusActive,
			CreatedAt:         now,
			UpdatedAt:         now,
			RotationPolicy: &secret.RotationPolicy{
				Enabled:        true,
				NextRotationAt: future, // In the future
				Frequency:      24 * time.Hour,
			},
		}
		require.NoError(t, repo.Set(ctx, sB))

		// Secret C: Disabled, Due (Should NOT be found)
		sC := &secret.Secret{
			ID:                uuid.New(),
			Name:              "rot/disabled",
			EncryptedValue:    []byte("x"),
			EncryptionContext: encCtx,
			Version:           1,
			Status:            secret.StatusActive,
			CreatedAt:         now,
			UpdatedAt:         now,
			RotationPolicy: &secret.RotationPolicy{
				Enabled:        false,
				NextRotationAt: past,
				Frequency:      24 * time.Hour,
			},
		}
		require.NoError(t, repo.Set(ctx, sC))

		// Perform Search
		results, err := repo.FindSecretsDueForRotation(ctx, now)
		require.NoError(t, err)

		// Assertions
		assert.Len(t, results, 1)
		assert.Equal(t, "rot/due", results[0].Name)
	})
}
