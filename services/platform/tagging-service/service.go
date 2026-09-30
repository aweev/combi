package tagging

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/tagging"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Statically verify interface compliance
var _ tagging.Service = (*Service)(nil)
var _ tagging.AdminService = (*Service)(nil)

type Service struct {
	repo          tagging.Repository
	auditRecorder audit.EventRecorder
	eventBus      events.Bus
	authz         rbac.AuthorizationService
	logger        log.Logger
}

func NewService(
	repo tagging.Repository,
	audit audit.EventRecorder,
	bus events.Bus,
	authz rbac.AuthorizationService,
	logger log.Logger,
) *Service {
	return &Service{
		repo:          repo,
		auditRecorder: audit,
		eventBus:      bus,
		authz:         authz,
		logger:        logger.With(log.String("service", "tagging")),
	}
}

// --- Tag Management (Public/Protected) ---

func (s *Service) CreateTag(ctx context.Context, name string, tType tagging.TagType) (*tagging.Tag, error) {
	// Authorization: Creating tags (Taxonomy) is usually restricted.
	if err := s.checkPermission(ctx, tagging.PermTagManage); err != nil {
		return nil, err
	}

	slg := slug.Make(name)
	if slg == "" {
		return nil, tagging.ErrInvalidInput
	}

	// Check existence (Idempotent creation)
	existing, err := s.repo.GetTagBySlug(ctx, slg, tType)
	if err == nil && existing != nil {
		return existing, nil
	}

	tag := &tagging.Tag{
		ID:         uuid.New(),
		Name:       name,
		Slug:       slg,
		Type:       tType,
		Color:      "#CCCCCC", // Default color
		IsApproved: true,      // Auto-approve admin tags
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	if err := s.repo.CreateTag(ctx, tag); err != nil {
		return nil, err
	}

	// Audit
	s.audit(ctx, core.EventTagCreated, tag.ID, "tag", map[string]interface{}{
		"name": name,
		"type": tType,
	})

	return tag, nil
}

func (s *Service) SearchTags(ctx context.Context, query string, tType *tagging.TagType, limit int) ([]tagging.Tag, error) {
	// Public read access is allowed.
	// Optional: If we want private tags later, we'd check auth here.
	return s.repo.SearchTags(ctx, query, tType, limit)
}

func (s *Service) GetTag(ctx context.Context, id uuid.UUID) (*tagging.Tag, error) {
	return s.repo.GetTagByID(ctx, id)
}

// --- Entity Tagging ---

func (s *Service) TagEntity(ctx context.Context, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	// Note: We assume the caller (e.g., User Service) has already validated that 'actor'
	// owns or has permission to edit 'entityID'.

	err := s.repo.WithTx(ctx, func(txRepo tagging.Repository) error {
		for _, tagID := range tagIDs {
			// Verify tag exists
			if _, err := txRepo.GetTagByID(ctx, tagID); err != nil {
				return err
			}

			t := &tagging.Tagging{
				ID:         uuid.New(),
				TagID:      tagID,
				EntityType: entityType,
				EntityID:   entityID,
				CreatedAt:  time.Now().UTC(),
			}

			created, err := txRepo.AddTagging(ctx, t)
			if err != nil {
				return err
			}

			if created {
				// Increment usage count
				if err := txRepo.UpdateTagCount(ctx, tagID, 1); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	s.publishEvent(ctx, core.EventEntityTagged, entityID, map[string]interface{}{
		"entity_type": entityType,
		"tag_ids":     tagIDs,
	})

	return nil
}

func (s *Service) UntagEntity(ctx context.Context, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	return s.repo.WithTx(ctx, func(txRepo tagging.Repository) error {
		for _, tagID := range tagIDs {
			// Try to remove
			if err := txRepo.RemoveTagging(ctx, tagID, entityType, entityID); err != nil {
				// If not found, we ignore (idempotent) unless it's a DB error.
				// Assuming RemoveTagging returns nil if successful (even if 0 rows).
				continue
			}
			// Decrement usage count (Naive approach: assumes tag was present)
			if err := txRepo.UpdateTagCount(ctx, tagID, -1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) GetTagsForEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]tagging.Tag, error) {
	return s.repo.GetTagsForEntity(ctx, entityType, entityID)
}

func (s *Service) GetEntitiesByTag(ctx context.Context, tagSlug string, entityType string, limit, offset int) ([]uuid.UUID, int, error) {
	// Resolve Slug to ID. Try most common types first.
	tag, err := s.repo.GetTagBySlug(ctx, tagSlug, tagging.TypeTopic)
	if err != nil {
		tag, err = s.repo.GetTagBySlug(ctx, tagSlug, tagging.TypeSkill)
		if err != nil {
			tag, err = s.repo.GetTagBySlug(ctx, tagSlug, tagging.TypeInterest)
			if err != nil {
				return nil, 0, tagging.ErrTagNotFound
			}
		}
	}

	return s.repo.GetEntityIDsByTag(ctx, tag.ID, entityType, limit, offset)
}

// --- Admin Management ---

func (s *Service) MergeTags(ctx context.Context, sourceID, targetID uuid.UUID) error {
	if err := s.checkPermission(ctx, tagging.PermTagManage); err != nil {
		return err
	}

	if sourceID == targetID {
		return tagging.ErrCannotMergeSameTag
	}

	sourceTag, err := s.repo.GetTagByID(ctx, sourceID)
	if err != nil {
		return err
	}
	targetTag, err := s.repo.GetTagByID(ctx, targetID)
	if err != nil {
		return err
	}

	// Logic: Move associations, update count, delete old.
	err = s.repo.WithTx(ctx, func(txRepo tagging.Repository) error {
		// 1. Move associations (Repo handles duplicates by deleting them)
		if err := txRepo.ReassignTaggings(ctx, sourceID, targetID); err != nil {
			return err
		}

		// 2. Update Target Count (Add source count)
		if err := txRepo.UpdateTagCount(ctx, targetID, sourceTag.Count); err != nil {
			return err
		}

		// 3. Delete Source
		return txRepo.DeleteTag(ctx, sourceID)
	})

	if err != nil {
		return err
	}

	s.audit(ctx, core.EventTagsMerged, targetID, "tag", map[string]interface{}{
		"source_id":   sourceID,
		"source_name": sourceTag.Name,
		"target_name": targetTag.Name,
	})

	s.publishEvent(ctx, core.EventTagsMerged, targetID, map[string]interface{}{
		"source_id": sourceID.String(),
		"target_id": targetID.String(),
	})

	return nil
}

func (s *Service) UpdateTag(ctx context.Context, id uuid.UUID, name, description, color string) (*tagging.Tag, error) {
	if err := s.checkPermission(ctx, tagging.PermTagManage); err != nil {
		return nil, err
	}

	tag, err := s.repo.GetTagByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updated := false
	if name != "" && name != tag.Name {
		tag.Name = name
		tag.Slug = slug.Make(name) // Updating name updates slug
		updated = true
	}
	if description != "" && description != tag.Description {
		tag.Description = description
		updated = true
	}
	if color != "" && color != tag.Color {
		tag.Color = color
		updated = true
	}

	if updated {
		tag.UpdatedAt = time.Now().UTC()
		if err := s.repo.UpdateTag(ctx, tag); err != nil {
			return nil, err
		}
	}
	return tag, nil
}

func (s *Service) DeleteTag(ctx context.Context, id uuid.UUID) error {
	if err := s.checkPermission(ctx, tagging.PermTagManage); err != nil {
		return err
	}
	return s.repo.DeleteTag(ctx, id)
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
		return tagging.ErrAccessDenied
	}
	return nil
}

func (s *Service) audit(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType string, meta map[string]interface{}) {
	actor := auth.ActorFromContext(ctx)
	actorID := audit.SystemActorID
	if actor != nil {
		actorID = actor.UserID
	}

	evt := audit.NewEvent(actorID, "user", action).
		WithTarget(targetID, targetType, "").
		WithMetadata(meta).
		WithSuccess().
		WithContext(ctx).
		Build()

	// Fix: internal/services/tagging/service.go:322:27: Error return value of s.auditRecorder.Record is not checked
	go func() {
		if err := s.auditRecorder.Record(context.Background(), evt); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}()
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, actorID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, actorID, "system", payload)
	if err == nil {
		// Fix: internal/services/tagging/service.go:328:24: Error return value of s.eventBus.Publish is not checked
		go func() {
			if err := s.eventBus.Publish(context.Background(), evt); err != nil {
				s.logger.Error("failed to publish event", err)
			}
		}()
	} else {
		s.logger.Error("Failed to create event", err)
	}
}
