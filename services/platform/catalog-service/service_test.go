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
	"github.com/combi/pkg/domain/user"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
)

type publicServiceTestSuite struct {
	repo     *catalog_mocks.MockRepository
	userSrv  *user_mocks.MockService
	audit    *audit_mocks.MockEventRecorder
	eventBus *events_mocks.MockBus
	service  *Service
}

func setupPublicService(t *testing.T) *publicServiceTestSuite {
	s := &publicServiceTestSuite{
		repo:     catalog_mocks.NewMockRepository(t),
		userSrv:  user_mocks.NewMockService(t),
		audit:    audit_mocks.NewMockEventRecorder(t),
		eventBus: events_mocks.NewMockBus(t),
	}
	s.service = NewService(
		s.repo, s.userSrv, &log.NoOpLogger{}, s.audit, s.eventBus,
	)
	return s
}

func contextWithUser(ctx context.Context) (context.Context, uuid.UUID) {
	userID := uuid.New()
	actor := &auth.Actor{UserID: userID, AccountID: userID}
	return auth.NewContextWithActor(ctx, actor), userID
}

func TestService_SubmitApplication(t *testing.T) {
	ctx := context.Background()
	userCtx, userID := contextWithUser(ctx)
	itemID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupPublicService(t)
		params := catalog.SubmitApplicationParams{
			ItemID:  itemID,
			Answers: map[string]any{"q1": "a1"},
		}
		item := &catalog.Item{ID: itemID, Status: catalog.StatusAvailable, Name: "Test"}

		// 1. Validation
		ts.repo.EXPECT().GetItem(userCtx, itemID).Return(item, nil).Once()

		// 2. Persist
		ts.repo.EXPECT().CreateApplication(userCtx, mock.MatchedBy(func(a *catalog.Application) bool {
			return a.UserID == userID && a.ItemID == itemID && a.Status == catalog.AppStatusPending
		})).Return(nil).Once()

		// 3. Audit & Event
		ts.audit.EXPECT().Record(userCtx, mock.Anything).Return(nil).Once()

		ts.eventBus.EXPECT().Publish(userCtx, mock.MatchedBy(func(e events.DomainEvent) bool {
			if e.Action != core.EventApplicationSubmitted {
				return false
			}
			// FIX: Check TargetID is not nil and not equal to uuid.Nil
			// TargetID is a *string containing the UUID
			if e.TargetID == nil || *e.TargetID == uuid.Nil {
				return false
			}
			return true
		})).Return(nil).Once()

		app, err := ts.service.SubmitApplication(userCtx, params)
		require.NoError(t, err)
		assert.Equal(t, userID, app.UserID)
	})

	t.Run("item not available", func(t *testing.T) {
		ts := setupPublicService(t)
		params := catalog.SubmitApplicationParams{ItemID: itemID}
		item := &catalog.Item{ID: itemID, Status: catalog.StatusDraft}

		ts.repo.EXPECT().GetItem(userCtx, itemID).Return(item, nil).Once()

		_, err := ts.service.SubmitApplication(userCtx, params)
		assert.ErrorIs(t, err, catalog.ErrItemNotAvailable)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		ts := setupPublicService(t)
		// Use raw background context (no actor)
		_, err := ts.service.SubmitApplication(ctx, catalog.SubmitApplicationParams{})
		require.Error(t, err)
		// Expect UNAUTHORIZED (401) when no actor present
		assert.Equal(t, apperror.Unauthorized, apperror.GetType(err))
	})
}

func TestService_ListUserApplications(t *testing.T) {
	ctx := context.Background()
	userCtx, userID := contextWithUser(ctx)

	t.Run("success enriched", func(t *testing.T) {
		ts := setupPublicService(t)
		itemID := uuid.New()

		apps := []*catalog.Application{
			{ID: uuid.New(), UserID: userID, ItemID: itemID},
		}
		item := &catalog.Item{ID: itemID, Name: "Item 1"}
		// FIX: Create a valid public profile pointer
		validProfile := &user.PublicProfile{UserID: userID, FirstName: "Test", LastName: "User"}

		ts.repo.EXPECT().ListUserApplications(userCtx, userID).Return(apps, nil).Once()

		// FIX: Return the valid profile
		ts.userSrv.EXPECT().GetPublicProfile(userCtx, userID).Return(validProfile, nil).Once()

		ts.repo.EXPECT().GetItem(mock.Anything, itemID).Return(item, nil).Once()

		res, err := ts.service.ListUserApplications(userCtx, userID)
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "Item 1", res[0].Item.Name)
		assert.Equal(t, "Test", res[0].User.FirstName)
	})
}

func TestService_WithdrawApplication(t *testing.T) {
	ctx := context.Background()
	userCtx, userID := contextWithUser(ctx)
	appID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupPublicService(t)
		app := &catalog.Application{ID: appID, UserID: userID, Status: catalog.AppStatusPending}

		ts.repo.EXPECT().WithTx(userCtx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(catalog.Repository) error) error {
			ts.repo.EXPECT().GetApplication(ctx, appID).Return(app, nil).Once()
			ts.repo.EXPECT().UpdateApplication(ctx, mock.MatchedBy(func(a *catalog.Application) bool {
				return a.Status == catalog.AppStatusWithdrawn
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		err := ts.service.WithdrawApplication(userCtx, appID, userID)
		require.NoError(t, err)
	})

	t.Run("forbidden", func(t *testing.T) {
		ts := setupPublicService(t)
		otherUser := uuid.New()
		app := &catalog.Application{ID: appID, UserID: otherUser, Status: catalog.AppStatusPending}

		ts.repo.EXPECT().WithTx(userCtx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(catalog.Repository) error) error {
			ts.repo.EXPECT().GetApplication(ctx, appID).Return(app, nil).Once()
			// Should fail before Update
			return fn(ts.repo)
		}).Once()

		err := ts.service.WithdrawApplication(userCtx, appID, userID)
		require.ErrorIs(t, err, catalog.ErrForbidden)
	})
}
