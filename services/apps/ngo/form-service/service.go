package form

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/form"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ form.Service = (*Service)(nil)
var _ reporting.DataFetcher = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo          form.Repository
	auditRecorder audit.EventRecorder
	eventBus      events.Bus
	authz         rbac.AuthorizationService
	logger        log.Logger
}

func NewService(
	repo form.Repository,
	audit audit.EventRecorder,
	eventBus events.Bus,
	authz rbac.AuthorizationService,
	logger log.Logger,
) *Service {
	return &Service{
		repo:          repo,
		auditRecorder: audit,
		eventBus:      eventBus,
		authz:         authz,
		logger:        logger.With(log.String("service", "form_service")),
	}
}

func (s *Service) FetchData(ctx context.Context, params map[string]any) (any, error) {
	// 1. Extract Form ID
	formIDStr, ok := params["form_id"].(string)
	if !ok {
		return nil, fmt.Errorf("form_id required")
	}
	formID := uuid.MustParse(formIDStr)

	// 2. Fetch Submissions (Assumes Repo has ListSubmissions)
	// We want ALL submissions, so we bypass standard pagination limits
	submissions, _, err := s.repo.ListSubmissions(ctx, formID, 10000, 0)
	if err != nil {
		return nil, err
	}

	// 3. Flatten Data for CSV
	// Form answers are often JSONB. We need to flatten them for the report.
	type FlatRow struct {
		SubmissionID string `json:"submission_id"`
		SubmittedAt  string `json:"submitted_at"`
		// Dynamic fields will be handled by the Reporting Service's JSON renderer
		// or we explicitly map specific known keys here.
		Answers json.RawMessage `json:"answers"`
	}

	rows := make([]FlatRow, len(submissions))
	for i, sub := range submissions {
		// Assuming sub.Data is map[string]any
		rows[i] = FlatRow{
			SubmissionID: sub.ID.String(),
			SubmittedAt:  sub.SubmittedAt.Format(time.RFC3339),
			Answers:      sub.Data,
		}
	}

	return rows, nil
}

// --- Form Definition Management ---

func (s *Service) CreateForm(ctx context.Context, def form.FormDefinition) (*form.FormDefinition, error) {
	if err := s.checkPermission(ctx, form.PermFormManage); err != nil {
		return nil, err
	}

	actor := auth.ActorFromContext(ctx)

	if def.ID == uuid.Nil {
		def.ID = uuid.New()
	}
	if def.Slug == "" {
		def.Slug = slug.Make(def.Title)
	}
	def.CreatedBy = actor.UserID
	def.CreatedAt = time.Now().UTC()
	def.UpdatedAt = time.Now().UTC()

	// Validate Schema structure
	if err := validateSchema(def.Schema); err != nil {
		return nil, err
	}

	if err := s.repo.CreateForm(ctx, &def); err != nil {
		return nil, err
	}

	s.auditSuccess(ctx, core.EventFormCreated, def.ID, "form", def.Title)
	return &def, nil
}

func (s *Service) GetForm(ctx context.Context, id uuid.UUID) (*form.FormDefinition, error) {
	return s.repo.GetForm(ctx, id)
}

func (s *Service) GetFormBySlug(ctx context.Context, slug string) (*form.FormDefinition, error) {
	return s.repo.GetFormBySlug(ctx, slug)
}

func (s *Service) ListForms(ctx context.Context, fType *form.FormType, activeOnly bool) ([]form.FormDefinition, error) {
	// Public read access allowed for listing (filtering handled by caller/repo)
	return s.repo.ListForms(ctx, fType, activeOnly)
}

func (s *Service) UpdateForm(ctx context.Context, def form.FormDefinition) (*form.FormDefinition, error) {
	if err := s.checkPermission(ctx, form.PermFormManage); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetForm(ctx, def.ID)
	if err != nil {
		return nil, err
	}

	// Merge updates
	if def.Title != "" {
		existing.Title = def.Title
	}
	if def.Description != "" {
		existing.Description = def.Description
	}
	if def.Schema != nil {
		if err := validateSchema(def.Schema); err != nil {
			return nil, err
		}
		existing.Schema = def.Schema
	}
	if def.ExpiresAt != nil {
		existing.ExpiresAt = def.ExpiresAt
	}
	// Note: Type usually cannot be changed after creation safely

	if err := s.repo.UpdateForm(ctx, existing); err != nil {
		return nil, err
	}

	s.auditSuccess(ctx, core.EventFormUpdated, existing.ID, "form", existing.Title)
	return existing, nil
}

// --- Submission Handling ---

