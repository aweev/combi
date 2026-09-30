package catalog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ catalog.Service = (*Service)(nil)

type Service struct {
	repo     catalog.Repository
	userSrv  user.Service
	logger   log.Logger
	audit    audit.EventRecorder
	eventBus events.Bus
}

func NewService(
	repo catalog.Repository,
	userSrv user.Service,
	logger log.Logger,
	audit audit.EventRecorder,
	eventBus events.Bus,
) *Service {
	return &Service{
		repo:     repo,
		userSrv:  userSrv,
		logger:   logger.With(log.String("service", "catalog_public")),
		audit:    audit,
		eventBus: eventBus,
	}
}

// --- Reads ---

func (s *Service) GetItem(ctx context.Context, id uuid.UUID) (*catalog.Item, error) {
	return s.repo.GetItem(ctx, id)
}

func (s *Service) GetItemBySlug(ctx context.Context, slug string) (*catalog.Item, error) {
	return s.repo.GetItemBySlug(ctx, slug)
}

func (s *Service) ListItems(ctx context.Context, filter catalog.ItemFilter) ([]*catalog.Item, int, error) {
	// For public API, we might want to force status=AVAILABLE here,
	// but usually the controller/filter handles that.
	return s.repo.ListItems(ctx, filter)
}

func (s *Service) ListVariants(ctx context.Context, itemID uuid.UUID) ([]*catalog.Variant, error) {
	return s.repo.ListVariants(ctx, itemID)
}

func (s *Service) GetCategoryTree(ctx context.Context) ([]*catalog.CategoryNode, error) {
	cats, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make(map[uuid.UUID]*catalog.CategoryNode)
	for _, c := range cats {
		nodes[c.ID] = &catalog.CategoryNode{Category: *c, Children: []*catalog.CategoryNode{}}
	}

	var roots []*catalog.CategoryNode
	for _, node := range nodes {
		if node.ParentID == nil {
			roots = append(roots, node)
		} else {
			if parent, exists := nodes[*node.ParentID]; exists {
				parent.Children = append(parent.Children, node)
			}
		}
	}
	return roots, nil
}

// --- Applications ---

func (s *Service) SubmitApplication(ctx context.Context, params catalog.SubmitApplicationParams) (*catalog.Application, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, apperror.New(401, apperror.Unauthorized, "authentication required", nil)
	}

	// 1. Validation
	item, err := s.repo.GetItem(ctx, params.ItemID)
	if err != nil {
		return nil, err
	}
	if item.Status != catalog.StatusAvailable {
		return nil, catalog.ErrItemNotAvailable
	}

	// 2. Create
	answersJSON, _ := json.Marshal(params.Answers)
	app := &catalog.Application{
		ID:        uuid.New(),
		UserID:    actor.UserID,
		ItemID:    params.ItemID,
		VariantID: params.VariantID,
		Status:    catalog.AppStatusPending,
		Answers:   answersJSON,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// 3. Persist
	if err := s.repo.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	// 4. Audit
	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventApplicationSubmitted).
		WithTarget(app.ID, "application", item.Name).
		WithSuccess().WithContext(ctx).Build()); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	// 5. Publish Event
	payload := catalog.ApplicationSubmittedPayload{
		ApplicationID: app.ID,
		ItemID:        app.ItemID,
		UserID:        app.UserID,
		Status:        string(app.Status),
	}

	evt, _ := events.NewEvent(core.EventApplicationSubmitted, actor.UserID, "user", payload)

	targetType := "catalog_application"
	evt.TargetID = &app.ID
	evt.TargetType = &targetType

	if err := s.eventBus.Publish(ctx, evt); err != nil {
		s.logger.Error("failed to publish application event", err, log.String("app_id", app.ID.String()))
	}

	return app, nil
}

func (s *Service) ListUserApplications(ctx context.Context, userID uuid.UUID) ([]*catalog.EnrichedApplication, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != userID {
		return nil, apperror.New(403, apperror.Unauthorized, "access denied", nil)
	}

	apps, err := s.repo.ListUserApplications(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(apps) == 0 {
		return []*catalog.EnrichedApplication{}, nil
	}

	// Enrich
	userProfile, err := s.userSrv.GetPublicProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	enriched := make([]*catalog.EnrichedApplication, len(apps))
	g, gCtx := errgroup.WithContext(ctx)
	itemMap := make(map[uuid.UUID]*catalog.Item)

	for _, a := range apps {
		aid := a.ItemID
		g.Go(func() error {
			it, err := s.repo.GetItem(gCtx, aid)
			if err == nil {
				itemMap[aid] = it
			}
			return nil
		})
	}
	_ = g.Wait()

	for i, a := range apps {
		item := itemMap[a.ItemID]
		if item == nil {
			item = &catalog.Item{Name: "Unknown Item"}
		}
		enriched[i] = &catalog.EnrichedApplication{
			Application: *a,
			User:        *userProfile,
			Item:        *item,
		}
	}

	return enriched, nil
}

func (s *Service) WithdrawApplication(ctx context.Context, appID, userID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != userID {
		return apperror.New(403, apperror.Unauthorized, "access denied", nil)
	}

	return s.repo.WithTx(ctx, func(txRepo catalog.Repository) error {
		app, err := txRepo.GetApplication(ctx, appID)
		if err != nil {
			return err
		}

		if app.UserID != userID {
			return catalog.ErrForbidden
		}
		if app.Status != catalog.AppStatusPending {
			return catalog.ErrWithdrawalNotAllowed
		}

		app.Status = catalog.AppStatusWithdrawn
		return txRepo.UpdateApplication(ctx, app)
	})
}
