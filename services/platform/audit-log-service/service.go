// audit-log-service (package auditlog) — ported from
// api/internal/services/audit/service.go.
//
// Service is the audit trail application service: querying and decrypting
// events, feeding the reporting service (reporting.DataFetcher), and
// dispatching export jobs. The dispatcher may be nil until a queue host is
// mounted in the fx graph — ExportEvents then returns
// ErrDispatcherNotConfigured instead of panicking (see errors.go).
package auditlog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ audit.Service = (*Service)(nil)
var _ reporting.DataFetcher = (*Service)(nil)

// Service is the audit trail application service.
type Service struct {
	repo       audit.EventRepository
	cryptoSvc  security.CryptoService
	dispatcher queue.Dispatcher // nil until a queue host is mounted
	logger     log.Logger
}

// NewService creates a new audit trail service.
func NewService(
	repo audit.EventRepository,
	cryptoSvc security.CryptoService,
	dispatcher queue.Dispatcher,
	logger log.Logger,
) *Service {
	return &Service{
		repo:       repo,
		cryptoSvc:  cryptoSvc,
		dispatcher: dispatcher,
		logger:     logger.With(log.String("component", "audit_service")),
	}
}

// FetchData implements reporting.DataFetcher: it maps the untyped report
// parameters back to typed EventFilters and returns the matching events for
// the reporting service to render.
func (s *Service) FetchData(ctx context.Context, params map[string]any) (any, error) {
	// Map untyped map[string]any back to typed EventFilters
	filters := audit.EventFilters{
		Limit: 10000, // Hard limit for safety
	}

	if actor, ok := params["actor_id"].(string); ok {
		uid := uuid.MustParse(actor)
		filters.ActorID = &uid
	}

	if target, ok := params["target_id"].(string); ok {
		uid := uuid.MustParse(target)
		filters.TargetID = &uid
	}

	events, _, err := s.repo.SearchEvents(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Return decrypted events (SearchEvents handles decryption)
	return events, nil
}

// GetEvent fetches a single event and decrypts its sensitive data.
func (s *Service) GetEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error) {
	event, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.decryptEvent(ctx, event); err != nil {
		return nil, audit.ErrDecryptionFailed.WithUnderlying(err)
	}
	return event, nil
}

// SearchEvents fetches events and decrypts them.
func (s *Service) SearchEvents(ctx context.Context, filters audit.EventFilters) ([]*audit.Event, int, error) {
	events, total, err := s.repo.SearchEvents(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("search events failed: %w", err)
	}

	for _, event := range events {
		if err := s.decryptEvent(ctx, event); err != nil {
			s.logger.Error("failed to decrypt audit event in search result", err,
				log.String("event_id", event.ID.String()),
			)
			// continue with empty sensitive data
		}
	}
	return events, total, nil
}

// ExportEvents triggers a background job to generate an export file.
func (s *Service) ExportEvents(ctx context.Context, filters audit.EventFilters, format string) (*queue.Job, error) {
	// Get the actor from context to know who requested the export.
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	if format != "csv" && format != "jsonl" { // json -> jsonl to match worker
		return nil, audit.ErrUnsupportedExportFormat
	}

	if s.dispatcher == nil {
		// The queue service module is not mounted in this host yet.
		return nil, ErrDispatcherNotConfigured
	}

	// Include the requester's ID in the payload.
	payloadMap := map[string]interface{}{
		"filters":      filters,
		"format":       format,
		"requester_id": actor.AccountID,
	}
	jobPayload, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export job payload: %w", err)
	}

	// Use a dedicated queue for long-running reports
	job, err := s.dispatcher.Dispatch(ctx, audit.JobAuditExport, jobPayload, queue.WithQueue("reports"))
	if err != nil {
		s.logger.Error("failed to dispatch audit export job", err)
		return nil, err
	}
	s.logger.Info("dispatched audit export job", log.String("job_id", job.ID.String()))
	return job, nil
}

// decryptEvent is the helper to handle decryption.
func (s *Service) decryptEvent(ctx context.Context, event *audit.Event) error {
	if event.EncryptedData == nil || event.EncryptionContext == nil {
		return nil
	}

	plaintext, err := s.cryptoSvc.Decrypt(ctx, event.EncryptedData, event.EncryptionContext)
	if err != nil {
		return audit.ErrDecryptionFailed
	}
	var sensitiveData audit.SensitiveData
	if err := json.Unmarshal(plaintext, &sensitiveData); err != nil {
		return fmt.Errorf("failed to unmarshal decrypted data: %w", err)
	}
	event.Sensitive = sensitiveData
	return nil
}

// SetDispatcher late-binds the queue dispatcher (used when the audit service
// is constructed before the queue infrastructure in the host graph).
func (s *Service) SetDispatcher(d queue.Dispatcher) {
	s.dispatcher = d
}
