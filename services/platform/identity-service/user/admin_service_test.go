package user_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/domain/user"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	"github.com/combi/pkg/log"
	userService "github.com/combi/identity/user"
)

// adminServiceTestSuite holds dependencies for testing the AdminService.
type adminServiceTestSuite struct {
	repo          *user_mocks.MockRepository
	auditRecorder *audit_mocks.MockEventRecorder
	authAdminSvc  *auth_mocks.MockAdminService
	authz         *rbac_mocks.MockAuthorizationService
	service       user.AdminService
	storage       *storage_mocks.MockService
}

// setupAdminService initializes a new test suite for the user admin service.
func setupAdminService(t *testing.T) adminServiceTestSuite {
	t.Helper()
	var ts adminServiceTestSuite
	ts.repo = user_mocks.NewMockRepository(t)
	ts.auditRecorder = audit_mocks.NewMockEventRecorder(t)
	ts.authAdminSvc = auth_mocks.NewMockAdminService(t)
	ts.authz = rbac_mocks.NewMockAuthorizationService(t)

	ts.storage = storage_mocks.NewMockService(t)
	// The AdminService receives storage directly, no need to mock GetStorage on authAdminSvc

	ts.service = userService.NewAdminService(
		ts.repo,
		ts.auditRecorder,
		ts.authAdminSvc,
		ts.authz,
		ts.storage,
		&log.NoOpLogger{},
	)
	return ts
}

func TestAdminService_GetUserDetails(t *testing.T) {
	adminCtx, _ := contextWithActor()
	targetUserID := uuid.New()

	t.Run("should get user details with permission", func(t *testing.T) {
		ts := setupAdminService(t)
		testUser := &user.User{ID: targetUserID}
		testProfile := &user.Profile{ID: targetUserID}

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil).Once()
		ts.repo.On("GetUser", adminCtx, targetUserID).Return(testUser, nil).Once()
		ts.repo.On("GetProfile", adminCtx, targetUserID).Return(testProfile, nil).Once()

		details, err := ts.service.GetUserDetails(adminCtx, targetUserID)
		require.NoError(t, err)
		assert.Equal(t, testUser, details.User)
		assert.Equal(t, testProfile, details.Profile)
	})

	t.Run("should fail without permission", func(t *testing.T) {
		ts := setupAdminService(t)
		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(false, nil).Once()

		_, err := ts.service.GetUserDetails(adminCtx, targetUserID)
		require.ErrorIs(t, err, rbac.ErrAccessDenied)
	})
}

