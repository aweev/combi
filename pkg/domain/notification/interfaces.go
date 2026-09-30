package notification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// Service defines the lean, client-facing contract.
type Service interface {
	Dispatch(ctx context.Context, req DispatchRequest) (*Notification, error)
	ListUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error)
	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	UpdateUserPreference(ctx context.Context, pref NotificationPreference) (*NotificationPreference, error)
	CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkAsRead(ctx context.Context, userID uuid.UUID, notificationID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}

// AdminService defines the comprehensive management contract.
type AdminService interface {
	// Notification Lifecycle
	GetNotification(ctx context.Context, id uuid.UUID) (*Notification, error)
	GetNotificationEvents(ctx context.Context, notificationID uuid.UUID) ([]NotificationEvent, error)
	RetryFailedNotification(ctx context.Context, notificationID uuid.UUID) (*Notification, error)

	// Template Management
	CreateTemplate(ctx context.Context, template NotificationTemplate) (*NotificationTemplate, error)
	GetTemplate(ctx context.Context, id uuid.UUID) (*NotificationTemplate, error)
	ListTemplates(ctx context.Context, limit, offset int) ([]NotificationTemplate, int, error)
	UpdateTemplate(ctx context.Context, template NotificationTemplate) (*NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, id uuid.UUID) error

	AddTemplateTranslation(ctx context.Context, translation TemplateTranslation) (*TemplateTranslation, error)

	// Provider Management
	CreateProvider(ctx context.Context, provider NotificationProvider) (*NotificationProvider, error)
	GetProvider(ctx context.Context, id uuid.UUID) (*NotificationProvider, error)
	ListProviders(ctx context.Context, channel *NotificationChannel) ([]NotificationProvider, error)
	UpdateProvider(ctx context.Context, provider NotificationProvider) (*NotificationProvider, error)
	DeleteProvider(ctx context.Context, id uuid.UUID) error

	// Stats
	GetSystemStats(ctx context.Context, start, end time.Time) (*NotificationStats, error)
}

type UserLocaleFetcher interface {
	GetUserLocale(ctx context.Context, userID uuid.UUID) (string, error)
}

// --- DTOs ---
// (Keep DispatchRequest, etc. as they were - they are perfect)
type DispatchRequest struct {
	RecipientID  uuid.UUID
	ToAddress    core.Email
	Locale       *string
	Reference    *NotificationRef
	ScheduledFor *time.Time
	Template     *TemplatePayload `validate:"required_without=Raw"`
	Raw          *RawPayload      `validate:"required_without=Template"`
	OutboxJob    *OutboxJobPayload
}

type OutboxJobPayload struct {
	JobType core.JobType
	Payload json.RawMessage
}

type TemplatePayload struct {
	Slug string
	Data map[string]any
}

type RawPayload struct {
	Channel  NotificationChannel
	Category string
	Subject  *string
	Body     string
}

// --- Internal Helper Interfaces ---
type TemplateRenderer interface {
	Render(ctx context.Context, templateID uuid.UUID, data map[string]any, locale string) (*RenderedTemplate, error)
}

type RenderedTemplate struct {
	Subject *string
	Body    string
}

type PreferenceChecker interface {
	CanSend(ctx context.Context, userID uuid.UUID, channel NotificationChannel, category string, templateID *uuid.UUID) (bool, error)
}
