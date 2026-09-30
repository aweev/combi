package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestIntegration(t *testing.T, provider core.ProviderType) *integration.Integration {
	t.Helper()
	return &integration.Integration{
		ID:                   uuid.New(),
		Provider:             provider,
		Name:                 "Test " + string(provider),
		Status:               integration.StatusActive,
		CredentialsEncrypted: []byte("fake-encrypted-data"),
		// Corrected: Removed 'Algorithm' if your struct doesn't support it,
		// or ensure internal/domain/security/types.go matches the definition above.
		EncryptionContext: &security.EncryptionContext{KeyID: "key-1"},
		Configuration:     json.RawMessage(`{"webhook_secret": "wh_123"}`),
		CreatedAt:         time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:         time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestIntegrationRepository_IntegrationCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewIntegrationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CreateAndGet", func(t *testing.T) {
		integ := newTestIntegration(t, core.ProviderStripe)
		require.NoError(t, repo.CreateIntegration(ctx, integ))

		got, err := repo.GetIntegration(ctx, integ.ID)
		require.NoError(t, err)
		assert.Equal(t, integ.Name, got.Name)
		assert.Equal(t, integ.Provider, got.Provider)
		assert.Equal(t, integ.EncryptionContext.KeyID, got.EncryptionContext.KeyID)
	})

	t.Run("GetActive", func(t *testing.T) {
		// Create an old one
		oldInteg := newTestIntegration(t, core.ProviderBrevo)
		oldInteg.UpdatedAt = time.Now().Add(-24 * time.Hour)
		require.NoError(t, repo.CreateIntegration(ctx, oldInteg))

		// Create a new one (should be retrieved)
		newInteg := newTestIntegration(t, core.ProviderBrevo)
		require.NoError(t, repo.CreateIntegration(ctx, newInteg))

		// Create an inactive one (should be ignored)
		inactive := newTestIntegration(t, core.ProviderBrevo)
		inactive.Status = integration.StatusInactive
		require.NoError(t, repo.CreateIntegration(ctx, inactive))

		got, err := repo.GetActiveIntegrationByProvider(ctx, core.ProviderBrevo)
		require.NoError(t, err)
		assert.Equal(t, newInteg.ID, got.ID)
	})

	t.Run("Update", func(t *testing.T) {
		integ := newTestIntegration(t, core.ProviderStripe)
		require.NoError(t, repo.CreateIntegration(ctx, integ))

		integ.Status = integration.StatusError
		errStr := "connection timeout"
		integ.LastError = &errStr
		now := time.Now().UTC().Truncate(time.Microsecond)
		integ.LastErrorAt = &now

		require.NoError(t, repo.UpdateIntegration(ctx, integ))

		got, err := repo.GetIntegration(ctx, integ.ID)
		require.NoError(t, err)
		assert.Equal(t, integration.StatusError, got.Status)
		assert.Equal(t, errStr, *got.LastError)
	})

	t.Run("Delete", func(t *testing.T) {
		integ := newTestIntegration(t, core.ProviderStripe)
		require.NoError(t, repo.CreateIntegration(ctx, integ))

		require.NoError(t, repo.DeleteIntegration(ctx, integ.ID))
		_, err := repo.GetIntegration(ctx, integ.ID)
		assert.ErrorIs(t, err, integration.ErrIntegrationNotFound)
	})

	t.Run("List", func(t *testing.T) {
		// Clean existing by relying on new context or check relative counts
		list, _, err := repo.ListIntegrations(ctx, integration.IntegrationFilter{Limit: 100})
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})
}

func TestIntegrationRepository_WebhookCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewIntegrationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	integ := newTestIntegration(t, core.ProviderStripe)
	require.NoError(t, repo.CreateIntegration(ctx, integ))

	t.Run("CreateAndGet", func(t *testing.T) {
		webhook := &integration.Webhook{
			ID:              uuid.New(),
			IntegrationID:   integ.ID,
			Provider:        core.ProviderStripe,
			ProviderEventID: "evt_123",
			EventType:       "payment.succeeded",
			Status:          integration.WebhookPending,
			Payload:         json.RawMessage(`{"amount": 100}`),
			ReceivedAt:      time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.CreateWebhook(ctx, webhook))

		got, err := repo.GetWebhook(ctx, webhook.ID)
		require.NoError(t, err)
		assert.Equal(t, webhook.ProviderEventID, got.ProviderEventID)
		assert.Equal(t, webhook.IntegrationID, got.IntegrationID)

		// Test Idempotency lookup
		gotByIdem, err := repo.GetWebhookByProviderID(ctx, core.ProviderStripe, "evt_123")
		require.NoError(t, err)
		assert.Equal(t, webhook.ID, gotByIdem.ID)
	})

	t.Run("DuplicateWebhook", func(t *testing.T) {
		wh1 := &integration.Webhook{
			ID:              uuid.New(),
			IntegrationID:   integ.ID,
			Provider:        core.ProviderStripe,
			ProviderEventID: "evt_duplicate",
			EventType:       "test",
			Status:          integration.WebhookPending,
			Payload:         json.RawMessage("{}"),
			ReceivedAt:      time.Now().UTC(),
		}
		require.NoError(t, repo.CreateWebhook(ctx, wh1))

		// Same provider ID
		wh2 := *wh1
		wh2.ID = uuid.New()
		err := repo.CreateWebhook(ctx, &wh2)
		assert.ErrorIs(t, err, integration.ErrDuplicateWebhook)
	})

	t.Run("UpdateWebhook", func(t *testing.T) {
		wh := &integration.Webhook{
			ID:              uuid.New(),
			IntegrationID:   integ.ID,
			Provider:        core.ProviderStripe,
			ProviderEventID: "evt_update",
			EventType:       "test",
			Status:          integration.WebhookPending,
			Payload:         json.RawMessage("{}"),
			ReceivedAt:      time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.CreateWebhook(ctx, wh))

		processedAt := time.Now().UTC().Truncate(time.Microsecond)
		wh.Status = integration.WebhookProcessed
		wh.ProcessedAt = &processedAt
		wh.RetryCount = 1

		require.NoError(t, repo.UpdateWebhook(ctx, wh))

		got, err := repo.GetWebhook(ctx, wh.ID)
		require.NoError(t, err)
		assert.Equal(t, integration.WebhookProcessed, got.Status)
		assert.NotNil(t, got.ProcessedAt)
	})
}
