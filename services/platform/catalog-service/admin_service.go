package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ catalog.AdminService = (*AdminService)(nil)

type AdminService struct {
	repo     catalog.Repository
	userSrv  user.Service
	rbac     rbac.AuthorizationService
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewAdminService(
	repo catalog.Repository,
	userSrv user.Service,
	rbac rbac.AuthorizationService,
	audit audit.EventRecorder,
	eventBus events.Bus,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:     repo,
		userSrv:  userSrv,
		rbac:     rbac,
		audit:    audit,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "catalog_admin")),
	}
}

// --- Item CRUD ---

func (s *AdminService) CreateItem(ctx context.Context, params catalog.CreateItemParams) (*catalog.Item, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogManage); err != nil {
		return nil, err
	}

	tagsJSON, _ := json.Marshal(params.Tags)
	propsJSON, _ := json.Marshal(params.Properties)

	item := &catalog.Item{
		ID:          uuid.New(),
		Type:        params.Type,
		Name:        params.Name,
		Slug:        params.Slug,
		Description: params.Description,
		Status:      params.Status,
		Tags:        tagsJSON,
		Properties:  propsJSON,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// 1. DB Transaction
	err := s.repo.WithTx(ctx, func(txRepo catalog.Repository) error {
		if err := txRepo.CreateItem(ctx, item); err != nil {
			return err
		}
		for _, catID := range params.CategoryIDs {
			if err := txRepo.AddItemToCategory(ctx, item.ID, catID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 2. Event Emission (Source of Truth Broadcast)
	// Listeners: Search Service (Index), Automation (Alerts)
	s.publishEvent(ctx, core.EventItemCreated, item.ID, "catalog_item", item)

	// 3. Audit
	s.auditSuccess(ctx, core.EventItemCreated, item.ID, "item", "Item created")

	return item, nil
}

func (s *AdminService) UpdateItem(ctx context.Context, itemID uuid.UUID, params catalog.UpdateItemParams) (*catalog.Item, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogManage); err != nil {
		return nil, err
	}

	var updatedItem *catalog.Item

	err := s.repo.WithTx(ctx, func(txRepo catalog.Repository) error {
		item, err := txRepo.GetItem(ctx, itemID)
		if err != nil {
			return err
		}

		if params.Name != nil {
			item.Name = *params.Name
		}
		if params.Slug != nil {
			item.Slug = *params.Slug
		}
		if params.Description != nil {
			item.Description = *params.Description
		}
		if params.Status != nil {
			item.Status = *params.Status
		}
		if params.Properties != nil {
			b, _ := json.Marshal(params.Properties)
			item.Properties = b
		}
		if params.Tags != nil {
			b, _ := json.Marshal(params.Tags)
			item.Tags = b
		}

		item.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateItem(ctx, item); err != nil {
			return err
		}

		if params.CategoryIDs != nil {
			if err := txRepo.RemoveAllCategoriesFromItem(ctx, item.ID); err != nil {
				return err
			}
			for _, catID := range params.CategoryIDs {
				if err := txRepo.AddItemToCategory(ctx, item.ID, catID); err != nil {
					return err
				}
			}
		}
		updatedItem = item
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, core.EventItemUpdated, updatedItem.ID, "catalog_item", updatedItem)
	s.auditSuccess(ctx, core.EventItemUpdated, itemID, "item", "Item updated")

	return updatedItem, nil
}

func (s *AdminService) DeleteItem(ctx context.Context, itemID uuid.UUID) error {
	if err := s.checkPerm(ctx, catalog.PermCatalogManage); err != nil {
		return err
	}

	if err := s.repo.DeleteItem(ctx, itemID); err != nil {
		return err
	}

	// Payload is just ID for deletion, usually handled by metadata in NewEvent,
	// but passing explicit nil payload signals data is gone.
	s.publishEvent(ctx, core.EventItemDeleted, itemID, "catalog_item", nil)
	s.auditSuccess(ctx, core.EventItemDeleted, itemID, "item", "Item deleted")

	return nil
}

// --- Category CRUD (Standard) ---

func (s *AdminService) CreateCategory(ctx context.Context, params catalog.CreateCategoryParams) (*catalog.Category, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogCategoryManage); err != nil {
		return nil, err
	}
	cat := &catalog.Category{
		ID: uuid.New(), Name: params.Name, Slug: params.Slug,
		ParentID: params.ParentID, Description: params.Description,
	}
	if err := s.repo.CreateCategory(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *AdminService) UpdateCategory(ctx context.Context, id uuid.UUID, params catalog.UpdateCategoryParams) (*catalog.Category, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogCategoryManage); err != nil {
		return nil, err
	}
	cat, err := s.repo.GetCategory(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		cat.Name = *params.Name
	}
	if params.Slug != nil {
		cat.Slug = *params.Slug
	}
	if params.Description != nil {
		cat.Description = params.Description
	}
	if params.ParentID != nil {
		cat.ParentID = params.ParentID
	}

	if err := s.repo.UpdateCategory(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *AdminService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	if err := s.checkPerm(ctx, catalog.PermCatalogCategoryManage); err != nil {
		return err
	}
	return s.repo.DeleteCategory(ctx, id)
}

// --- Application Management ---

func (s *AdminService) GetApplication(ctx context.Context, id uuid.UUID) (*catalog.EnrichedApplication, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogApplicationRead); err != nil {
		return nil, err
	}
	app, err := s.repo.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	enriched, err := s.enrichApplications(ctx, []*catalog.Application{app})
	if err != nil {
		return nil, err
	}
	return enriched[0], nil
}

func (s *AdminService) ListApplicationsForItem(ctx context.Context, itemID uuid.UUID, filter catalog.ApplicationFilter) ([]*catalog.EnrichedApplication, int, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogApplicationRead); err != nil {
		return nil, 0, err
	}
	apps, total, err := s.repo.ListApplications(ctx, itemID, filter)
	if err != nil {
		return nil, 0, err
	}
	enriched, err := s.enrichApplications(ctx, apps)
	return enriched, total, err
}

func (s *AdminService) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status catalog.ApplicationStatus, reason *string) (*catalog.Application, error) {
	if err := s.checkPerm(ctx, catalog.PermCatalogApplicationWrite); err != nil {
		return nil, err
	}

	app, err := s.repo.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}

	if app.Status.IsFinal() {
		return nil, catalog.ErrInvalidStatus
	}

	app.Status = status
	if err := s.repo.UpdateApplication(ctx, app); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, core.EventApplicationStatusUpdated, app.ID, "catalog_application", app)
	s.auditSuccess(ctx, core.EventApplicationStatusUpdated, id, "application", fmt.Sprintf("Status changed to %s", status))

	return app, nil
}

