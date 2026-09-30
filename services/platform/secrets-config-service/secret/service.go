package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Service implements secret.Service (Read-Only path).
type Service struct {
	repo          secret.Repository
	cryptoService security.CryptoService
	audit         audit.EventRecorder
	eventBus      events.Bus
	logger        log.Logger
}

var _ secret.Service = (*Service)(nil)

func NewService(
	repo secret.Repository,
	crypto security.CryptoService,
	audit audit.EventRecorder,
	eventBus events.Bus,
	logger log.Logger,
) secret.Service {
	return &Service{
		repo:          repo,
		cryptoService: crypto,
		audit:         audit,
		eventBus:      eventBus,
		logger:        logger.With(log.String("service", "secret_read")),
	}
}

func (s *Service) GetSecretValue(ctx context.Context, key string) (secret.SecretValue, error) {
	// 1. Fetch the secret metadata and encrypted value from the repository.
	sec, err := s.repo.Get(ctx, key)
	if err != nil {
		if _, ok := err.(*apperror.AppError); ok {
			return nil, err
		}
		return nil, secret.ErrSecretNotFound.WithUnderlying(err)
	}

	// 2. Decrypt the value.
	encCtx, err := sec.GetEncryptionContext()
	if err != nil {
		s.publishDecryptionFailure(ctx, sec, err)
		return nil, secret.ErrDecryptionFailed.WithUnderlying(err)
	}

	plaintext, err := s.cryptoService.Decrypt(ctx, sec.EncryptedValue, encCtx)
	if err != nil {
		s.publishDecryptionFailure(ctx, sec, err)
		return nil, secret.ErrDecryptionFailed.WithUnderlying(err)
	}

	var value secret.SecretValue
	if err := json.Unmarshal(plaintext, &value); err != nil {
		unmarshalErr := fmt.Errorf("failed to unmarshal decrypted secret: %w", err)
		s.publishDecryptionFailure(ctx, sec, unmarshalErr)
		return nil, secret.ErrDecryptionFailed.WithUnderlying(unmarshalErr)
	}

	// 3. Fire an audit event (asynchronously to not block read).
	s.auditAccess(ctx, sec)

	return value, nil
}

func (s *Service) auditAccess(ctx context.Context, sec *secret.Secret) {
	actor := auth.ActorFromContext(ctx)
	actorID := system.SystemActorID
	actorType := system.SystemActorType
	if actor != nil {
		actorID = actor.UserID
		actorType = "user"
	}

	// Audit Event
	event := audit.NewEvent(actorID, actorType, secret.EventSecretAccessed).
		WithSuccess().
		WithTarget(sec.ID, "secret", sec.Name).
		WithContext(ctx).
		WithMetadata(map[string]interface{}{"version": sec.Version}).
		Build()

	// We use a background context because the main request might finish quickly
	go func() {
		if err := s.audit.Record(context.Background(), event); err != nil {
			s.logger.Error("failed to record secret access audit", err)
		}
	}()

	// Domain Event (Optional, for other services to react)
	// In this case, we might not need a full domain event for every access
	// if audit is sufficient, to reduce bus noise.
}

func (s *Service) publishDecryptionFailure(ctx context.Context, sec *secret.Secret, err error) {
	actor := auth.ActorFromContext(ctx)
	actorID := system.SystemActorID
	actorType := system.SystemActorType
	if actor != nil {
		actorID = actor.UserID
		actorType = "user"
	}

	// Domain Event for System Alerting
	payload := secret.SecretDecryptionFailedPayload{
		Name:        sec.Name,
		Version:     sec.Version,
		AttemptedBy: actorID.String(),
		Timestamp:   time.Now().UTC(),
		Error:       err.Error(),
	}

	evt, err := events.NewEvent(secret.EventSecretDecryptionFailed, actorID, actorType, payload)
	if err != nil {
		s.logger.Error("failed to create decryption failure event", err)
		return
	}
	// Fire and forget
	go func() {
		if pubErr := s.eventBus.Publish(context.Background(), evt); pubErr != nil {
			s.logger.Error("failed to publish decryption failure event", pubErr)
		}
	}()
}
