package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
	"github.com/combi/workflow"

	// Mocks
	catalog_mocks "github.com/combi/pkg/domain/catalog/mocks"
	notif_mocks "github.com/combi/pkg/domain/notification/mocks"
	org_mocks "github.com/combi/pkg/domain/organization/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	user_mocks "github.com/combi/pkg/domain/user/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type activitiesSuite struct {
	logger       log.Logger
	catalog      *catalog_mocks.MockAdminService
	notification *notif_mocks.MockService
	userAdmin    *user_mocks.MockAdminService
	rbacAdmin    *rbac_mocks.MockAdminService
	orgSvc       *org_mocks.MockService
	activities   *workflow.Activities
}

func setupActivities(t *testing.T) *activitiesSuite {
	s := &activitiesSuite{
		logger:       &log.NoOpLogger{},
		catalog:      catalog_mocks.NewMockAdminService(t),
		notification: notif_mocks.NewMockService(t),
		userAdmin:    user_mocks.NewMockAdminService(t),
		rbacAdmin:    rbac_mocks.NewMockAdminService(t),
		orgSvc:       org_mocks.NewMockService(t),
	}

	// Initialize with subset of mocks needed for tests
	s.activities = workflow.NewActivities(
		s.logger,
		s.catalog,
		s.notification,
		nil, // UserSvc
		s.userAdmin,
		nil, // ContentAdmin
		s.orgSvc,
		nil, // Project
		s.rbacAdmin,
		nil, // Financial
	)
	return s
}

func TestActivities_ActivateParticipantProfile(t *testing.T) {
	s := setupActivities(t)
	ctx := context.Background()
	userID := uuid.New()
	roleID := uuid.New()

	t.Run("success", func(t *testing.T) {
		// 1. Update Status
		s.userAdmin.On("UpdateUserStatus", ctx, userID, user.StatusActive, (*string)(nil)).
			Return(&user.User{}, nil).Once()

		// 2. Get Role
		s.rbacAdmin.On("GetRoleBySlug", ctx, "participant").
			Return(&rbac.Role{ID: roleID}, nil).Once()

		// 3. Assign Role
		s.rbacAdmin.On("AssignRoleToUser", ctx, userID, roleID).
			Return(&rbac.RoleAssignment{}, nil).Once()

		err := s.activities.ActivateParticipantProfile(ctx, userID)
		require.NoError(t, err)
	})

	t.Run("failure on user update", func(t *testing.T) {
		s.userAdmin.On("UpdateUserStatus", ctx, userID, user.StatusActive, (*string)(nil)).
			Return(nil, errors.New("db error")).Once()

		err := s.activities.ActivateParticipantProfile(ctx, userID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to update user status")
	})
}

func TestActivities_DeactivateParticipantProfile(t *testing.T) {
	s := setupActivities(t)
	ctx := context.Background()
	userID := uuid.New()
	roleID := uuid.New()

	// This is a compensation activity, so it shouldn't error hard on partial failures
	t.Run("runs all steps even if one fails", func(t *testing.T) {
		// 1. Revert Status (Fails)
		s.userAdmin.On("UpdateUserStatus", ctx, userID, user.StatusPending, (*string)(nil)).
			Return(nil, errors.New("cannot revert")).Once()

		// 2. Get Role (Success)
		s.rbacAdmin.On("GetRoleBySlug", ctx, "participant").
			Return(&rbac.Role{ID: roleID}, nil).Once()

		// 3. Revoke Role (Success)
		s.rbacAdmin.On("RevokeRoleFromUser", ctx, userID, roleID).
			Return(nil).Once()

		err := s.activities.DeactivateParticipantProfile(ctx, userID)
		require.NoError(t, err) // Should handle errors internally
	})
}

func TestActivities_CreatePartnerOrganization(t *testing.T) {
	s := setupActivities(t)
	ctx := context.Background()
	ownerID := uuid.New()

	t.Run("passes explicit owner id", func(t *testing.T) {
		s.orgSvc.On("CreateOrganization", ctx, mock.MatchedBy(func(p organization.CreateOrganizationParams) bool {
			return p.OwnerUserID == ownerID && p.Name == "Acme" && p.Type == "PARTNER"
		})).Return(&organization.Organization{ID: uuid.New()}, nil).Once()

		_, err := s.activities.CreatePartnerOrganization(ctx, "Acme", "acme", ownerID)
		require.NoError(t, err)
	})
}

func TestActivities_SendNotification(t *testing.T) {
	s := setupActivities(t)
	ctx := context.Background()
	userID := uuid.New()
	refID := uuid.New()

	t.Run("dispatches correctly", func(t *testing.T) {
		s.notification.On("Dispatch", ctx, mock.MatchedBy(func(req notification.DispatchRequest) bool {
			return req.RecipientID == userID &&
				req.Template.Slug == "welcome" &&
				req.Reference.EntityID == refID &&
				req.Reference.EntityType == "application"
		})).Return(&notification.Notification{}, nil).Once()

		err := s.activities.SendNotification(ctx, userID, "welcome", map[string]interface{}{"a": 1}, "application", refID)
		require.NoError(t, err)
	})
}