func (s *Service) Submit(ctx context.Context, formID uuid.UUID, userID *uuid.UUID, data map[string]any) (*form.Submission, error) {
	f, err := s.repo.GetForm(ctx, formID)
	if err != nil {
		return nil, err
	}

	// 1. Validate Access
	if !f.IsActive {
		return nil, form.ErrFormExpired
	}
	if f.ExpiresAt != nil && time.Now().After(*f.ExpiresAt) {
		return nil, form.ErrFormExpired
	}
	// If form is NOT anonymous, userID is required
	if !f.IsAnonymous && userID == nil {
		return nil, apperror.New(401, "AUTH_REQUIRED", "This form requires authentication", nil)
	}

	// 2. Validate Data against Schema
	if err := validateAnswers(f.Schema, data); err != nil {
		return nil, err
	}

	dataBytes, _ := json.Marshal(data)
	sub := &form.Submission{
		ID:          uuid.New(),
		FormID:      formID,
		UserID:      userID,
		Status:      form.StatusSubmitted,
		Data:        dataBytes,
		SubmittedAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateSubmission(ctx, sub); err != nil {
		return nil, err
	}

	// 3. Events
	s.publishEvent(ctx, core.EventSubmissionReceived, formID, map[string]interface{}{
		"submission_id": sub.ID,
		"form_type":     f.Type,
	})

	return sub, nil
}

func (s *Service) ListSubmissions(ctx context.Context, formID uuid.UUID, limit, offset int) ([]form.Submission, int, error) {
	if err := s.checkPermission(ctx, form.PermFormViewResults); err != nil {
		return nil, 0, err
	}
	return s.repo.ListSubmissions(ctx, formID, limit, offset)
}

func (s *Service) GetSubmission(ctx context.Context, id uuid.UUID) (*form.Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, id)
	if err != nil {
		return nil, err
	}

	// Access Control: Owner OR Admin
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	isOwner := sub.UserID != nil && *sub.UserID == actor.UserID
	if !isOwner {
		if err := s.checkPermission(ctx, form.PermFormViewResults); err != nil {
			return nil, err
		}
	}

	return sub, nil
}

func (s *Service) UpdateSubmissionStatus(ctx context.Context, id uuid.UUID, status form.SubmissionStatus) error {
	// Typically Admin only
	if err := s.checkPermission(ctx, form.PermFormManage); err != nil {
		return err
	}

	sub, err := s.repo.GetSubmission(ctx, id)
	if err != nil {
		return err
	}

	sub.Status = status
	if err := s.repo.UpdateSubmission(ctx, sub); err != nil {
		return err
	}

	s.publishEvent(ctx, core.EventSubmissionStatusChanged, sub.FormID, map[string]interface{}{
		"submission_id": sub.ID,
		"status":        status,
	})
	return nil
}

// --- Voting ---

func (s *Service) CastVote(ctx context.Context, formID uuid.UUID, userID uuid.UUID, data map[string]any) error {
	f, err := s.repo.GetForm(ctx, formID)
	if err != nil {
		return err
	}

	if f.Type != form.TypeVote {
		return form.ErrInvalidInput
	}
	if !f.IsActive || (f.ExpiresAt != nil && time.Now().After(*f.ExpiresAt)) {
		return form.ErrVotingClosed
	}

	// 1. Validate Vote Data
	if err := validateAnswers(f.Schema, data); err != nil {
		return err
	}

	// 2. Execute Vote Transaction
	err = s.repo.WithTx(ctx, func(txRepo form.Repository) error {
		// Check double voting
		hasVoted, err := txRepo.CheckVoteParticipation(ctx, formID, userID)
		if err != nil {
			return err
		}
		if hasVoted {
			return form.ErrAlreadyVoted
		}

		// Record Participation
		if err := txRepo.RecordVoteParticipation(ctx, formID, userID); err != nil {
			return err
		}

		// Record Submission (Anonymously if configured)
		var submitterID *uuid.UUID
		if !f.IsAnonymous {
			submitterID = &userID
		}

		dataBytes, _ := json.Marshal(data)
		sub := &form.Submission{
			ID:          uuid.New(),
			FormID:      formID,
			UserID:      submitterID,
			Status:      form.StatusSubmitted,
			Data:        dataBytes,
			SubmittedAt: time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		return txRepo.CreateSubmission(ctx, sub)
	})

	if err != nil {
		return err
	}

	s.auditSuccess(ctx, core.EventVoteCast, formID, "form", "User cast vote")
	return nil
}

func (s *Service) HasVoted(ctx context.Context, formID, userID uuid.UUID) (bool, error) {
	return s.repo.CheckVoteParticipation(ctx, formID, userID)
}

func (s *Service) GetVoteResults(ctx context.Context, formID uuid.UUID) (*form.VoteResultSummary, error) {
	// Check Perms (Results might be public or private)
	// For now, assume restricted
	if err := s.checkPermission(ctx, form.PermFormViewResults); err != nil {
		return nil, err
	}

	f, err := s.repo.GetForm(ctx, formID)
	if err != nil {
		return nil, err
	}

	results, err := s.repo.GetAggregateResults(ctx, formID)
	if err != nil {
		return nil, err
	}

	totalVotes := 0
	for _, count := range results {
		totalVotes += count
	}

	// Calculate Outcomes if VotingConfig exists
	quorumMet := false
	passed := false

	// Very basic logic for a "Yes/No" type vote where "Yes" is the key
	// In a real system, we'd need to know which option is "Affirmative"
	if f.VotingConfig != nil && totalVotes > 0 {
		// Placeholder: assume totalVotes vs some total eligible population (not tracked here)
		// or simply pass if Yes > 50%
		yesVotes := results["Yes"]
		passThreshold := float64(totalVotes) * (float64(f.VotingConfig.PassPct) / 100.0)
		if float64(yesVotes) >= passThreshold {
			passed = true
		}
		quorumMet = true // Assuming if we have results, quorum met for now
	}

	return &form.VoteResultSummary{
		TotalVotes: totalVotes,
		Results:    results,
		QuorumMet:  quorumMet,
		Passed:     passed,
	}, nil
}

// --- Helpers ---

func (s *Service) checkPermission(ctx context.Context, perm core.PermissionName) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	allowed, err := s.authz.Authorize(ctx, rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: perm,
	})
	if err != nil {
		return err
	}
	if !allowed {
		return form.ErrAccessDenied
	}
	return nil
}

