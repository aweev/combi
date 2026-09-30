package notification

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Repository defines the complete data persistence contract.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	// Notification & Event operations
	CreateNotification(ctx context.Context, n *Notification) error
	GetNotification(ctx context.Context, id uuid.UUID) (*Notification, error)
	ListUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error)
	UpdateNotification(ctx context.Context, n *Notification) error
	GetNotificationByProviderID(ctx context.Context, providerName, providerID string) (*Notification, error)
	CreateEvent(ctx context.Context, event *NotificationEvent) error
	GetNotificationEvents(ctx context.Context, notificationID uuid.UUID) ([]NotificationEvent, error)
	GetScheduledNotifications(ctx context.Context, dueTime time.Time, limit int) ([]Notification, error)
	DeleteOldNotifications(ctx context.Context, olderThan time.Time) (int64, error)
	DeleteOldEvents(ctx context.Context, olderThan time.Time) (int64, error)

	GetNotificationForUpdate(ctx context.Context, id uuid.UUID) (*Notification, error)

	// Template & Translation operations
	CreateTemplate(ctx context.Context, t *NotificationTemplate) error
	GetTemplate(ctx context.Context, id uuid.UUID) (*NotificationTemplate, error)
	GetTemplateBySlug(ctx context.Context, slug string) (*NotificationTemplate, error)
	ListTemplates(ctx context.Context, limit, offset int) ([]NotificationTemplate, int, error)
	UpdateTemplate(ctx context.Context, t *NotificationTemplate) error
	DeleteTemplate(ctx context.Context, id uuid.UUID) error
	CreateTemplateTranslation(ctx context.Context, tt *TemplateTranslation) error
	GetTemplateTranslations(ctx context.Context, templateID uuid.UUID) ([]TemplateTranslation, error)
	IsTemplateInUse(ctx context.Context, id uuid.UUID) (*NotificationTemplate, error)

	// Provider operations
	CreateProvider(ctx context.Context, p *NotificationProvider) error
	GetProvider(ctx context.Context, id uuid.UUID) (*NotificationProvider, error)
	GetProviderByName(ctx context.Context, name string) (*NotificationProvider, error)
	ListProviders(ctx context.Context, channel *NotificationChannel) ([]NotificationProvider, error)
	UpdateProvider(ctx context.Context, p *NotificationProvider) error
	DeleteProvider(ctx context.Context, id uuid.UUID) error
	FindProviderForDispatch(ctx context.Context, channel NotificationChannel) (*NotificationProvider, error)
	IsProviderInUse(ctx context.Context, id uuid.UUID) (*NotificationProvider, error)

	// Preference operations
	GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	UpsertPreference(ctx context.Context, p *NotificationPreference) (*NotificationPreference, error)

	// Analytics operations
	GetSystemStats(ctx context.Context, start, end time.Time) (*NotificationStats, error)

	CreateNotificationAndOutboxJob(ctx context.Context, n *Notification, job *OutboxJobPayload, queue string, priority int) error

	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}

// ChannelProvider defines the contract for a specific delivery provider (e.g., SMTP).
type ChannelProvider interface {
	Send(ctx context.Context, notification *Notification) (*SendResult, error)
	GetName() string
	GetType() ProviderType
}

// WebhookParser defines the contract for parsing incoming webhooks.
type WebhookParser interface {
	Parse(r *http.Request) ([]ProviderEvent, error)
}

type ProviderEvent struct {
	ProviderMessageID string
	EventType         string // "delivered", "bounced", "failed"
	Timestamp         time.Time
	Details           map[string]any
}
