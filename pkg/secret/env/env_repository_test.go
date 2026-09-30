package env_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/combi/pkg/domain/secret"
	p_secret "github.com/combi/pkg/secret/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvRepository(t *testing.T) {
	// Setup initial env var state
	initialSecrets := map[string]map[string]interface{}{
		"db/password": {"value": "secret123"},
		"api/key":     {"key": "abc-def"},
	}
	data, _ := json.Marshal(initialSecrets)
	os.Setenv("ngo_SECRETS", string(data))
	defer os.Unsetenv("ngo_SECRETS")

	repo, err := p_secret.NewEnvRepository()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Get Existing", func(t *testing.T) {
		s, err := repo.Get(ctx, "db/password")
		require.NoError(t, err)
		assert.Equal(t, "db/password", s.Name)

		// Verify payload structure (it should be a JSON byte array of the map)
		var val map[string]interface{}
		json.Unmarshal(s.EncryptedValue, &val)
		assert.Equal(t, "secret123", val["value"])
	})

	t.Run("Get Missing", func(t *testing.T) {
		_, err := repo.Get(ctx, "missing/secret")
		assert.ErrorIs(t, err, secret.ErrSecretNotFound)
	})

	t.Run("Set (Update)", func(t *testing.T) {
		s, _ := repo.Get(ctx, "api/key")
		s.Version = 2
		err := repo.Set(ctx, s)
		require.NoError(t, err)

		updated, _ := repo.Get(ctx, "api/key")
		assert.Equal(t, 2, updated.Version)
	})

	t.Run("List", func(t *testing.T) {
		keys, err := repo.List(ctx, "")
		require.NoError(t, err)
		assert.Len(t, keys, 2)
		assert.Contains(t, keys, "db/password")
		assert.Contains(t, keys, "api/key")

		keysPrefix, _ := repo.List(ctx, "db/")
		assert.Len(t, keysPrefix, 1)
		assert.Equal(t, "db/password", keysPrefix[0])
	})

	t.Run("Delete", func(t *testing.T) {
		err := repo.Delete(ctx, "db/password")
		require.NoError(t, err)

		_, err = repo.Get(ctx, "db/password")
		assert.ErrorIs(t, err, secret.ErrSecretNotFound)
	})
}
