package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
)

// Statically verify that Service implements the domain interface.
var _ notification.Service = (*Service)(nil)

// Service implements the lean, client-facing contract for dispatching and managing notifications.
// Its primary responsibility is to create notification records and dispatch jobs for asynchronous processing.
type Service struct {
	repo       notification.Repository
	dispatcher queue.Dispatcher
	queueName  string
}

// NewService creates a new instance of the primary notification service.
func NewService(repo notification.Repository, dispatcher queue.Dispatcher, queueName string) *Service {
	if queueName == "" {
		queueName = "default"
	}
	return &Service{
		repo:       repo,
		dispatcher: dispatcher,
		queueName:  queueName,
	}
}

func (s *Service) Dispatch(ctx context.Context, req notification.DispatchRequest) (*notification.Notification, error) {
	// 1. Validation
	if req.RecipientID == uuid.Nil {
		return nil, notification.ErrInvalidRecipient
	}
	if req.Template == nil && req.Raw == nil {
		return nil, fmt.Errorf("%w: missing payload", notification.ErrInvalidInput)
	}

	// 2. Build Model
	notif, err := s.buildNotificationFromRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// 3. Outbox / Schedule Logic
	var outboxJob *notification.OutboxJobPayload
	isScheduled := req.ScheduledFor != nil && req.ScheduledFor.After(time.Now())

	if !isScheduled {
		// Immediate dispatch via Transactional Outbox
		payload, err := json.Marshal(map[string]string{"notification_id": notif.ID.String()})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal outbox payload: %w", err)
		}
		outboxJob = &notification.OutboxJobPayload{
			JobType: notification.JobSendNotification,
			Payload: payload,
		}
	}

	// 4. Transactional Persist
	// This method saves the Notification AND the Job (if outboxJob is not nil) in one TX.
	if err := s.repo.CreateNotificationAndOutboxJob(ctx, notif, outboxJob, s.queueName, 5); err != nil {
		return nil, fmt.Errorf("persist failed: %w", err)
	}

	// 5. (Optional Optimization) Immediate Dispatch Signal
	// If you have a real-time requirement, you can signal the queue here.
	// But `OutboxProcessor` will pick it up shortly anyway.

	return notif, nil
}

// ListUserNotifications retrieves a paginated list of notifications for a specific user.
func (s *Service) ListUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]notification.Notification, int, error) {
	// Business rule: An actor can only list their own notifications, unless they have a specific admin permission.
	// This logic would typically live here or in an API gateway layer.
	// For now, we assume the check has passed.
	return s.repo.ListUserNotifications(ctx, userID, limit, offset)
}

// GetUserPreferences retrieves all notification preferences for a given user.
func (s *Service) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]notification.NotificationPreference, error) {
	return s.repo.GetUserPreferences(ctx, userID)
}

// UpdateUserPreference creates or updates a user's choice about receiving notifications.
func (s *Service) UpdateUserPreference(ctx context.Context, pref notification.NotificationPreference) (*notification.NotificationPreference, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != pref.UserID {
		return nil, notification.ErrUnauthorized
	}

	updatedPref, err := s.repo.UpsertPreference(ctx, &pref)
	if err != nil {
		return nil, err
	}

	return updatedPref, nil
}

// --- Private Helper Methods ---

func (s *Service) buildNotificationFromRequest(ctx context.Context, req notification.DispatchRequest) (*notification.Notification, error) {
	now := time.Now().UTC()
	notif := &notification.Notification{
		ID:          uuid.New(),
		RecipientID: req.RecipientID,
		ToAddress:   req.ToAddress,
		Status:      notification.StatusPending,
		Reference:   req.Reference,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.Template != nil {
		template, err := s.repo.GetTemplateBySlug(ctx, req.Template.Slug)
		if err != nil {
			return nil, err // Likely ErrTemplateNotFound
		}
		if !template.IsActive {
			return nil, notification.ErrTemplateInactive
		}
		notif.TemplateID = &template.ID
		notif.Channel = template.Channel
		notif.Category = template.Category
		notif.Data = req.Template.Data
	} else {
		notif.Channel = req.Raw.Channel
		notif.Category = req.Raw.Category
		notif.Subject = req.Raw.Subject
		notif.Body = req.Raw.Body
	}
	return notif, nil
}

func (s *Service) CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}

func (s *Service) MarkAsRead(ctx context.Context, userID uuid.UUID, notificationID uuid.UUID) error {
	// 1. Fetch to verify ownership
	notif, err := s.repo.GetNotification(ctx, notificationID)
	if err != nil {
		return err
	}
	if notif.RecipientID != userID {
		return notification.ErrUnauthorized
	}

	// 2. Update
	now := time.Now().UTC()
	notif.Status = notification.StatusRead
	notif.ReadAt = &now
	notif.UpdatedAt = now

	return s.repo.UpdateNotification(ctx, notif)
}

func (s *Service) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}
