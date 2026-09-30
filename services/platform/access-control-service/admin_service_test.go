// Ported from api/internal/services/rbac/admin_service_test.go.
package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
)

// adminServiceTestSuite holds all mocked dependencies for the AdminService.
type adminServiceTestSuite struct {
	repo       *rbac_mocks.MockRepository
	cache      *mocks.MockService
	dispatcher *queue_mocks.MockDispatcher
	audit      *audit_mocks.MockEventRecorder
	service    *AdminService
}

// setupAdminService initializes a new test suite with fresh mocks.
func setupAdminService(t *testing.T) adminServiceTestSuite {
	t.Helper()
	var ts adminServiceTestSuite
	ts.repo = rbac_mocks.NewMockRepository(t)
	ts.cache = mocks.NewMockService(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)

	ts.service = NewAdminService(
		ts.repo,
		ts.cache,
		ts.dispatcher,
		ts.audit,
		&log.NoOpLogger{},
	)
	return ts
}

// Helper to create a context with a test user actor.
func contextWithActor() (context.Context, *auth.Actor) {
	actor := &auth.Actor{AccountID: uuid.New(), UserID: uuid.New()}
	return auth.NewContextWithActor(context.Background(), actor), actor
}

func TestAdminService_AssignRoleToUser(t *testing.T) {
	ctx, _ := contextWithActor()
	userID, roleID := uuid.New(), uuid.New()

	t.Run("should assign role and clear user cache successfully", func(t *testing.T) {
		ts := setupAdminService(t)

		// --- Arrange Mocks ---
		ts.repo.On("AssignRoleToUser", ctx, mock.AnythingOfType("*rbac.RoleAssignment")).Return(nil).Once()
		// CRITICAL: Expect the cache to be cleared for the specific user.
		ts.cache.On("Delete", ctx, ts.service.buildUserPermissionsCacheKey(userID)).Return(nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		// --- Act ---
		assignment, err := ts.service.AssignRoleToUser(ctx, userID, roleID)

		// --- Assert ---
		require.NoError(t, err)
		require.NotNil(t, assignment)
		assert.Equal(t, userID, assignment.UserID)
		assert.Equal(t, roleID, assignment.RoleID)
	})

	t.Run("should still succeed if cache clearing fails", func(t *testing.T) {
		ts := setupAdminService(t)
		cacheErr := errors.New("redis connection failed")

		ts.repo.On("AssignRoleToUser", ctx, mock.Anything).Return(nil).Once()
		// Simulate cache failure.
		ts.cache.On("Delete", ctx, mock.Anything).Return(cacheErr).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		// --- Act ---
		_, err := ts.service.AssignRoleToUser(ctx, userID, roleID)

		// --- Assert ---
		// The primary operation should NOT fail just because the cache invalidation did.
		// The error should be logged, which our NoOpLogger will swallow.
		require.NoError(t, err)
	})
}

func TestAdminService_RevokeRoleFromUser(t *testing.T) {
	ctx, _ := contextWithActor()
	userID, roleID := uuid.New(), uuid.New()

	t.Run("should revoke role and clear user cache successfully", func(t *testing.T) {
		ts := setupAdminService(t)

		// --- Arrange Mocks ---
		ts.repo.On("RevokeRoleFromUser", ctx, userID, roleID).Return(nil).Once()
		ts.cache.On("Delete", ctx, ts.service.buildUserPermissionsCacheKey(userID)).Return(nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		// --- Act ---
		err := ts.service.RevokeRoleFromUser(ctx, userID, roleID)

		// --- Assert ---
		require.NoError(t, err)
	})
}

func TestAdminService_UpdateRole(t *testing.T) {
	ctx, _ := contextWithActor()
	roleID := uuid.New()
	role := &rbac.Role{ID: roleID, Name: "Original Name", IsActive: true, Type: rbac.RoleTypeCustom}

	t.Run("should update name without dispatching invalidation job", func(t *testing.T) {
		ts := setupAdminService(t)
		newName := "Updated Name"
		params := rbac.UpdateRoleParams{Name: &newName}

		// --- Arrange Mocks ---
		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		ts.repo.On("UpdateRole", ctx, mock.MatchedBy(func(r *rbac.Role) bool {
			return r.Name == newName
		})).Return(nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		// --- Act ---
		_, err := ts.service.UpdateRole(ctx, roleID, params)

		// --- Assert ---
		require.NoError(t, err)
		// CRITICAL: The dispatcher should NOT be called for a simple name change.
		ts.dispatcher.AssertNotCalled(t, "Dispatch")
	})

	t.Run("should dispatch invalidation job when IsActive status changes", func(t *testing.T) {
		ts := setupAdminService(t)
		newIsActive := false
		params := rbac.UpdateRoleParams{IsActive: &newIsActive}

		// --- Arrange Mocks ---
		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		ts.repo.On("UpdateRole", ctx, mock.MatchedBy(func(r *rbac.Role) bool {
			return r.IsActive == newIsActive
		})).Return(nil).Once()
		// CRITICAL: Expect the dispatcher to be called.
		ts.dispatcher.On("Dispatch", ctx, rbac.JobRBACInvalidateUserCache, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		// --- Act ---
		_, err := ts.service.UpdateRole(ctx, roleID, params)

		// --- Assert ---
		require.NoError(t, err)
	})

	t.Run("should return error for system roles", func(t *testing.T) {
		ts := setupAdminService(t)
		systemRole := &rbac.Role{ID: roleID, Type: rbac.RoleTypeSystem}
		params := rbac.UpdateRoleParams{}

		ts.repo.On("GetRole", ctx, roleID).Return(systemRole, nil).Once()

		_, err := ts.service.UpdateRole(ctx, roleID, params)
		require.Error(t, err)
		assert.True(t, errors.Is(err, rbac.ErrSystemRoleImmutable))
	})
}

func TestAdminService_AddAndRemovePermissionsToRole(t *testing.T) {
	ctx, _ := contextWithActor()
	roleID := uuid.New()
	permIDs := []uuid.UUID{uuid.New(), uuid.New()}

	t.Run("AddPermissionsToRole should dispatch invalidation job", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.On("AddPermissionsToRole", ctx, roleID, permIDs).Return(nil).Once()
		ts.dispatcher.On("Dispatch", ctx, rbac.JobRBACInvalidateUserCache, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.AddPermissionsToRole(ctx, roleID, permIDs)
		require.NoError(t, err)
	})

	t.Run("RemovePermissionsFromRole should dispatch invalidation job", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.On("RemovePermissionsFromRole", ctx, roleID, permIDs).Return(nil).Once()
		ts.dispatcher.On("Dispatch", ctx, rbac.JobRBACInvalidateUserCache, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.RemovePermissionsFromRole(ctx, roleID, permIDs)
		require.NoError(t, err)
	})
}

func TestAdminService_DeleteRole(t *testing.T) {
	ctx, _ := contextWithActor()
	roleID := uuid.New()
	role := &rbac.Role{ID: roleID, Name: "To Delete", Type: rbac.RoleTypeCustom}

	t.Run("should delete role successfully if not in use", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		ts.repo.On("CountUsersInRole", ctx, roleID).Return(0, nil).Once() // Not in use
		ts.repo.On("DeleteRole", ctx, roleID).Return(nil).Once()
		ts.audit.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.DeleteRole(ctx, roleID)
		require.NoError(t, err)
	})

	t.Run("should return ErrRoleInUse if role is assigned to users", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		ts.repo.On("CountUsersInRole", ctx, roleID).Return(5, nil).Once() // In use

		err := ts.service.DeleteRole(ctx, roleID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, rbac.ErrRoleInUse))
		ts.repo.AssertNotCalled(t, "DeleteRole") // Should not proceed to delete
	})
}

func TestAdminService_CreateRole(t *testing.T) {
	ctx, actor := contextWithActor()
	params := rbac.CreateRoleParams{Name: "New Role", Slug: "new-role"}

	t.Run("should create role within a transaction", func(t *testing.T) {
		ts := setupAdminService(t)

		// Mock the transaction wrapper
		ts.repo.On("WithTx", ctx, mock.AnythingOfType("func(rbac.Repository) error")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(rbac.Repository) error)
				// Execute the function passed to WithTx, using the same mock repo
				err := fn(ts.repo)
				require.NoError(t, err)
			}).Return(nil).Once()

		// Set expectations for calls *inside* the transaction
		ts.repo.On("GetRoleBySlug", ctx, "new-role").Return(nil, rbac.ErrRoleNotFound).Once()
		ts.repo.On("CreateRole", ctx, mock.AnythingOfType("*rbac.Role")).Return(nil).Once()

		// Set expectation for audit call *after* the transaction
		ts.audit.On("Record", mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == rbac.EventRoleCreated && e.ActorID == actor.UserID
		})).Return(nil).Once()

		_, err := ts.service.CreateRole(ctx, params)
		require.NoError(t, err)
	})

	t.Run("should fail if role slug already exists", func(t *testing.T) {
		ts := setupAdminService(t)
		existingRole := &rbac.Role{}

		ts.repo.On("WithTx", ctx, mock.Anything).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(rbac.Repository) error)
				err := fn(ts.repo)
				// The inner function should return the specific error
				require.ErrorIs(t, err, rbac.ErrRoleAlreadyExists)
			}).Return(rbac.ErrRoleAlreadyExists).Once()

		// Mock the check inside the transaction
		ts.repo.On("GetRoleBySlug", ctx, "new-role").Return(existingRole, nil).Once()

		_, err := ts.service.CreateRole(ctx, params)
		require.ErrorIs(t, err, rbac.ErrRoleAlreadyExists)
		ts.repo.AssertNotCalled(t, "CreateRole")
	})
}

// TestAdminService_NilSafeDependencies verifies the nil-safe stopgaps:
// mutating operations succeed without a dispatcher or audit recorder.
func TestAdminService_NilSafeDependencies(t *testing.T) {
	ctx, _ := contextWithActor()
	userID, roleID := uuid.New(), uuid.New()

	ts := setupAdminService(t)
	ts.service.SetDispatcher(nil)
	ts.service.SetAuditRecorder(nil)

	ts.repo.On("AssignRoleToUser", ctx, mock.Anything).Return(nil).Once()
	ts.cache.On("Delete", ctx, mock.Anything).Return(nil).Once()

	// No audit/dispatcher mocks are set: the service must not call them
	// (nil-safe) and the operation must still succeed.
	_, err := ts.service.AssignRoleToUser(ctx, userID, roleID)
	require.NoError(t, err)
}
