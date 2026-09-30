package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type EventListener struct {
	repo   notification.Repository
	logger log.Logger
}

func NewEventListener(repo notification.Repository, logger log.Logger) *EventListener {
	return &EventListener{
		repo:   repo,
		logger: logger.With(log.String("component", "notification_listener")),
	}
}

func (l *EventListener) HandleEmailBounced(ctx context.Context, event events.DomainEvent) error {
	var payload notification.EmailBouncedPayload
	if err := event.UnmarshalPayload(&payload); err != nil {
		return fmt.Errorf("failed to unmarshal bounce payload: %w", err)
	}

	return l.updateStatus(ctx, payload.Provider, payload.ProviderMessageID, notification.StatusBounced, payload.Reason)
}

func (l *EventListener) HandleSMSFailed(ctx context.Context, event events.DomainEvent) error {
	var payload notification.SMSFailedPayload
	if err := event.UnmarshalPayload(&payload); err != nil {
		return fmt.Errorf("failed to unmarshal sms fail payload: %w", err)
	}

	reason := fmt.Sprintf("%s (Code: %s)", payload.ErrorMessage, payload.ErrorCode)
	return l.updateStatus(ctx, payload.Provider, payload.ProviderMessageID, notification.StatusFailed, reason)
}

func (l *EventListener) updateStatus(ctx context.Context, provider, msgID string, status notification.NotificationStatus, reason string) error {
	return l.repo.WithTx(ctx, func(txRepo notification.Repository) error {
		// Find notification by Provider ID
		notif, err := txRepo.GetNotificationByProviderID(ctx, provider, msgID)
		if err != nil {
			if err == notification.ErrNotificationNotFound {
				// This is expected if we receive webhooks for emails sent by other systems sharing the same account
				l.logger.Debug("Received event for unknown notification", log.String("provider_id", msgID))
				return nil
			}
			return err
		}

		// Update Status
		notif.Status = status
		notif.LastError = &reason
		notif.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateNotification(ctx, notif); err != nil {
			return err
		}

		// Log Event
		return txRepo.CreateEvent(ctx, &notification.NotificationEvent{
			ID:             uuid.New(),
			NotificationID: notif.ID,
			Source:         notification.SourceProvider,
			EventType:      string(status),
			Message:        reason,
			CreatedAt:      time.Now().UTC(),
		})
	})
}