// --- Private Helpers ---

func (s *AdminService) checkPerm(ctx context.Context, perm core.PermissionName) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		// 401 Unauthorized: User is not logged in
		return apperror.New(401, apperror.Unauthorized, "not authenticated", nil)
	}

	allowed, err := s.rbac.Authorize(ctx, rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: perm,
	})
	if err != nil {
		return err
	}

	if !allowed {
		// FIX: Return 403 Forbidden here, NOT Unauthorized
		// 403 Forbidden: User is logged in but lacks permission
		return apperror.New(403, apperror.Forbidden, "access denied", nil)
	}
	return nil
}

func (s *AdminService) auditSuccess(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType, msg string) {
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
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *AdminService) publishEvent(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType string, data interface{}) {
	userID := uuid.Nil
	if actor := auth.ActorFromContext(ctx); actor != nil {
		userID = actor.UserID
	}

	var payload []byte
	if data != nil {
		var err error
		payload, err = json.Marshal(data)
		if err != nil {
			s.logger.Error("failed to marshal event payload", err)
			return
		}
	}

	// Using the platform event factory
	evt, err := events.NewEvent(action, userID, "user", payload)
	if err != nil {
		s.logger.Error("failed to create event", err, log.String("action", string(action)))
		return
	}

	// Add metadata to help routing
	evt.TargetID = &targetID
	evt.TargetType = &targetType

	if err := s.eventBus.Publish(ctx, evt); err != nil {
		s.logger.Error("failed to publish catalog event", err, log.String("action", string(action)))
	}
}

func (s *AdminService) enrichApplications(ctx context.Context, apps []*catalog.Application) ([]*catalog.EnrichedApplication, error) {
	if len(apps) == 0 {
		return nil, nil
	}

	userIDs := make([]uuid.UUID, len(apps))
	itemIDs := make([]uuid.UUID, len(apps))
	for i, a := range apps {
		userIDs[i] = a.UserID
		itemIDs[i] = a.ItemID
	}

	var users map[uuid.UUID]user.PublicProfile
	itemMap := make(map[uuid.UUID]*catalog.Item)

	g, gCtx := errgroup.WithContext(ctx)

	// Concurrent fetch: Users
	g.Go(func() error {
		var err error
		users, err = s.userSrv.GetPublicProfiles(gCtx, userIDs)
		return err
	})

	// Concurrent fetch: Items
	g.Go(func() error {
		// Optimization: Check if repo supports GetItemsByIds(ids)
		for _, id := range itemIDs {
			if _, exists := itemMap[id]; exists {
				continue
			}
			it, err := s.repo.GetItem(gCtx, id)
			if err == nil {
				itemMap[id] = it
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := make([]*catalog.EnrichedApplication, len(apps))
	for i, app := range apps {
		u := users[app.UserID]
		item := itemMap[app.ItemID]
		if item == nil {
			item = &catalog.Item{Name: "Unknown Item"}
		}
		result[i] = &catalog.EnrichedApplication{Application: *app, User: u, Item: *item}
	}
	return result, nil
}
