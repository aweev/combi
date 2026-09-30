// audit-log-service — ported from api/internal/services/audit/compliance_service.go.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

var _ audit.ComplianceService = (*ComplianceService)(nil)

// ComplianceService manages legal holds, data retention policies, and user
// anonymization (GDPR right-to-be-forgotten) requests.
type ComplianceService struct {
	legalHoldRepo  audit.LegalHoldRepository
	retentionRepo  audit.RetentionPolicyRepository
	dispatcher     queue.Dispatcher // nil until a queue host is mounted
	audit          audit.EventRecorder
	privacyManager privacy.Manager // nil until the privacy module is mounted
	logger         log.Logger
}

// NewComplianceService creates a new compliance service.
func NewComplianceService(
	legalHoldRepo audit.LegalHoldRepository,
	retentionRepo audit.RetentionPolicyRepository,
	dispatcher queue.Dispatcher,
	auditRecorder audit.EventRecorder,
	privacyManager privacy.Manager,
	logger log.Logger,
) *ComplianceService {
	return &ComplianceService{
		legalHoldRepo:  legalHoldRepo,
		retentionRepo:  retentionRepo,
		dispatcher:     dispatcher,
		audit:          auditRecorder,
		privacyManager: privacyManager,
		logger:         logger.With(log.String("component", "compliance_service")),
	}
}

// ApplyLegalHold freezes data deletion for a subject (e.g. pending litigation).
func (s *ComplianceService) ApplyLegalHold(ctx context.Context, params audit.ApplyLegalHoldParams) (*audit.LegalHold, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	hold := &audit.LegalHold{
		ID:          uuid.New(),
		Name:        params.Name,
		Description: params.Description,
		HoldSubject: params.HoldSubject,
		CreatedBy:   actor.AccountID,
		IssuedAt:    time.Now().UTC(),
	}

	if err := s.legalHoldRepo.CreateLegalHold(ctx, hold); err != nil {
		s.logger.Error("failed to create legal hold in repository", err)
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventLegalHoldApplied).
		WithSuccess().
		WithTarget(hold.ID, "legal_hold", hold.Name).
		WithContext(ctx).Build()
	_ = s.audit.Record(context.Background(), auditEvent)

	return hold, nil
}

// ReleaseLegalHold re-enables data deletion for a held subject. Idempotent:
// releasing an already-released hold is a no-op returning the stored hold.
func (s *ComplianceService) ReleaseLegalHold(ctx context.Context, holdID uuid.UUID) (*audit.LegalHold, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	hold, err := s.legalHoldRepo.GetLegalHold(ctx, holdID)
	if err != nil {
		return nil, err
	}

	if hold.ReleasedAt != nil {
		return hold, nil
	}

	now := time.Now().UTC()
	hold.ReleasedAt = &now
	hold.ReleasedBy = &actor.AccountID

	if err := s.legalHoldRepo.UpdateLegalHold(ctx, hold); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventLegalHoldReleased).
		WithSuccess().
		WithTarget(hold.ID, "legal_hold", hold.Name).
		WithContext(ctx).Build()
	_ = s.audit.Record(context.Background(), auditEvent)

	return hold, nil
}

// ListLegalHolds returns a page of legal holds.
func (s *ComplianceService) ListLegalHolds(ctx context.Context, limit, offset int) ([]*audit.LegalHold, int, error) {
	return s.legalHoldRepo.ListLegalHolds(ctx, limit, offset)
}

// DefineRetentionPolicy creates a new retention policy for a data type.
func (s *ComplianceService) DefineRetentionPolicy(ctx context.Context, params audit.DefineRetentionPolicyParams) (*audit.RetentionPolicy, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	now := time.Now().UTC()
	policy := &audit.RetentionPolicy{
		ID:          uuid.New(),
		Name:        params.Name,
		Description: params.Description,
		DataType:    params.DataType,
		Duration:    params.Duration,
		IsActive:    params.IsActive,
		CreatedBy:   actor.AccountID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.retentionRepo.CreateRetentionPolicy(ctx, policy); err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventRetentionPolicyCreated).
		WithSuccess().
		WithTarget(policy.ID, "retention_policy", policy.Name).
		WithContext(ctx).Build()
	_ = s.audit.Record(context.Background(), auditEvent)
	s.logger.Info("defined retention policy", log.String("policy_id", policy.ID.String()))
	return policy, nil
}

// GetRetentionPolicy fetches a single retention policy.
func (s *ComplianceService) GetRetentionPolicy(ctx context.Context, policyID uuid.UUID) (*audit.RetentionPolicy, error) {
	return s.retentionRepo.GetRetentionPolicy(ctx, policyID)
}

// ListRetentionPolicies returns a page of retention policies.
func (s *ComplianceService) ListRetentionPolicies(ctx context.Context, limit, offset int) ([]*audit.RetentionPolicy, int, error) {
	return s.retentionRepo.ListRetentionPolicies(ctx, limit, offset)
}

