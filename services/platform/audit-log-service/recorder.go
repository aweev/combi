// audit-log-service — ported from api/internal/services/audit/recorder.go.
package auditlog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Statically ensure AsyncEventRecorder implements the domain interface.
var _ audit.EventRecorder = (*AsyncEventRecorder)(nil)

// AsyncEventRecorder is the application-layer implementation for recording audit events.
type AsyncEventRecorder struct {
	cryptoSvc security.CryptoService
	enqueuer  queue.JobEnqueuer
	logger    log.Logger
}

// NewAsyncEventRecorder creates a new recorder.
func NewAsyncEventRecorder(
	cryptoSvc security.CryptoService,
	enqueuer queue.JobEnqueuer,
	logger log.Logger,
) *AsyncEventRecorder {
	return &AsyncEventRecorder{
		cryptoSvc: cryptoSvc,
		enqueuer:  enqueuer,
		logger:    logger.With(log.String("component", "audit_recorder")),
	}
}

// Record implements the audit.EventRecorder interface. It is
// fire-and-forget: failures are logged (with the full encrypted payload as a
// last-resort fallback) but never returned, so the calling service is never
// failed by audit logging.
func (r *AsyncEventRecorder) Record(ctx context.Context, event audit.Event) error {
	// 1. Marshal the sensitive part of the event for encryption.
	plaintext, err := json.Marshal(event.Sensitive)
	if err != nil {
		// This is a programming error (bad data in builder). Log critically but don't crash.
		r.logger.Error("failed to marshal sensitive audit data for encryption", err,
			log.String("action", string(event.Action)),
			log.String("actor_id", event.ActorID.String()),
		)
		// We return nil to uphold the "fire-and-forget" contract. The calling service should not fail.
		return nil
	}

	// 2. Encrypt the sensitive data using the crypto service.
	ciphertext, encContext, err := r.cryptoSvc.Encrypt(ctx, plaintext)
	if err != nil {
		r.logger.Error("failed to encrypt sensitive audit data", err, log.String("action", string(event.Action)))
		return nil
	}

	eventForQueue := event
	eventForQueue.Sensitive = audit.SensitiveData{}
	eventForQueue.EncryptedData = ciphertext
	eventForQueue.EncryptionContext = encContext

	// 4. Marshal the entire prepared event to be the job payload.
	jobPayload, err := json.Marshal(eventForQueue)
	if err != nil {
		r.logger.Error("failed to marshal final audit event for queue", err,
			log.String("event_id", eventForQueue.ID.String()),
		)
		return nil
	}

	job := &queue.Job{
		ID:          uuid.New(),
		Type:        audit.JobAuditStoreEvent,
		Payload:     jobPayload,
		Queue:       "critical",
		Priority:    0,
		MaxAttempts: 5,
		Status:      queue.StatusPending,
		CreatedAt:   time.Now().UTC(),
		AvailableAt: time.Now().UTC(),
		// Actor info is on the event itself, not needed on the job.
	}

	if err := r.enqueuer.EnqueueJob(ctx, job); err != nil {
		r.logger.Error("CRITICAL: AUDIT EVENT DROPPED FROM QUEUE - FALLBACK LOGGING", err,
			log.String("event_id", event.ID.String()),
			log.String("fallback_payload", string(jobPayload)),
		)
	}

	return nil
}

// SetEnqueuer late-binds the job enqueuer (used when the recorder is
// constructed before the queue infrastructure in the host graph).
func (r *AsyncEventRecorder) SetEnqueuer(e queue.JobEnqueuer) {
	r.enqueuer = e
}
