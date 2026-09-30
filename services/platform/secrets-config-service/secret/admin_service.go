package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

type AdminService struct {
	repo          secret.Repository
	cryptoService security.CryptoService
	audit         audit.EventRecorder
	eventBus      events.Bus
	dispatcher    queue.Dispatcher
	cache         cache.Service
	logger        log.Logger
}

var _ secret.AdminService = (*AdminService)(nil)

func NewAdminService(
	repo secret.Repository,
	crypto security.CryptoService,
	audit audit.EventRecorder,
	eventBus events.Bus,
	dispatcher queue.Dispatcher,
	cache cache.Service,
	logger log.Logger,
) secret.AdminService {
	return &AdminService{
		repo:          repo,
		cryptoService: crypto,
		audit:         audit,
		eventBus:      eventBus,
		dispatcher:    dispatcher,
		cache:         cache,
		logger:        logger.With(log.String("service", "secret_admin")),
	}
}
func cacheKey(name string) string {
	return fmt.Sprintf("secret:%s", name)
}
func (s *AdminService) CreateSecret(ctx context.Context, params secret.CreateSecretParams) (*secret.Secret, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 1. Validate & Encrypt
	plaintext, err := json.Marshal(params.Value)
	if err != nil {
		return nil, apperror.New(400, "INVALID_VALUE", "Secret value is not valid JSON.", err)
	}

	ciphertext, encCtx, err := s.cryptoService.Encrypt(ctx, plaintext)
	if err != nil {
		return nil, apperror.New(500, "ENCRYPTION_FAILED", "Failed to encrypt secret value.", err)
	}

	// 2. Build Model
	now := time.Now().UTC()
	newSecret := &secret.Secret{
		ID:             uuid.New(),
		Name:           params.Name,
		EncryptedValue: ciphertext,
		Version:        1,
		Status:         secret.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		RotationPolicy: params.RotationPolicy,
		Metadata:       params.Metadata,
	}
	if err := newSecret.SetEncryptionContext(encCtx); err != nil {
		return nil, apperror.New(500, "INTERNAL_ERROR", "Failed to set encryption context.", err)
	}

	// 3. Persist
	if err := s.repo.Set(ctx, newSecret); err != nil {
		return nil, err
	}

	// 4. Audit
	auditEvent := audit.NewEvent(actor.UserID, "user", secret.EventSecretCreated).
		WithSuccess().
		WithTarget(newSecret.ID, "secret", newSecret.Name).
		WithContext(ctx).Build()

	go func() {
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
		}
	}()

	// 5. Domain Event
	// Use the proper eventbus helper
	domainEvent, err := events.NewEvent(secret.EventSecretCreated, actor.UserID, "user", map[string]string{
		"name": newSecret.Name,
		"id":   newSecret.ID.String(),
	})
	if err != nil {
		return nil, err
	}

	go func() {
		if err := s.eventBus.Publish(context.Background(), domainEvent); err != nil {
			s.logger.Error("failed to publish secret created event", err)
		}
	}()

	s.logger.Info("Secret created", log.String("name", newSecret.Name), log.String("actor_id", actor.UserID.String()))
	return newSecret, nil
}