func TestAdminService_UpdateUserStatus(t *testing.T) {
	adminCtx, _ := contextWithActor()
	targetUserID := uuid.New()
	reason := "User violated terms of service."

	t.Run("should deactivate user and invalidate sessions", func(t *testing.T) {
		ts := setupAdminService(t)
		originalUser := &user.User{ID: targetUserID, Status: user.StatusActive}

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil).Once()

		ts.repo.On("WithTx", adminCtx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(user.Repository) error)

			ts.repo.On("GetUser", adminCtx, targetUserID).Return(originalUser, nil)
			ts.repo.On("UpdateUser", adminCtx, mock.Anything).Return(nil)

			ts.authAdminSvc.On("UpdateAccountStatus", adminCtx, targetUserID, auth.StatusSuspended, mock.AnythingOfType("*string")).Return(&auth.Account{}, nil)

			ts.authAdminSvc.On("AdminInvalidateAllSessionsForAccount", adminCtx, targetUserID).Return(nil)
			// Execute logic
			err := fn(ts.repo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.auditRecorder.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.UpdateUserStatus(adminCtx, targetUserID, user.StatusDeactivated, &reason)
		require.NoError(t, err)
	})

	t.Run("should return error if auth service fails to update status", func(t *testing.T) {
		ts := setupAdminService(t)
		originalUser := &user.User{ID: targetUserID, Status: user.StatusActive}
		authErr := errors.New("auth service unavailable")

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil).Once()

		// --- MOCKING THE TRANSACTION ---
		ts.repo.On("WithTx", adminCtx, mock.AnythingOfType("func(user.Repository) error")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(user.Repository) error)

				// Set up mocks for calls made *inside* the transaction function
				ts.repo.On("GetUser", adminCtx, targetUserID).Return(originalUser, nil).Once()
				ts.repo.On("UpdateUser", adminCtx, mock.Anything).Return(nil).Once()

				// THIS IS THE CALL THAT WILL FAIL
				ts.authAdminSvc.On("UpdateAccountStatus", adminCtx, targetUserID, auth.StatusSuspended, &reason).Return(nil, authErr).Once()

				// Execute the function passed to WithTx and assert it returns an error
				err := fn(ts.repo)
				require.Error(t, err)
				assert.ErrorIs(t, err, authErr, "The inner function should return the auth error")

			}).
			Return(fmt.Errorf("failed to update auth account status: %w", authErr)).Once()
		// --- END MOCKING ---

		// --- Act ---
		_, err := ts.service.UpdateUserStatus(adminCtx, targetUserID, user.StatusDeactivated, &reason)

		// --- Assert ---
		require.Error(t, err)
		assert.ErrorIs(t, err, authErr, "The final error should wrap the original auth service error")
		assert.Contains(t, err.Error(), "failed to update auth account status")

		// No audit event should be recorded on failure
		ts.auditRecorder.AssertNotCalled(t, "Record")
	})

	t.Run("should do nothing if status is not changed", func(t *testing.T) {
		ts := setupAdminService(t)
		originalUser := &user.User{ID: targetUserID, Status: user.StatusActive}

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil)
		ts.repo.On("WithTx", adminCtx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(user.Repository) error)
			ts.repo.On("GetUser", adminCtx, targetUserID).Return(originalUser, nil)
			fn(ts.repo)
		}).Return(nil).Once()

		_, err := ts.service.UpdateUserStatus(adminCtx, targetUserID, user.StatusActive, nil)

		require.NoError(t, err)
		// Crucially, no update or audit calls should be made.
		ts.repo.AssertNotCalled(t, "UpdateUser")
		ts.authAdminSvc.AssertNotCalled(t, "UpdateAccountStatus")
		ts.auditRecorder.AssertNotCalled(t, "Record")
	})
	t.Run("should return error and rollback if user update fails", func(t *testing.T) {
		ts := setupAdminService(t)
		originalUser := &user.User{ID: targetUserID, Status: user.StatusActive}
		repoErr := errors.New("database update failed")

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil).Once()

		ts.repo.On("WithTx", adminCtx, mock.AnythingOfType("func(user.Repository) error")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(user.Repository) error)
				// Mocks for inside the transaction
				ts.repo.On("GetUser", adminCtx, targetUserID).Return(originalUser, nil).Once()
				// THIS IS THE CALL THAT WILL FAIL
				ts.repo.On("UpdateUser", adminCtx, mock.Anything).Return(repoErr).Once()

				err := fn(ts.repo)
				require.ErrorIs(t, err, repoErr)
			}).
			Return(fmt.Errorf("failed to update user status in repository: %w", repoErr)).Once()

		// --- Act ---
		_, err := ts.service.UpdateUserStatus(adminCtx, targetUserID, user.StatusDeactivated, &reason)

		// --- Assert ---
		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)

		// The auth service should not have been called if the user update failed first.
		ts.authAdminSvc.AssertNotCalled(t, "UpdateAccountStatus")
		ts.auditRecorder.AssertNotCalled(t, "Record")
	})
}

func TestAdminService_UpdateUser(t *testing.T) {
	adminCtx, _ := contextWithActor()
	targetUserID := uuid.New()
	originalEmail := core.MustNewEmail(t, "original@test.com")
	newEmail := core.MustNewEmail(t, "new@test.com")

	t.Run("should update user name and email successfully", func(t *testing.T) {
		ts := setupAdminService(t)
		firstName := "Jane"
		lastName := "Doe"
		params := user.UpdateUserParams{FirstName: &firstName, LastName: &lastName, Email: &newEmail}
		originalUser := &user.User{ID: targetUserID, Email: originalEmail}

		ts.authz.On("Authorize", adminCtx, mock.Anything).Return(true, nil)
		ts.repo.On("WithTx", adminCtx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(user.Repository) error)
			ts.repo.On("GetUser", adminCtx, targetUserID).Return(originalUser, nil)
			// Mock call to auth service for email change
			ts.authAdminSvc.On("AdminUpdateAccountEmail", adminCtx, targetUserID, newEmail.String()).Return(nil)
			ts.repo.On("UpdateUser", adminCtx, mock.MatchedBy(func(u *user.User) bool {
				return u.FirstName == firstName && u.LastName == lastName && u.Email == newEmail
			})).Return(nil)
			fn(ts.repo)
		}).Return(nil)
		ts.auditRecorder.On("Record", mock.Anything, mock.Anything).Return(nil)

		updatedUser, err := ts.service.UpdateUser(adminCtx, targetUserID, params)

		require.NoError(t, err)
		assert.Equal(t, newEmail.String(), updatedUser.Email.String())
		assert.Equal(t, firstName, updatedUser.FirstName)
	})
}
