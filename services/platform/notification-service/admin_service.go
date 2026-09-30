package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ notification.AdminService = (*AdminService)(nil)

type AdminService struct {
	repo          notification.Repository
	dispatcher    queue.Dispatcher
	auditRecorder audit.EventRecorder
	crypto        security.CryptoService
	logger        log.Logger
}

func NewAdminService(repo notification.Repository, dispatcher queue.Dispatcher, auditRecorder audit.EventRecorder, crypto security.CryptoService, logger log.Logger) *AdminService {
	return &AdminService{
		repo:          repo,
		dispatcher:    dispatcher,
		auditRecorder: auditRecorder,
		crypto:        crypto,
		logger:        logger.With(log.String("service", "notification_admin")),
	}
}

// --- Notification Lifecycle ---

func (s *AdminService) GetNotification(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	return s.repo.GetNotification(ctx, id)
}

func (s *AdminService) GetNotificationEvents(ctx context.Context, notificationID uuid.UUID) ([]notification.NotificationEvent, error) {
	return s.repo.GetNotificationEvents(ctx, notificationID)
}

func (s *AdminService) RetryFailedNotification(ctx context.Context, notificationID uuid.UUID) (*notification.Notification, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}

	var retriedNotification *notification.Notification
	err := s.repo.WithTx(ctx, func(txRepo notification.Repository) error {
		n, err := txRepo.GetNotification(ctx, notificationID)
		if err != nil {
			return err
		}
		if n.Status != notification.StatusFailed && n.Status != notification.StatusBounced {
			return fmt.Errorf("%w: cannot retry notification in '%s' status", notification.ErrInvalidInput, n.Status)
		}
		n.Status = notification.StatusPending
		n.Attempts = 0
		n.LastError = nil
		n.UpdatedAt = time.Now().UTC()
		if err := txRepo.UpdateNotification(ctx, n); err != nil {
			return err
		}
		retriedNotification = n
		return nil
	})
	if err != nil {
		return nil, err
	}

	jobPayload, _ := json.Marshal(map[string]string{"notification_id": retriedNotification.ID.String()})
	_, err = s.dispatcher.Dispatch(ctx, notification.JobSendNotification, jobPayload, queue.WithQueue("notifications"))
	if err != nil {
		s.logger.Error("Failed to dispatch retry job", err, log.String("notification_id", retriedNotification.ID.String()))
		return nil, fmt.Errorf("failed to dispatch retry job: %w", err)
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventNotificationRetried).
		WithSuccess().
		WithTarget(retriedNotification.ID, "notification", retriedNotification.RecipientID.String()). // CHANGED
		WithMetadata(map[string]interface{}{
			"original_status": "FAILED", // or capture from before update
			"channel":         retriedNotification.Channel,
		}).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return retriedNotification, nil
}

// --- Template Management ---

func (s *AdminService) CreateTemplate(ctx context.Context, template notification.NotificationTemplate) (*notification.NotificationTemplate, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}

	template.ID = uuid.New()
	template.CreatedBy = actor.UserID
	template.CreatedAt = time.Now().UTC()
	template.UpdatedAt = time.Now().UTC()

	if err := s.repo.CreateTemplate(ctx, &template); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventTemplateCreated).
		WithSuccess().
		WithTarget(template.ID, "notification_template", template.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return &template, nil
}
func (s *AdminService) GetTemplate(ctx context.Context, id uuid.UUID) (*notification.NotificationTemplate, error) {
	return s.repo.GetTemplate(ctx, id)
}

func (s *AdminService) ListTemplates(ctx context.Context, limit, offset int) ([]notification.NotificationTemplate, int, error) {
	return s.repo.ListTemplates(ctx, limit, offset)
}

func (s *AdminService) UpdateTemplate(ctx context.Context, template notification.NotificationTemplate) (*notification.NotificationTemplate, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}
	existing, err := s.repo.GetTemplate(ctx, template.ID)
	if err != nil {
		return nil, err
	}

	existing.Name = template.Name
	existing.Slug = template.Slug
	existing.Description = template.Description
	existing.Type = template.Type
	existing.Category = template.Category
	existing.Channel = template.Channel
	existing.IsActive = template.IsActive
	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateTemplate(ctx, existing); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventTemplateUpdated).
		WithSuccess().
		WithTarget(existing.ID, "notification_template", existing.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return existing, nil
}

