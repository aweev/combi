package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// RotationWorker handles the background job for rotating secrets.
type RotationWorker struct {
	repo          secret.Repository
	cryptoService security.CryptoService
	audit         audit.EventRecorder
	eventBus      events.Bus
	cache         cache.Service
	factory       secret.RotatorFactory
	logger        log.Logger
}

// NewRotationWorker creates a new instance of the rotation worker.
func NewRotationWorker(
	repo secret.Repository,
	crypto security.CryptoService,
	audit audit.EventRecorder,
	bus events.Bus,
	cache cache.Service,
	factory secret.RotatorFactory,
	logger log.Logger,
) *RotationWorker {
	return &RotationWorker{
		repo:          repo,
		cryptoService: crypto,
		audit:         audit,
		eventBus:      bus,
		cache:         cache,
		factory:       factory,
		logger:        logger.With(log.String("worker", string(secret.JobSecretRotate))),
	}
}

func (w *RotationWorker) JobType() core.JobType {
	return secret.JobSecretRotate
}

type rotationPayload struct {
	SecretName string `json:"secret_name"` // Changed from SecretID (string) to SecretName for repository lookup
}

// ProcessJob performs the secret rotation.
func (w *RotationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p rotationPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid rotation payload: %w", err))
	}

	logger := w.logger.With(log.String("job_id", job.ID.String()), log.String("secret_name", p.SecretName))

	// 1. Get the current secret.
	currentSecret, err := w.repo.Get(ctx, p.SecretName)
	if err != nil {
		// If secret is missing, this is non-retryable (it was deleted).
		// If repo error (db down), it is retryable.
		// Assuming repo returns generic error for not found, we might need to differentiate.
		// For now, treat as retryable unless we know for sure.
		return nil, fmt.Errorf("failed to get secret for rotation: %w", err)
	}

	// 2. Get the appropriate rotator.
	rotator, err := w.factory.GetRotator(currentSecret)
	if err != nil {
		// Configuration error, non-retryable.
		w.handleRotationFailure(ctx, currentSecret, err)
		return nil, queue.NewNonRetryableError(fmt.Errorf("no rotator found for secret: %w", err))
	}

	// 3. Perform the rotation (Generate new value).
	newValue, err := rotator.Rotate(ctx, currentSecret)
	if err != nil {
		// Rotation logic failed (e.g. external API down). Retryable.
		w.handleRotationFailure(ctx, currentSecret, err)
		return nil, fmt.Errorf("rotator failed: %w", err)
	}

	// 4. Encrypt and save the new version.
	plaintext, _ := json.Marshal(newValue)
	ciphertext, encCtx, err := w.cryptoService.Encrypt(ctx, plaintext)
	if err != nil {
		w.handleRotationFailure(ctx, currentSecret, err)
		return nil, fmt.Errorf("rotator failed: %w", err)
	}

	oldVersion := currentSecret.Version
	currentSecret.EncryptedValue = ciphertext
	_ = currentSecret.SetEncryptionContext(encCtx)
	currentSecret.Version++
	currentSecret.Status = secret.StatusActive
	currentSecret.UpdatedAt = time.Now().UTC()

	if currentSecret.RotationPolicy != nil {
		currentSecret.RotationPolicy.ScheduleNextRotation()
	}

	if err := w.repo.Set(ctx, currentSecret); err != nil {
		w.handleRotationFailure(ctx, currentSecret, err)
		return nil, fmt.Errorf("failed to save rotated secret: %w", err)
	}

	// Invalidate cache
	if err := w.cache.Delete(ctx, fmt.Sprintf("secret:%s", p.SecretName)); err != nil {
		logger.Error("failed to invalidate secret cache", err)
	}

	// 5. Audit and publish success event.
	w.handleRotationSuccess(ctx, currentSecret, oldVersion)
	logger.Info("Secret rotation successful", log.Int("new_version", currentSecret.Version))

	result, _ := json.Marshal(map[string]interface{}{
		"new_version": currentSecret.Version,
		"status":      "rotated",
	})
	return result, nil
}

func (w *RotationWorker) handleRotationSuccess(ctx context.Context, sec *secret.Secret, oldVersion int) {
	// Audit Event
	auditEvent := audit.NewEvent(system.SystemActorID, system.SystemActorType, secret.EventSecretRotated).
		WithSuccess().
		WithTarget(sec.ID, "secret", sec.Name).
		WithMetadata(map[string]interface{}{
			"old_version": oldVersion,
			"new_version": sec.Version,
			"rotator":     "system",
		}).Build()

	go func() {
		if err := w.audit.Record(context.Background(), auditEvent); err != nil {
			w.logger.Error("failed to record rotation success audit", err)
		}
	}()

	// Domain Event (using refined EventBus)
	payload := secret.SecretRotatedPayload{
		Name:       sec.Name,
		OldVersion: oldVersion,
		NewVersion: sec.Version,
		RotatedBy:  "system",
		Timestamp:  time.Now().UTC(),
	}
	event, err := events.NewEvent(secret.EventSecretRotated, system.SystemActorID, system.SystemActorType, payload)
	if err != nil {
		w.logger.Error("failed to create rotation success event", err)
		return
	}

	go func() {
		if err := w.eventBus.Publish(context.Background(), event); err != nil {
			w.logger.Error("failed to publish rotation success event", err)
		}
	}()
}

func (w *RotationWorker) handleRotationFailure(ctx context.Context, sec *secret.Secret, err error) {
	logEntry := w.logger.With(log.String("secret_name", sec.Name), log.String("secret_id", sec.ID.String()))
	logEntry.Error("Secret rotation failed", err)

	// Update status to ROTATION_FAILED if possible
	sec.Status = secret.StatusRotationFailed
	sec.UpdatedAt = time.Now().UTC()
	if dbErr := w.repo.Set(ctx, sec); dbErr != nil {
		logEntry.Error("Failed to update secret status to ROTATION_FAILED", dbErr)
	}

	// Audit Event
	auditEvent := audit.NewEvent(system.SystemActorID, system.SystemActorType, secret.EventSecretRotationFailed).
		WithFailure(err.Error()).
		WithTarget(sec.ID, "secret", sec.Name).
		Build()

	go func() {
		if err := w.audit.Record(context.Background(), auditEvent); err != nil {
			w.logger.Error("failed to record audit failure event", err)
		}
	}()

	// Domain Event
	payload := secret.RotationFailedPayload{
		Name:             sec.Name,
		AttemptedVersion: sec.Version + 1,
		AttemptedBy:      "system",
		Reason:           err.Error(),
		Timestamp:        time.Now().UTC(),
	}

	event, eventErr := events.NewEvent(secret.EventSecretRotated, system.SystemActorID, system.SystemActorType, payload)
	if eventErr != nil {
		logEntry.Error("Failed to create domain event", eventErr)
		return
	}
	go func() {
		if err := w.eventBus.Publish(context.Background(), event); err != nil {
			w.logger.Error("failed to publish rotation failure event", err)
		}
	}()
}
