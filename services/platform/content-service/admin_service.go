package content

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/log"
)

var _ content.AdminService = (*AdminService)(nil)

type AdminService struct {
	repo   content.Repository
	audit  audit.EventRecorder
	logger log.Logger
}

func NewAdminService(repo content.Repository, audit audit.EventRecorder, logger log.Logger) *AdminService {
	return &AdminService{
		repo:   repo,
		audit:  audit,
		logger: logger.With(log.String("service", "content_admin")),
	}
}

// --- Space Management ---

func (s *AdminService) CreateSpace(ctx context.Context, params content.CreateSpaceParams) (*content.Space, error) {
	// Validation is done via DTO tags in handler, but business logic goes here.
	// e.g. check unique key handled by repo constraint.

	space := &content.Space{
		ID:            uuid.New(),
		Key:           params.Key,
		Name:          params.Name,
		IsDefault:     false, // Default handling logic can be added
		Locales:       params.Locales,
		DefaultLocale: params.DefaultLocale,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := s.repo.CreateSpace(ctx, space); err != nil {
		return nil, err
	}

	s.auditAction(ctx, core.EventSpaceCreated, space.ID, "space", space.Name)
	return space, nil
}

func (s *AdminService) UpdateSpace(ctx context.Context, id uuid.UUID, params content.UpdateSpaceParams) (*content.Space, error) {
	space, err := s.repo.GetSpace(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		space.Name = *params.Name
	}
	if params.DefaultLocale != nil {
		space.DefaultLocale = *params.DefaultLocale
	}
	if params.Locales != nil {
		space.Locales = params.Locales
	}

	if err := s.repo.UpdateSpace(ctx, space); err != nil {
		return nil, err
	}
	s.auditAction(ctx, core.EventSpaceUpdated, space.ID, "space", space.Name)
	return space, nil
}

func (s *AdminService) DeleteSpace(ctx context.Context, id uuid.UUID) error {
	// Check if default
	space, err := s.repo.GetSpace(ctx, id)
	if err != nil {
		return err
	}
	if space.IsDefault {
		return content.ErrDefaultSpaceCannotBeDeleted
	}
	s.auditAction(ctx, core.EventSpaceDeleted, id, "space", "Space deleted")
	return nil
}

func (s *AdminService) GetSpace(ctx context.Context, id uuid.UUID) (*content.Space, error) {
	return s.repo.GetSpace(ctx, id)
}

func (s *AdminService) ListSpaces(ctx context.Context) ([]*content.Space, error) {
	return s.repo.ListSpaces(ctx)
}

// --- Model Management ---

func (s *AdminService) CreateModel(ctx context.Context, params content.CreateModelParams) (*content.Model, error) {
	model := &content.Model{
		ID:        uuid.New(),
		SpaceID:   params.SpaceID,
		APIKey:    params.APIKey,
		Name:      params.Name,
		Fields:    params.Fields,
		IsSystem:  false,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateModel(ctx, model); err != nil {
		return nil, err
	}
	s.auditAction(ctx, core.EventModelCreated, model.ID, "model", model.Name)
	return model, nil
}

func (s *AdminService) UpdateModel(ctx context.Context, id uuid.UUID, params content.UpdateModelParams) (*content.Model, error) {
	model, err := s.repo.GetModel(ctx, id)
	if err != nil {
		return nil, err
	}

	if model.IsSystem {
		return nil, content.ErrSystemModelImmutable
	}

	// Check if model is in use before allowing breaking changes (simplified: prevent any field change if in use)
	inUse, _ := s.repo.IsModelInUse(ctx, id)
	if inUse && params.Fields != nil {
		// For robust system: allow additive changes, block destructive.
		// Here: strict block on field changes if entries exist.
		return nil, content.ErrModelInUse
	}

	if params.Name != nil {
		model.Name = *params.Name
	}
	if params.APIKey != nil {
		model.APIKey = *params.APIKey
	}
	if params.Fields != nil {
		model.Fields = params.Fields
	}

	if err := s.repo.UpdateModel(ctx, model); err != nil {
		return nil, err
	}
	s.auditAction(ctx, core.EventModelUpdated, model.ID, "model", model.Name)
	return model, nil
}

func (s *AdminService) DeleteModel(ctx context.Context, id uuid.UUID) error {
	model, err := s.repo.GetModel(ctx, id)
	if err != nil {
		return err
	}
	if model.IsSystem {
		return content.ErrSystemModelImmutable
	}
	s.auditAction(ctx, core.EventModelDeleted, id, "model", "Model deleted")
	return nil
}

func (s *AdminService) GetModel(ctx context.Context, id uuid.UUID) (*content.Model, error) {
	return s.repo.GetModel(ctx, id)
}

func (s *AdminService) ListModels(ctx context.Context, spaceID uuid.UUID) ([]*content.Model, error) {
	return s.repo.ListModels(ctx, spaceID)
}

func (s *AdminService) UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, params content.UpdateEntryStatusParams) (*content.Entry, error) {
	// Admin override of status logic
	entry, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	entry.Status = params.Status
	entry.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return nil, err
	}
	s.auditAction(ctx, core.EventContentUpdated, entryID, "entry", "Status Admin Override")
	return entry, nil
}

// --- Helpers ---

func (s *AdminService) auditAction(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType, msg string) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return
	}
	evt := audit.NewEvent(actor.UserID, "user", action).
		WithTarget(targetID, targetType, msg).
		WithSuccess().
		WithContext(ctx).
		Build()
	if err := s.audit.Record(ctx, evt); err != nil {
		s.logger.Warn("failed to record admin audit event", log.ErrorAttr(err))
	}
}

func (s *AdminService) GetModelByAPIKey(ctx context.Context, apiKey string) (*content.Model, error) {
	return s.repo.GetModelByAPIKey(ctx, apiKey)
}