func (s *AdminService) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return notification.ErrUnauthorized
	}

	template, err := s.repo.GetTemplate(ctx, id)
	if err != nil {
		return err
	}

	// Business Logic: Prevent deletion of templates that are in use.
	inUseTemplate, err := s.repo.IsTemplateInUse(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check if template is in use: %w", err)
	}
	if inUseTemplate != nil {
		return fmt.Errorf("%w: cannot delete template '%s' as it is in use", notification.ErrInvalidInput, template.Name)
	}

	if err := s.repo.DeleteTemplate(ctx, id); err != nil {
		return err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventTemplateDeleted).
		WithSuccess().
		WithTarget(id, "notification_template", template.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return nil
}

func (s *AdminService) AddTemplateTranslation(ctx context.Context, translation notification.TemplateTranslation) (*notification.TemplateTranslation, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}

	template, err := s.repo.GetTemplate(ctx, translation.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("parent template not found: %w", err)
	}

	translation.ID = uuid.New()
	translation.CreatedAt = time.Now().UTC()
	translation.UpdatedAt = time.Now().UTC()

	if err := s.repo.CreateTemplateTranslation(ctx, &translation); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventTemplateTranslationAdded).
		WithSuccess().
		WithTarget(template.ID, "notification_template", fmt.Sprintf("%s (%s translation)", template.Name, translation.Locale)).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return &translation, nil
}

func (s *AdminService) GetSystemStats(ctx context.Context, start, end time.Time) (*notification.NotificationStats, error) {
	return s.repo.GetSystemStats(ctx, start, end)
}

func (s *AdminService) CreateProvider(ctx context.Context, provider notification.NotificationProvider) (*notification.NotificationProvider, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}

	provider.ID = uuid.New()
	provider.CreatedAt = time.Now().UTC()
	provider.UpdatedAt = time.Now().UTC()

	// Encrypt the configuration (API Keys) before saving.
	// We assume provider.Config.EncryptedData currently holds the RAW JSON bytes from the handler.
	if len(provider.Config.EncryptedData) > 0 {
		encrypted, encCtx, err := s.crypto.Encrypt(ctx, provider.Config.EncryptedData)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt provider config: %w", err)
		}
		provider.Config.EncryptedData = encrypted
		provider.Config.EncryptionContext = encCtx
	}

	if err := s.repo.CreateProvider(ctx, &provider); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventProviderCreated).
		WithSuccess().
		WithTarget(provider.ID, "notification_provider", provider.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return &provider, nil
}

func (s *AdminService) GetProvider(ctx context.Context, id uuid.UUID) (*notification.NotificationProvider, error) {
	provider, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	// We do NOT decrypt the config on read to prevent leaking keys in API responses.
	// If admin needs to "view" config, we typically mask it or require a separate explicit "reveal" endpoint.
	provider.Config.EncryptedData = nil
	provider.Config.EncryptionContext = nil
	return provider, nil
}

func (s *AdminService) ListProviders(ctx context.Context, channel *notification.NotificationChannel) ([]notification.NotificationProvider, error) {
	providers, err := s.repo.ListProviders(ctx, channel)
	if err != nil {
		return nil, err
	}
	// Sanitize config
	for i := range providers {
		providers[i].Config.EncryptedData = nil
		providers[i].Config.EncryptionContext = nil
	}
	return providers, nil
}

func (s *AdminService) UpdateProvider(ctx context.Context, provider notification.NotificationProvider) (*notification.NotificationProvider, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, notification.ErrUnauthorized
	}

	existing, err := s.repo.GetProvider(ctx, provider.ID)
	if err != nil {
		return nil, err
	}

	// Only update fields if provided.
	// If new config is provided (in EncryptedData slot as raw bytes), encrypt it.
	if len(provider.Config.EncryptedData) > 0 {
		encrypted, encCtx, err := s.crypto.Encrypt(ctx, provider.Config.EncryptedData)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt new provider config: %w", err)
		}
		existing.Config.EncryptedData = encrypted
		existing.Config.EncryptionContext = encCtx
	}

	existing.IsActive = provider.IsActive
	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateProvider(ctx, existing); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventProviderUpdated).
		WithSuccess().
		WithTarget(existing.ID, "notification_provider", existing.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return existing, nil
}

func (s *AdminService) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return notification.ErrUnauthorized
	}

	provider, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return err
	}

	// Business Rule: Prevent deleting a provider if it's the last one for a channel or in use.
	// This check can be complex, for now we allow delete if Repo allows it.

	if err := s.repo.DeleteProvider(ctx, id); err != nil {
		return err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventProviderDeleted).
		WithSuccess().
		WithTarget(id, "notification_provider", provider.Name).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return nil
}
