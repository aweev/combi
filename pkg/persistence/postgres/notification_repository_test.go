package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepository_CRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewNotificationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CreateAndGet", func(t *testing.T) {
		email, _ := core.NewEmail("test@example.com")
		n := &notification.Notification{
			ID:          uuid.New(),
			RecipientID: uuid.New(),
			Channel:     notification.ChannelEmail,
			Status:      notification.StatusPending,
			Subject:     stringPointer("Hello"),
			Body:        "Test body",
			Data:        map[string]any{"key": "val"},
			ToAddress:   email,
			Category:    "transactional",
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, repo.CreateNotification(ctx, n))

		got, err := repo.GetNotification(ctx, n.ID)
		require.NoError(t, err)
		assert.Equal(t, n.Subject, got.Subject)
		assert.Equal(t, n.ToAddress.String(), got.ToAddress.String())
	})

	t.Run("Update", func(t *testing.T) {
		email, _ := core.NewEmail("upd@example.com")
		n := &notification.Notification{
			ID:          uuid.New(),
			RecipientID: uuid.New(),
			Channel:     notification.ChannelEmail,
			Status:      notification.StatusPending,
			Body:        "Body",
			ToAddress:   email,
			Category:    "cat",
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, repo.CreateNotification(ctx, n))

		n.Status = notification.StatusSent
		now := time.Now().UTC()
		n.SentAt = &now
		n.ProviderName = "SMTP"
		require.NoError(t, repo.UpdateNotification(ctx, n))

		got, err := repo.GetNotification(ctx, n.ID)
		require.NoError(t, err)
		assert.Equal(t, notification.StatusSent, got.Status)
		assert.Equal(t, "SMTP", got.ProviderName)
	})
}

func TestNotificationRepository_Template(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewNotificationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("TemplateLifecycle", func(t *testing.T) {
		tmpl := &notification.NotificationTemplate{
			ID:        uuid.New(),
			Name:      "Welcome",
			Slug:      "welcome-v1",
			Type:      notification.TemplateTypeTransactional,
			Category:  "onboarding",
			Channel:   notification.ChannelEmail,
			IsActive:  true,
			Locale:    "en-US",
			Subject:   stringPointer("Welcome!"),
			Body:      "<h1>Hi</h1>",
			CreatedBy: uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, repo.CreateTemplate(ctx, tmpl))

		got, err := repo.GetTemplateBySlug(ctx, "welcome-v1")
		require.NoError(t, err)
		assert.Equal(t, tmpl.ID, got.ID)

		// Update
		tmpl.Body = "<h2>Hi Updated</h2>"
		require.NoError(t, repo.UpdateTemplate(ctx, tmpl))

		updated, err := repo.GetTemplate(ctx, tmpl.ID)
		require.NoError(t, err)
		assert.Equal(t, "<h2>Hi Updated</h2>", updated.Body)
	})
}

func TestNotificationRepository_Provider(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewNotificationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("ProviderCRUD", func(t *testing.T) {
		prov := &notification.NotificationProvider{
			ID:       uuid.New(),
			Name:     "Primary SMTP",
			Type:     notification.ProviderSMTP,
			Channel:  notification.ChannelEmail,
			IsActive: true,
			Config: notification.EncryptedConfig{
				EncryptedData:     []byte("secret"),
				EncryptionContext: &security.EncryptionContext{KeyID: "k1"},
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, repo.CreateProvider(ctx, prov))

		got, err := repo.GetProviderByName(ctx, "Primary SMTP")
		require.NoError(t, err)
		assert.Equal(t, prov.ID, got.ID)

		list, err := repo.ListProviders(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})
}

func TestNotificationRepository_Events(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewNotificationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup Parent Notification
	email, _ := core.NewEmail("evt@test.com")
	n := &notification.Notification{
		ID:          uuid.New(),
		RecipientID: uuid.New(),
		Channel:     notification.ChannelEmail,
		Status:      notification.StatusPending,
		Body:        "b",
		ToAddress:   email,
		Category:    "c",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, repo.CreateNotification(ctx, n))

	t.Run("EventLog", func(t *testing.T) {
		evt := &notification.NotificationEvent{
			ID:             uuid.New(),
			NotificationID: n.ID,
			Source:         notification.SourceInternal,
			EventType:      "queued",
			Message:        "Enqueued for delivery",
			Details:        map[string]any{"job_id": "123"},
			CreatedAt:      time.Now().UTC(),
		}
		require.NoError(t, repo.CreateEvent(ctx, evt))

		events, err := repo.GetNotificationEvents(ctx, n.ID)
		require.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "queued", events[0].EventType)
	})
}

func stringPointer(s string) *string {
	return &s
}
