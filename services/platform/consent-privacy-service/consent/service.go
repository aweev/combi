package consent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/consent"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Statically ensure implementation
var _ consent.Service = (*Service)(nil)
var _ privacy.Anonymizer = (*Service)(nil)

type Service struct {
	repo     consent.Repository
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewService(repo consent.Repository, audit audit.EventRecorder, bus events.Bus, logger log.Logger) *Service {
	return &Service{
		repo:     repo,
		audit:    audit,
		eventBus: bus,
		logger:   logger.With(log.String("service", "consent")),
	}
}

// --- User Actions ---

func (s *Service) GrantConsent(ctx context.Context, userID uuid.UUID, revisionID uuid.UUID, ip, ua string) error {
	// 1. Validate Revision
	rev, err := s.repo.GetRevision(ctx, revisionID)
	if err != nil {
		return err
	}
	if !rev.IsActive {
		return consent.ErrRevisionInactive
	}

	// 2. Create Consent Record
	uc := &consent.UserConsent{
		ID:         uuid.New(),
		UserID:     userID,
		RevisionID: revisionID,
		Status:     consent.StatusActive,
		IPAddress:  ip,
		UserAgent:  ua,
		GrantedAt:  time.Now().UTC(),
	}

	if err := s.repo.CreateUserConsent(ctx, uc); err != nil {
		return err
	}

	payload := consent.ConsentGrantedPayload{
		UserID:      userID,
		AgreementID: rev.AgreementID,
		RevisionID:  revisionID,
		IPAddress:   ip,
	}

	// 3. Audit & Event
	s.auditAction(ctx, core.EventConsentGranted, userID, revisionID, "success")
	s.publishEvent(ctx, core.EventConsentGranted, userID, payload)

	return nil
}

func (s *Service) RevokeConsent(ctx context.Context, userID uuid.UUID, agreementID uuid.UUID) error {
	// 1. Check if Mandatory (Business Rule)
	// We allow revocation technically, but the frontend/middleware will block access immediately after.
	// Ideally, we warn the user.

	// 2. Revoke in DB
	if err := s.repo.RevokeUserConsent(ctx, userID, agreementID, time.Now().UTC()); err != nil {
		return err
	}

	// 3. Audit & Event
	s.auditAction(ctx, core.EventConsentRevoked, userID, agreementID, "revoked")
	s.publishEvent(ctx, core.EventConsentRevoked, userID, map[string]string{
		"agreement_id": agreementID.String(),
	})

	return nil
}

func (s *Service) CheckCompliance(ctx context.Context, userID uuid.UUID) (*consent.ComplianceStatus, error) {
	missing, err := s.repo.GetMissingMandatoryConsents(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Note: ActiveConsents could be fetched here if needed for the UI,
	// but GetMissing is the critical part for the middleware.
	return &consent.ComplianceStatus{
		UserID:          userID,
		IsCompliant:     len(missing) == 0,
		MissingConsents: missing,
	}, nil
}

// HasConsented is the high-performance check used by Middleware.
func (s *Service) HasConsented(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	return s.repo.HasActiveConsent(ctx, userID, slug)
}

// --- Admin Management ---

func (s *Service) CreateAgreement(ctx context.Context, a consent.Agreement) (*consent.Agreement, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now().UTC()

	if err := s.repo.CreateAgreement(ctx, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) PublishRevision(ctx context.Context, r consent.Revision) (*consent.Revision, error) {
	r.ID = uuid.New()
	r.PublishedAt = time.Now().UTC()
	r.IsActive = true // Default to active upon publish

	if err := s.repo.CreateRevision(ctx, &r); err != nil {
		return nil, err
	}

	payload := consent.RevisionPublishedPayload{
		AgreementID: r.AgreementID,
		RevisionID:  r.ID,
		Version:     r.Version,
		Summary:     r.Summary,
	}
	s.publishEvent(ctx, core.EventRevisionPublished, uuid.Nil, payload)

	return &r, nil
}

func (s *Service) GetLatestRevision(ctx context.Context, slug string) (*consent.Revision, error) {
	agreement, err := s.repo.GetAgreementBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.repo.GetLatestActiveRevision(ctx, agreement.ID)
}

// --- Helpers ---

func (s *Service) auditAction(ctx context.Context, action core.EventAction, userID, targetID uuid.UUID, outcome string) {
	evt := audit.NewEvent(userID, "user", action).
		WithTarget(targetID, "consent_entity", targetID.String()).
		WithMetadata(map[string]interface{}{"outcome": outcome}).
		WithContext(ctx).
		Build()
	// Fire and forget with error logging
	go func() {
		if err := s.audit.Record(context.Background(), evt); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}()
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, actorID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, actorID, "user", payload)
	if err != nil {
		s.logger.Error("failed to create domain event", err)
		return
	}
	// Fire and forget with error logging
	go func() {
		if err := s.eventBus.Publish(context.Background(), evt); err != nil {
			s.logger.Error("failed to publish domain event", err)
		}
	}()
}

func (s *Service) GetAgreement(ctx context.Context, slug string) (*consent.Agreement, error) {
	return s.repo.GetAgreementBySlug(ctx, slug)
}

// AnonymizeUser implements the Right to be Forgotten for Legal Records.
// We DO NOT delete the record (needed for legal defense).
// We SCRUB the IP Address and User Agent.
func (s *Service) AnonymizeUser(ctx context.Context, userID uuid.UUID) (*privacy.ErasureReport, error) {
	s.logger.Info("Anonymizing consent records", log.String("user_id", userID.String()))

	count, err := s.repo.AnonymizeUserConsents(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to anonymize consents: %w", err)
	}

	return &privacy.ErasureReport{
		Service:           "ConsentService",
		RecordsDeleted:    0,
		RecordsAnonymized: count,
		Details:           "Scrubbed IP and User-Agent from consent history.",
	}, nil
}
