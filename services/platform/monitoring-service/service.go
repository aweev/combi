package monitoring

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
)

var _ monitoring.AlertAdminService = (*adminService)(nil)

type adminService struct {
	repo          monitoring.AlertRuleRepository
	silenceRepo   monitoring.AlertSilenceRepository
	auditRecorder audit.EventRecorder
	logger        log.Logger
}

func NewAdminService(
	repo monitoring.AlertRuleRepository,
	silenceRepo monitoring.AlertSilenceRepository,
	audit audit.EventRecorder,
	logger log.Logger,
) monitoring.AlertAdminService {
	return &adminService{
		repo:          repo,
		silenceRepo:   silenceRepo,
		auditRecorder: audit,
		logger:        logger.With(log.String("service", "monitoring_admin")),
	}
}

// --- Rules ---

func (s *adminService) CreateAlertRule(ctx context.Context, rule *monitoring.AlertRule) (*monitoring.AlertRule, error) {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	rule.CreatedAt = time.Now().UTC()
	rule.UpdatedAt = rule.CreatedAt

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, err
	}

	s.audit(ctx, "monitoring.rule.created", rule.ID, rule.Name)
	return rule, nil
}

func (s *adminService) UpdateAlertRule(ctx context.Context, rule *monitoring.AlertRule) (*monitoring.AlertRule, error) {
	existing, err := s.repo.GetRule(ctx, rule.ID)
	if err != nil {
		return nil, err
	}

	// Merge updates
	existing.Name = rule.Name
	existing.Description = rule.Description
	existing.Severity = rule.Severity
	existing.PromQLQuery = rule.PromQLQuery
	existing.ForDuration = rule.ForDuration
	existing.Labels = rule.Labels
	existing.Annotations = rule.Annotations
	existing.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateRule(ctx, existing); err != nil {
		return nil, err
	}

	s.audit(ctx, "monitoring.rule.updated", existing.ID, existing.Name)
	return existing, nil
}

func (s *adminService) DeleteAlertRule(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteRule(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, "monitoring.rule.deleted", id, "")
	return nil
}

func (s *adminService) GetAlertRule(ctx context.Context, id uuid.UUID) (*monitoring.AlertRule, error) {
	return s.repo.GetRule(ctx, id)
}

func (s *adminService) ListAlertRules(ctx context.Context) ([]*monitoring.AlertRule, error) {
	return s.repo.ListRules(ctx)
}

// --- Silences ---

func (s *adminService) CreateSilence(ctx context.Context, silence *monitoring.AlertSilence) (*monitoring.AlertSilence, error) {
	if silence.ID == uuid.Nil {
		silence.ID = uuid.New()
	}
	silence.CreatedAt = time.Now().UTC()

	if err := s.silenceRepo.CreateSilence(ctx, silence); err != nil {
		return nil, err
	}

	s.audit(ctx, "monitoring.silence.created", silence.ID, silence.Comment)
	return silence, nil
}

func (s *adminService) DeleteSilence(ctx context.Context, id uuid.UUID) error {
	return s.silenceRepo.DeleteSilence(ctx, id)
}

func (s *adminService) GetSilence(ctx context.Context, id uuid.UUID) (*monitoring.AlertSilence, error) {
	return s.silenceRepo.GetSilence(ctx, id)
}

func (s *adminService) ListSilences(ctx context.Context, activeOnly bool) ([]*monitoring.AlertSilence, error) {
	if activeOnly {
		return s.silenceRepo.ListActiveSilences(ctx, time.Now().UTC())
	}
	return s.silenceRepo.ListAllSilences(ctx)
}

// Helper
func (s *adminService) audit(ctx context.Context, action string, targetID uuid.UUID, msg string) {
	// Assuming a simplified internal audit helper or passing correct params
	// This is a placeholder for the actual AuditService call
	// s.auditRecorder.Record(...)
}
