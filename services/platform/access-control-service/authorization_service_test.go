// Ported from api/internal/services/rbac/authorization_service_test.go.
package acl

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
)

// authzServiceTestSuite holds all mocked dependencies for the AuthorizationService.
type authzServiceTestSuite struct {
	repo    *rbac_mocks.MockRepository
	cache   *mocks.MockService
	service *AuthorizationService
}

// setupAuthzService initializes a new test suite with fresh mocks.
func setupAuthzService(t *testing.T) authzServiceTestSuite {
	t.Helper()
	var ts authzServiceTestSuite
	ts.repo = rbac_mocks.NewMockRepository(t)
	ts.cache = mocks.NewMockService(t)

	ts.service = NewAuthorizationService(
		ts.repo,
		ts.cache,
		&log.NoOpLogger{},
	)
	return ts
}

func TestAuthorizationService_Authorize(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	permToRead := core.PermissionName("users:read")
	permToWrite := core.PermissionName("users:write")
	userPermissions := []core.PermissionName{permToRead}

	t.Run("should allow access when permission exists in cache", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)
		cachedData, _ := json.Marshal(userPermissions)

		// --- Arrange Mocks ---
		// 1. Cache hit
		ts.cache.On("Get", ctx, cacheKey).Return(cachedData, nil).Once()

		// --- Act ---
		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToRead}
		allowed, err := ts.service.Authorize(ctx, check)

		// --- Assert ---
		require.NoError(t, err)
		assert.True(t, allowed)
		// CRITICAL: The repository should NOT be called on a cache hit.
		ts.repo.AssertNotCalled(t, "GetUserEffectivePermissionNames")
	})

	t.Run("should deny access when permission is not in cache", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)
		cachedData, _ := json.Marshal(userPermissions)

		// --- Arrange Mocks ---
		ts.cache.On("Get", ctx, cacheKey).Return(cachedData, nil).Once()

		// --- Act ---
		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToWrite}
		allowed, err := ts.service.Authorize(ctx, check)

		// --- Assert ---
		require.NoError(t, err)
		assert.False(t, allowed)
		ts.repo.AssertNotCalled(t, "GetUserEffectivePermissionNames")
	})

	t.Run("should fetch from repo and allow access on cache miss", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)

		// --- Arrange Mocks ---
		// 1. Cache miss
		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		// 2. Repo is called
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(userPermissions, nil).Once()
		// 3. Result is stored back in the cache
		ts.cache.On("Set", ctx, cacheKey, userPermissions, permissionsCacheTTL).Return(nil).Once()

		// --- Act ---
		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToRead}
		allowed, err := ts.service.Authorize(ctx, check)

		// --- Assert ---
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should fetch from repo and deny access on cache miss", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)

		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(userPermissions, nil).Once()
		ts.cache.On("Set", ctx, cacheKey, userPermissions, permissionsCacheTTL).Return(nil).Once()

		// --- Act ---
		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToWrite}
		allowed, err := ts.service.Authorize(ctx, check)

		// --- Assert ---
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("should return error if repository fails on cache miss", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)
		repoErr := errors.New("database is down")

		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(nil, repoErr).Once()

		// --- Act ---
		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToRead}
		_, err := ts.service.Authorize(ctx, check)

		// --- Assert ---
		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
		// Cache Set should not be called if the repo fails.
		ts.cache.AssertNotCalled(t, "Set")
	})

	t.Run("should proceed and succeed even if cache Get fails", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)
		cacheErr := errors.New("redis is down")

		// 1. Cache Get fails
		ts.cache.On("Get", ctx, cacheKey).Return(nil, cacheErr).Once()
		// 2. Logic should fall back to repo
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(userPermissions, nil).Once()
		// 3. It should still attempt to set the cache
		ts.cache.On("Set", ctx, cacheKey, userPermissions, permissionsCacheTTL).Return(nil).Once()

		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToRead}
		allowed, err := ts.service.Authorize(ctx, check)

		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should proceed and succeed even if cache Set fails", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)
		cacheErr := errors.New("redis is read-only")

		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(userPermissions, nil).Once()
		// Cache Set fails, but the overall operation should not.
		ts.cache.On("Set", ctx, cacheKey, userPermissions, permissionsCacheTTL).Return(cacheErr).Once()

		check := rbac.PermissionCheck{ActorID: userID, PermissionName: permToRead}
		allowed, err := ts.service.Authorize(ctx, check)

		// The error from cache.Set is logged but not returned, so the authorization succeeds.
		require.NoError(t, err)
		assert.True(t, allowed)
	})
}

func TestAuthorizationService_GetUserEffectivePermissions(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	permNames := []core.PermissionName{"users:read", "posts:create"}
	permDetails := []*rbac.Permission{
		{ID: uuid.New(), Name: "users:read"},
		{ID: uuid.New(), Name: "posts:create"},
	}

	t.Run("should return permission details from repo after getting names from cache/repo", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)

		// --- Arrange Mocks (simulating a cache miss) ---
		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return(permNames, nil).Once()
		ts.cache.On("Set", ctx, cacheKey, permNames, mock.Anything).Return(nil).Once()
		// After getting the names, it calls the repo again to get the full
		// details. Order is unspecified (the names come back through a
		// permission set), so match on set equality instead of slice order.
		ts.repo.On("GetPermissionsByNames", ctx, mock.MatchedBy(func(names []core.PermissionName) bool {
			if len(names) != len(permNames) {
				return false
			}
			want := make(map[core.PermissionName]bool, len(permNames))
			for _, n := range permNames {
				want[n] = true
			}
			for _, n := range names {
				if !want[n] {
					return false
				}
			}
			return true
		})).Return(permDetails, nil).Once()

		// --- Act ---
		permissions, err := ts.service.GetUserEffectivePermissions(ctx, userID)

		// --- Assert ---
		require.NoError(t, err)
		require.Len(t, permissions, 2)
		assert.Equal(t, permDetails[0].ID, permissions[0].ID)
		assert.Equal(t, permDetails[1].ID, permissions[1].ID)
	})

	t.Run("should return an empty slice if user has no permissions", func(t *testing.T) {
		ts := setupAuthzService(t)
		cacheKey := ts.service.buildUserPermissionsCacheKey(userID)

		ts.cache.On("Get", ctx, cacheKey).Return(nil, nil).Once()
		ts.repo.On("GetUserEffectivePermissionNames", ctx, userID).Return([]core.PermissionName{}, nil).Once()
		ts.cache.On("Set", ctx, cacheKey, mock.Anything, mock.Anything).Return(nil).Once()

		permissions, err := ts.service.GetUserEffectivePermissions(ctx, userID)

		require.NoError(t, err)
		assert.Empty(t, permissions)
		// It should not call GetPermissionsByNames if there are no names to fetch.
		ts.repo.AssertNotCalled(t, "GetPermissionsByNames")
	})
}