func (s *Service) auditSuccess(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType, msg string) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return
	}
	evt := audit.NewEvent(actor.UserID, "user", action).
		WithTarget(targetID, targetType, msg).
		WithSuccess().
		WithContext(ctx).
		Build()
	if err := s.auditRecorder.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, targetID uuid.UUID, payload interface{}) {
	actor := auth.ActorFromContext(ctx)
	actorID := uuid.Nil
	if actor != nil {
		actorID = actor.UserID
	}

	pBytes, _ := json.Marshal(payload)
	targetType := "form"

	evt, _ := events.NewEvent(action, actorID, "user", pBytes)
	evt.TargetID = &targetID
	evt.TargetType = &targetType

	_ = s.eventBus.Publish(ctx, evt)
}

// --- Schema Validation Logic ---

type fieldDef struct {
	ID       string         `json:"id"`
	Type     form.FieldType `json:"type"`
	Required bool           `json:"required"`
	Options  []string       `json:"options"`
}

func validateSchema(schemaJSON json.RawMessage) error {
	var fields []fieldDef
	if err := json.Unmarshal(schemaJSON, &fields); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("schema cannot be empty")
	}
	return nil
}

func validateAnswers(schemaJSON json.RawMessage, data map[string]any) error {
	var fields []fieldDef
	if err := json.Unmarshal(schemaJSON, &fields); err != nil {
		return err
	}

	for _, field := range fields {
		val, exists := data[field.ID]

		// 1. Check Required
		if field.Required {
			if !exists || val == nil || val == "" {
				return fmt.Errorf("%w: missing required field '%s'", form.ErrInvalidSubmission, field.ID)
			}
		}

		if !exists {
			continue
		}

		// 2. Check Options (Select/Radio)
		if field.Type == form.FieldSelect || field.Type == form.FieldRadio {
			strVal, ok := val.(string)
			if !ok {
				return fmt.Errorf("field '%s' must be a string", field.ID)
			}
			validOption := false
			for _, opt := range field.Options {
				if opt == strVal {
					validOption = true
					break
				}
			}
			if !validOption {
				return fmt.Errorf("%w: invalid option '%s' for field '%s'", form.ErrInvalidSubmission, strVal, field.ID)
			}
		}
	}
	return nil
}

// ApplySyncChange handles incoming offline operations for Forms.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	switch op.EntityType {
	case "form_submission":
		if op.Action != sync_domain.ActionCreate {
			return uuid.Nil, 0, fmt.Errorf("offline updates not supported for submissions (append-only)")
		}

		// 1. Unmarshal Payload
		// The payload from the client matches the SubmitRequest DTO or similar
		var req struct {
			FormID  uuid.UUID      `json:"form_id"`
			Answers map[string]any `json:"answers"`
		}
		if err := json.Unmarshal(op.Payload, &req); err != nil {
			return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
		}

		// 2. Call Logic
		// We pass 'actorID' as the user submitting it.
		// Note: Submit() expects *uuid.UUID for user ID.
		submission, err := s.Submit(ctx, req.FormID, &actorID, req.Answers)
		if err != nil {
			return uuid.Nil, 0, err
		}

		// 3. Return Result
		// Version 1 for new objects
		return submission.ID, 1, nil

	default:
		return uuid.Nil, 0, fmt.Errorf("unknown entity type: %s", op.EntityType)
	}
}

// FetchDelta returns changes for the client to download.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	// For Forms, clients usually need definitions (to render them)
	// and maybe their own previous submissions.

	// 1. Fetch Active Forms
	// This ensures the offline client has the latest questions/schema
	forms, err := s.ListForms(ctx, nil, true) // nil type = all, true = active only
	if err != nil {
		return nil, err
	}

	results := make([]sync_domain.EntityState, 0, len(forms))

	for _, f := range forms {
		// Filter by 'updated_at' > since
		if f.UpdatedAt.After(since) {
			data, _ := json.Marshal(f)
			results = append(results, sync_domain.EntityState{
				ID:        f.ID,
				Version:   int(f.UpdatedAt.Unix()), // Simple versioning based on time
				Deleted:   !f.IsActive,
				Data:      data,
				UpdatedAt: f.UpdatedAt,
			})
		}
	}

	return results, nil
}