// AnonymizeUser triggers a background job to perform a "Right to be
// Forgotten" request. A duplicate request (privacy.ErrRequestPending)
// returns a nil job without error.
func (s *ComplianceService) AnonymizeUser(ctx context.Context, userID uuid.UUID) (*queue.Job, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	if s.privacyManager == nil {
		// The privacy module is not mounted in this host yet.
		return nil, ErrPrivacyManagerNotConfigured
	}

	req, err := s.privacyManager.InitiateErasure(ctx, userID, "Admin requested via Compliance API")
	if err != nil {
		if errors.Is(err, privacy.ErrRequestPending) {
			// Handle duplicate gracefully
			return nil, nil
		}
		return nil, err
	}

	if s.dispatcher == nil {
		// The queue module is not mounted in this host yet.
		return nil, ErrDispatcherNotConfigured
	}

	jobPayload, err := json.Marshal(map[string]string{"request_id": req.ID.String()})
	if err != nil {
		s.logger.Error("failed to marshal anonymization job payload", err, log.String("user_id", userID.String()))
		return nil, fmt.Errorf("internal server error creating job payload: %w", err)
	}

	job, err := s.dispatcher.Dispatch(
		ctx,
		audit.JobComplianceAnonymizeUser,
		jobPayload,
		queue.WithQueue("compliance"),
		queue.WithPriority(0),
		queue.WithTimeout(1*time.Hour),
		queue.WithUniqueKey(fmt.Sprintf("anonymize-user:%s", userID), 24*time.Hour),
	)
	if err != nil {
		s.logger.Error("failed to dispatch user anonymization job", err, log.String("user_id", userID.String()))
		return nil, err
	}

	var auditMeta map[string]interface{}

	if job == nil {
		s.logger.Info("duplicate user anonymization request ignored", log.String("user_id", userID.String()))
		auditMeta = map[string]interface{}{"note": "Duplicate request ignored"}
	} else {
		auditMeta = map[string]interface{}{"job_id": job.ID.String()}
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventAnonymizationRequested).
		WithSuccess().
		WithTarget(userID, "user", "").
		WithMetadata(auditMeta).
		WithContext(ctx).Build()
	s.recordAuditEvent(auditEvent)

	if job != nil {
		s.logger.Info("dispatched user anonymization job",
			log.String("user_id", userID.String()),
			log.String("job_id", job.ID.String()),
		)
	}

	return job, nil
}

func (s *ComplianceService) recordAuditEvent(event audit.Event) {
	if err := s.audit.Record(context.Background(), event); err != nil {
		s.logger.Error("CRITICAL: failed to record audit event", err,
			log.String("event_action", string(event.Action)),
			log.String("actor_id", event.ActorID.String()),
		)
	}
}

// SetDispatcher late-binds the queue dispatcher.
func (s *ComplianceService) SetDispatcher(d queue.Dispatcher) {
	s.dispatcher = d
}

// SetPrivacyManager late-binds the privacy manager (erasure orchestration).
// The host calls this after the consent-privacy module is mounted; until
// then erasure-gated paths return an honest unavailable error (nil-guarded
// at the call site). Constructor-wiring it would cycle the fx graph with
// the consent-privacy module, which consumes audit.EventRecorder.
func (s *ComplianceService) SetPrivacyManager(pm privacy.Manager) {
	s.privacyManager = pm
}

// UpdateRetentionPolicy overwrites a retention policy's definition.
func (s *ComplianceService) UpdateRetentionPolicy(ctx context.Context, id uuid.UUID, params audit.DefineRetentionPolicyParams) (*audit.RetentionPolicy, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 1. Fetch existing policy
	policy, err := s.retentionRepo.GetRetentionPolicy(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Update fields
	policy.Name = params.Name
	policy.Description = params.Description
	policy.DataType = params.DataType
	policy.Duration = params.Duration
	policy.IsActive = params.IsActive
	policy.UpdatedAt = time.Now().UTC()

	// 3. Persist
	if err := s.retentionRepo.UpdateRetentionPolicy(ctx, policy); err != nil {
		return nil, err
	}

	// 4. Audit
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventRetentionPolicyUpdated).
		WithSuccess().
		WithTarget(policy.ID, "retention_policy", policy.Name).
		WithContext(ctx).Build()
	_ = s.audit.Record(context.Background(), auditEvent)

	s.logger.Info("updated retention policy", log.String("policy_id", id.String()))
	return policy, nil
}

// DeleteRetentionPolicy removes a retention policy.
func (s *ComplianceService) DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	// 1. Check existence (optional, but good for audit context)
	policy, err := s.retentionRepo.GetRetentionPolicy(ctx, id)
	if err != nil {
		return err
	}

	// 2. Delete
	if err := s.retentionRepo.DeleteRetentionPolicy(ctx, id); err != nil {
		return err
	}

	// 3. Audit
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventRetentionPolicyDeleted).
		WithSuccess().
		WithTarget(id, "retention_policy", policy.Name).
		WithContext(ctx).Build()
	_ = s.audit.Record(context.Background(), auditEvent)

	s.logger.Info("deleted retention policy", log.String("policy_id", id.String()))
	return nil
}