func (s *AdminService) UpdateSecret(ctx context.Context, key string, params secret.UpdateSecretParams) (*secret.Secret, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 1. Fetch
	existingSecret, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// 2. Apply Updates
	updated := false
	if params.Value != nil {
		plaintext, err := json.Marshal(params.Value)
		if err != nil {
			return nil, apperror.New(400, "INVALID_VALUE", "Secret value is not valid JSON.", err)
		}
		ciphertext, encCtx, err := s.cryptoService.Encrypt(ctx, plaintext)
		if err != nil {
			return nil, apperror.New(500, "ENCRYPTION_FAILED", "Failed to encrypt new secret value.", err)
		}
		existingSecret.EncryptedValue = ciphertext
		if err := existingSecret.SetEncryptionContext(encCtx); err != nil {
			return nil, apperror.New(500, "INTERNAL_ERROR", "Failed to set new encryption context.", err)
		}
		existingSecret.Version++
		updated = true
	}

	if params.RotationPolicy != nil {
		existingSecret.RotationPolicy = params.RotationPolicy
		updated = true
	}
	if params.Metadata != nil {
		existingSecret.Metadata = params.Metadata
		updated = true
	}

	if !updated {
		return s.GetSecretMetadata(ctx, key)
	}

	existingSecret.UpdatedAt = time.Now().UTC()

	// 3. Persist
	if err := s.repo.Set(ctx, existingSecret); err != nil {
		return nil, err
	}

	if err := s.cache.Delete(ctx, cacheKey(key)); err != nil {
		// Log but don't fail the request
		s.logger.Error("Failed to invalidate secret cache on update", err, log.String("key", key))
	}

	// 4. Audit & Events
	auditEvent := audit.NewEvent(actor.UserID, "user", secret.EventSecretUpdated).
		WithSuccess().
		WithTarget(existingSecret.ID, "secret", existingSecret.Name).
		WithContext(ctx).
		WithMetadata(map[string]interface{}{"new_version": existingSecret.Version}).
		Build()

	go func() {
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
		}
	}()

	domainEvent, err := events.NewEvent(secret.EventSecretUpdated, actor.UserID, "user", map[string]interface{}{
		"name":    existingSecret.Name,
		"version": existingSecret.Version,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		if err := s.eventBus.Publish(context.Background(), domainEvent); err != nil {
			s.logger.Error("failed to publish secret updated event", err)
		}
	}()

	return existingSecret, nil
}

func (s *AdminService) GetSecretMetadata(ctx context.Context, key string) (*secret.Secret, error) {
	sec, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	sec.EncryptedValue = nil
	sec.EncryptionContext = nil
	return sec, nil
}

func (s *AdminService) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	return s.repo.List(ctx, prefix)
}

func (s *AdminService) DeleteSecret(ctx context.Context, key string) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	sec, err := s.repo.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, key); err != nil {
		return err
	}

	_ = s.cache.Delete(context.Background(), cacheKey(key))

	auditEvent := audit.NewEvent(actor.UserID, "user", secret.EventSecretDeleted).
		WithSuccess().
		WithTarget(sec.ID, "secret", sec.Name).
		WithContext(ctx).Build()

	go func() {
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
		}
	}()

	domainEvent, err := events.NewEvent(secret.EventSecretDeleted, actor.UserID, "user", map[string]string{"name": key})
	if err != nil {
		return err
	}

	go func() {
		if err := s.eventBus.Publish(context.Background(), domainEvent); err != nil {
			s.logger.Error("failed to publish secret deleted event", err)
		}
	}()

	return nil
}

func (s *AdminService) RotateSecret(ctx context.Context, key string) (*secret.Secret, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 1. Fetch
	sec, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// 2. Dispatch Job
	payload, _ := json.Marshal(map[string]interface{}{"secret_name": sec.Name})
	_, err = s.dispatcher.Dispatch(ctx, secret.JobSecretRotate, payload,
		queue.WithQueue("critical"),
		queue.WithPriority(0),
		queue.WithUniqueKey("secret-rotation:"+sec.ID.String(), 5*time.Minute),
	)
	if err != nil {
		return nil, apperror.New(500, "DISPATCH_FAILED", "Failed to enqueue secret rotation job.", err)
	}

	// 3. Audit
	auditEvent := audit.NewEvent(actor.UserID, "user", secret.EventSecretRotated).
		WithSuccess().
		WithTarget(sec.ID, "secret", sec.Name).
		WithContext(ctx).
		WithMetadata(map[string]interface{}{"message": "Rotation job dispatched"}).
		Build()

	go func() {
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
		}
	}()

	// Return metadata
	sec.EncryptedValue = nil
	sec.EncryptionContext = nil
	return sec, nil
}

func (s *AdminService) SetDispatcher(d queue.Dispatcher) {
	s.dispatcher = d
}
