package catalog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/catalog"
	catalog_mocks "github.com/combi/pkg/domain/catalog/mocks"
	"github.com/combi/pkg/domain/core"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
)

type adminServiceTestSuite struct {
	repo     *catalog_mocks.MockRepository
	userSrv  *user_mocks.MockService
	rbac     *rbac_mocks.MockAuthorizationService
	audit    *audit_mocks.MockEventRecorder
	eventBus *events_mocks.MockBus
	service  *AdminService
}

func setupAdminService(t *testing.T) *adminServiceTestSuite {
	s := &adminServiceTestSuite{
		repo:     catalog_mocks.NewMockRepository(t),
		userSrv:  user_mocks.NewMockService(t),
		rbac:     rbac_mocks.NewMockAuthorizationService(t),
		audit:    audit_mocks.NewMockEventRecorder(t),
		eventBus: events_mocks.NewMockBus(t),
	}
	s.service = NewAdminService(
		s.repo, s.userSrv, s.rbac, s.audit, s.eventBus, &log.NoOpLogger{},
	)
	return s
}

func contextWithAdmin(ctx context.Context) (context.Context, uuid.UUID) {
	adminID := uuid.New()
	actor := &auth.Actor{
		UserID:    adminID,
		AccountID: adminID,
	}
	return auth.NewContextWithActor(ctx, actor), adminID
}

func TestAdminService_CreateItem(t *testing.T) {
	ctx := context.Background()
	adminCtx, _ := contextWithAdmin(ctx)

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)
		params := catalog.CreateItemParams{
			Name: "Bootcamp", Slug: "bootcamp", Type: "PROGRAM",
			Status: catalog.StatusDraft, CategoryIDs: []uuid.UUID{uuid.New()},
		}

		// 1. RBAC Check
		ts.rbac.EXPECT().Authorize(adminCtx, mock.MatchedBy(func(c interface{}) bool {
			return true
		})).Return(true, nil).Once()

		// 2. Transaction
		ts.repo.EXPECT().WithTx(adminCtx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(catalog.Repository) error) error {
			ts.repo.EXPECT().CreateItem(ctx, mock.Anything).Return(nil).Once()
			ts.repo.EXPECT().AddItemToCategory(ctx, mock.Anything, params.CategoryIDs[0]).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		// 3. Event & Audit
		ts.eventBus.EXPECT().Publish(adminCtx, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventItemCreated
		})).Return(nil).Once()
		ts.audit.EXPECT().Record(adminCtx, mock.Anything).Return(nil).Once()

		item, err := ts.service.CreateItem(adminCtx, params)
		require.NoError(t, err)
		assert.Equal(t, "Bootcamp", item.Name)
	})

	t.Run("permission denied", func(t *testing.T) {
		ts := setupAdminService(t)
		ts.rbac.EXPECT().Authorize(adminCtx, mock.Anything).Return(false, nil).Once()

		_, err := ts.service.CreateItem(adminCtx, catalog.CreateItemParams{})
		require.Error(t, err)
		// Expect FORBIDDEN (403) when Authorize returns false
		assert.Equal(t, apperror.Forbidden, apperror.GetType(err))
	})
}

func TestAdminService_UpdateItem(t *testing.T) {
	ctx := context.Background()
	adminCtx, _ := contextWithAdmin(ctx)
	itemID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)
		newName := "Updated Name"
		params := catalog.UpdateItemParams{Name: &newName}
		existing := &catalog.Item{ID: itemID, Name: "Old"}

		ts.rbac.EXPECT().Authorize(adminCtx, mock.Anything).Return(true, nil).Once()

		ts.repo.EXPECT().WithTx(adminCtx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(catalog.Repository) error) error {
			ts.repo.EXPECT().GetItem(ctx, itemID).Return(existing, nil).Once()
			ts.repo.EXPECT().UpdateItem(ctx, mock.MatchedBy(func(i *catalog.Item) bool {
				return i.Name == "Updated Name"
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		ts.eventBus.EXPECT().Publish(adminCtx, mock.Anything).Return(nil).Once()
		ts.audit.EXPECT().Record(adminCtx, mock.Anything).Return(nil).Once()

		item, err := ts.service.UpdateItem(adminCtx, itemID, params)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", item.Name)
	})
}

func TestAdminService_UpdateApplicationStatus(t *testing.T) {
	ctx := context.Background()
	adminCtx, _ := contextWithAdmin(ctx)
	appID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupAdminService(t)
		existingApp := &catalog.Application{ID: appID, Status: catalog.AppStatusPending}

		ts.rbac.EXPECT().Authorize(adminCtx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetApplication(adminCtx, appID).Return(existingApp, nil).Once()

		ts.repo.EXPECT().UpdateApplication(adminCtx, mock.MatchedBy(func(a *catalog.Application) bool {
			return a.Status == catalog.AppStatusApproved
		})).Return(nil).Once()

		ts.eventBus.EXPECT().Publish(adminCtx, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventApplicationStatusUpdated
		})).Return(nil).Once()
		ts.audit.EXPECT().Record(adminCtx, mock.Anything).Return(nil).Once()

		res, err := ts.service.UpdateApplicationStatus(adminCtx, appID, catalog.AppStatusApproved, nil)
		require.NoError(t, err)
		assert.Equal(t, catalog.AppStatusApproved, res.Status)
	})

	t.Run("cannot change final status", func(t *testing.T) {
		ts := setupAdminService(t)
		existingApp := &catalog.Application{ID: appID, Status: catalog.AppStatusRejected}

		ts.rbac.EXPECT().Authorize(adminCtx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetApplication(adminCtx, appID).Return(existingApp, nil).Once()

		_, err := ts.service.UpdateApplicationStatus(adminCtx, appID, catalog.AppStatusApproved, nil)
		require.ErrorIs(t, err, catalog.ErrInvalidStatus)
	})
}
