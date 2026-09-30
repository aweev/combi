package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	notif_mocks "github.com/combi/pkg/domain/notification/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	nSrc "github.com/combi/notification"
)

// serviceTestSuite encapsulates all mocked dependencies for the main notification.Service.
type serviceTestSuite struct {
	repo       *notif_mocks.MockRepository
	dispatcher *queue_mocks.MockDispatcher
	service    notification.Service
}

// setupService initializes a new test suite with fresh mocks.
func setupService(t *testing.T) serviceTestSuite {
	var ts serviceTestSuite
	ts.repo = notif_mocks.NewMockRepository(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.service = nSrc.NewService(ts.repo, ts.dispatcher, "default")
	return ts
}

func TestService_Dispatch(t *testing.T) {
	ctx := context.Background()
	recipientID := uuid.New()
	toAddress := core.MustNewEmail(t, "test@example.com")
	templateSlug := "welcome-email"
	mockTemplate := &notification.NotificationTemplate{
		ID:       uuid.New(),
		Slug:     templateSlug,
		Channel:  notification.ChannelEmail,
		Category: "onboarding",
		IsActive: true,
	}

	t.Run("should succeed for a valid templated request", func(t *testing.T) {
		ts := setupService(t)
		req := notification.DispatchRequest{
			RecipientID: recipientID,
			ToAddress:   toAddress,
			Template: &notification.TemplatePayload{
				Slug: templateSlug,
				Data: map[string]any{"name": "John"},
			},
		}

		ts.repo.EXPECT().GetTemplateBySlug(ctx, templateSlug).Return(mockTemplate, nil).Once()

		// FIX: Expect direct call to CreateNotificationAndOutboxJob on the main repo mock.
		// Use mock.Anything for pointers to avoid address mismatches.
		ts.repo.EXPECT().CreateNotificationAndOutboxJob(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		// FIX: Removed dispatcher.Dispatch expectation as the service uses Outbox pattern now.

		notif, err := ts.service.Dispatch(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, recipientID, notif.RecipientID)
		assert.Equal(t, notification.StatusPending, notif.Status)
		assert.Equal(t, mockTemplate.ID, *notif.TemplateID)
		assert.Equal(t, mockTemplate.Channel, notif.Channel)
		assert.Equal(t, mockTemplate.Category, notif.Category)
	})

	t.Run("should succeed for a valid raw content request", func(t *testing.T) {
		ts := setupService(t)
		req := notification.DispatchRequest{
			RecipientID: recipientID,
			ToAddress:   toAddress,
			Raw: &notification.RawPayload{
				Channel:  notification.ChannelSMS,
				Category: "alerts",
				Body:     "Your security code is 123456",
			},
		}

		// FIX: Expect direct call to CreateNotificationAndOutboxJob.
		ts.repo.EXPECT().CreateNotificationAndOutboxJob(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		notif, err := ts.service.Dispatch(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Nil(t, notif.TemplateID)
		assert.Equal(t, notification.ChannelSMS, notif.Channel)
		assert.Equal(t, "alerts", notif.Category)
		assert.Equal(t, "Your security code is 123456", notif.Body)
	})

	t.Run("should succeed for a scheduled notification without creating an outbox job", func(t *testing.T) {
		ts := setupService(t)
		scheduledFor := time.Now().Add(1 * time.Hour)
		req := notification.DispatchRequest{
			RecipientID:  recipientID,
			ToAddress:    toAddress,
			ScheduledFor: &scheduledFor,
			Template: &notification.TemplatePayload{
				Slug: templateSlug,
			},
		}

		ts.repo.EXPECT().GetTemplateBySlug(ctx, templateSlug).Return(mockTemplate, nil).Once()

		// FIX: Expect nil for the OutboxJobPayload argument
		ts.repo.EXPECT().CreateNotificationAndOutboxJob(ctx, mock.Anything, (*notification.OutboxJobPayload)(nil), mock.Anything, mock.Anything).Return(nil).Once()

		ts.dispatcher.AssertNotCalled(t, "Dispatch")

		notif, err := ts.service.Dispatch(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.StatusPending, notif.Status)
	})

	t.Run("should fail if recipient ID is missing", func(t *testing.T) {
		ts := setupService(t)
		req := notification.DispatchRequest{
			RecipientID: uuid.Nil,
			ToAddress:   toAddress,
			Template:    &notification.TemplatePayload{Slug: templateSlug},
		}

		_, err := ts.service.Dispatch(ctx, req)

		assert.ErrorIs(t, err, notification.ErrInvalidRecipient)
	})

	t.Run("should fail if template is not found", func(t *testing.T) {
		ts := setupService(t)
		req := notification.DispatchRequest{
			RecipientID: recipientID,
			ToAddress:   toAddress,
			Template:    &notification.TemplatePayload{Slug: "non-existent-template"},
		}

		ts.repo.EXPECT().GetTemplateBySlug(ctx, "non-existent-template").Return(nil, notification.ErrTemplateNotFound).Once()

		_, err := ts.service.Dispatch(ctx, req)

		assert.ErrorIs(t, err, notification.ErrTemplateNotFound)
	})

	t.Run("should fail if template is inactive", func(t *testing.T) {
		ts := setupService(t)
		inactiveTemplate := &notification.NotificationTemplate{ID: uuid.New(), IsActive: false}
		req := notification.DispatchRequest{
			RecipientID: recipientID,
			ToAddress:   toAddress,
			Template:    &notification.TemplatePayload{Slug: "inactive-template"},
		}

		ts.repo.EXPECT().GetTemplateBySlug(ctx, "inactive-template").Return(inactiveTemplate, nil).Once()

		_, err := ts.service.Dispatch(ctx, req)

		assert.ErrorIs(t, err, notification.ErrTemplateInactive)
	})

	t.Run("should return notification object even if job dispatch fails", func(t *testing.T) {
		ts := setupService(t)
		req := notification.DispatchRequest{
			RecipientID: recipientID,
			ToAddress:   toAddress,
			Template:    &notification.TemplatePayload{Slug: templateSlug},
		}

		ts.repo.EXPECT().GetTemplateBySlug(ctx, templateSlug).Return(mockTemplate, nil).Once()

		// Simulate DB error during persist
		ts.repo.EXPECT().CreateNotificationAndOutboxJob(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

		notif, err := ts.service.Dispatch(ctx, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "persist failed")
		require.Nil(t, notif)
	})
}
