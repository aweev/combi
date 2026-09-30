// Ported from api/internal/services/rbac/cache_invalidation_worker_test.go.
package acl

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
)

// cacheWorkerTestSuite holds all mocked dependencies for the CacheInvalidationWorker.
type cacheWorkerTestSuite struct {
	repo     *rbac_mocks.MockRepository
	cache    *mocks.MockService
	worker   *CacheInvalidationWorker
	adminSvc *AdminService
}

// setupCacheWorker initializes a new test suite.
func setupCacheWorker(t *testing.T) cacheWorkerTestSuite {
	t.Helper()
	var ts cacheWorkerTestSuite
	ts.repo = rbac_mocks.NewMockRepository(t)
	ts.cache = mocks.NewMockService(t)

	// Create a real AdminService instance but inject our mock dependencies into it.
	// The worker needs this to call the unexported `clearUserPermissions` method.
	ts.adminSvc = NewAdminService(ts.repo, ts.cache, nil, nil, &log.NoOpLogger{})

	ts.worker = NewCacheInvalidationWorker(ts.repo, ts.adminSvc, &log.NoOpLogger{})

	return ts
}

func TestCacheInvalidationWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	roleID := uuid.New()
	role := &rbac.Role{ID: roleID, Slug: "test-role"}

	t.Run("should invalidate caches for all users in a role", func(t *testing.T) {
		ts := setupCacheWorker(t)

		user1ID := uuid.New()
		user2ID := uuid.New()

		users := []rbac.UserRoleSummary{
			{UserID: user1ID},
			{UserID: user2ID},
		}

		payload, _ := json.Marshal(invalidatePayload{RoleID: roleID.String()})
		job := &queue.Job{Payload: payload}

		// --- Arrange Mocks ---
		// 1. Worker gets the role details.
		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		// 2. Worker lists users in that role.
		ts.repo.On("ListUsersByRoleSlug", ctx, role.Slug).Return(users, nil).Once()
		// 3. The cache `Delete` method is called for EACH user.
		ts.cache.On("Delete", ctx, ts.adminSvc.buildUserPermissionsCacheKey(user1ID)).Return(nil).Once()
		ts.cache.On("Delete", ctx, ts.adminSvc.buildUserPermissionsCacheKey(user2ID)).Return(nil).Once()

		// --- Act ---
		result, err := ts.worker.ProcessJob(ctx, job)

		// --- Assert ---
		require.NoError(t, err)
		var resultData map[string]int
		err = json.Unmarshal(result, &resultData)
		require.NoError(t, err)
		assert.Equal(t, 2, resultData["invalidated_user_count"])
	})

	t.Run("should succeed with no error if no users are in the role", func(t *testing.T) {
		ts := setupCacheWorker(t)
		payload, _ := json.Marshal(invalidatePayload{RoleID: roleID.String()})
		job := &queue.Job{Payload: payload}

		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		// Return an empty list of users.
		ts.repo.On("ListUsersByRoleSlug", ctx, role.Slug).Return([]rbac.UserRoleSummary{}, nil).Once()

		result, err := ts.worker.ProcessJob(ctx, job)

		require.NoError(t, err)
		assert.Nil(t, result, "Result should be nil when no users are processed")
		// CRITICAL: Assert that the cache delete method was never called.
		ts.cache.AssertNotCalled(t, "Delete")
	})

	t.Run("should succeed with no error if role is not found", func(t *testing.T) {
		ts := setupCacheWorker(t)
		payload, _ := json.Marshal(invalidatePayload{RoleID: roleID.String()})
		job := &queue.Job{Payload: payload}

		// Simulate the role being deleted between job dispatch and execution.
		ts.repo.On("GetRole", ctx, roleID).Return(nil, rbac.ErrRoleNotFound).Once()

		result, err := ts.worker.ProcessJob(ctx, job)

		require.NoError(t, err)
		assert.Nil(t, result)
		ts.repo.AssertNotCalled(t, "ListUsersByRoleSlug")
		ts.cache.AssertNotCalled(t, "Delete")
	})

	t.Run("should return a retryable error if listing users fails", func(t *testing.T) {
		ts := setupCacheWorker(t)
		payload, _ := json.Marshal(invalidatePayload{RoleID: roleID.String()})
		job := &queue.Job{Payload: payload}
		repoErr := errors.New("database connection lost")

		ts.repo.On("GetRole", ctx, roleID).Return(role, nil).Once()
		ts.repo.On("ListUsersByRoleSlug", ctx, role.Slug).Return(nil, repoErr).Once()

		_, err := ts.worker.ProcessJob(ctx, job)

		require.Error(t, err)
		// This should be a generic, retryable error, not a NonRetryableError.
		var nonRetryable *queue.NonRetryableError
		assert.False(t, errors.As(err, &nonRetryable))
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("should return a NonRetryableError for invalid payload", func(t *testing.T) {
		ts := setupCacheWorker(t)

		// Invalid JSON
		job := &queue.Job{Payload: json.RawMessage(`{"role_id":123}`)} // role_id is not a string

		_, err := ts.worker.ProcessJob(ctx, job)

		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		require.True(t, errors.As(err, &nonRetryable), "error should be NonRetryableError")
	})

	t.Run("should return a NonRetryableError for invalid role_id UUID", func(t *testing.T) {
		ts := setupCacheWorker(t)
		payload, _ := json.Marshal(invalidatePayload{RoleID: "not-a-uuid"})
		job := &queue.Job{Payload: payload}

		_, err := ts.worker.ProcessJob(ctx, job)

		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		require.True(t, errors.As(err, &nonRetryable), "error should be NonRetryableError")
	})
}
