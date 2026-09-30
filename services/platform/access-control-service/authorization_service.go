// access-control-service (package acl) — ported from
// api/internal/services/rbac/authorization_service.go.
//
// The AuthorizationService is the high-performance, read-only side of RBAC:
// permission checks with a cache-aside strategy on top of the shared
// pkg/cache service.
package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
)

const (
	userPermissionsCachePrefix = "rbac::user_perms::"
	userRolesCachePrefix       = "rbac::user_roles::"
)

var _ rbac.AuthorizationService = (*AuthorizationService)(nil)

const permissionsCacheTTL = 15 * time.Minute

// AuthorizationService is the high-performance, read-only service for permission checks.
// It utilizes caching to minimize database load.
type AuthorizationService struct {
	repo      rbac.Repository
	cache     cache.Service
	evaluator *PolicyEvaluator
	logger    log.Logger
}

func NewAuthorizationService(repo rbac.Repository, cacheSvc cache.Service, logger log.Logger) *AuthorizationService {
	return &AuthorizationService{
		repo:      repo,
		cache:     cacheSvc,
		evaluator: NewPolicyEvaluator(repo), // ABAC evaluator
		logger:    logger.With(log.String("component", "authorization_service")),
	}
}

// buildUserPermissionsCacheKey is a private helper to ensure cache keys are consistent.
func (s *AuthorizationService) buildUserPermissionsCacheKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s%s", userPermissionsCachePrefix, userID.String())
}

// getUserPermissionsSet retrieves a user's effective permissions, using a cache-aside strategy.
func (s *AuthorizationService) getUserPermissionsSet(ctx context.Context, userID uuid.UUID) (map[core.PermissionName]bool, error) {
	key := s.buildUserPermissionsCacheKey(userID)

	// 1. Check Cache First
	cachedData, err := s.cache.Get(ctx, key)
	if err == nil && cachedData != nil {
		var permissions []core.PermissionName
		if json.Unmarshal(cachedData, &permissions) == nil {
			s.logger.Debug("Cache hit for user permissions", log.String("user_id", userID.String()))
			permSet := make(map[core.PermissionName]bool, len(permissions))
			for _, p := range permissions {
				permSet[p] = true
			}
			return permSet, nil
		}
	}

	// 2. Cache Miss or Error: Fetch from Repository
	s.logger.Debug("Cache miss for user permissions, fetching from repository", log.String("user_id", userID.String()))
	permissions, err := s.repo.GetUserEffectivePermissionNames(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective permissions from repo: %w", err)
	}

	// 3. Store in Cache for next time
	if err := s.cache.Set(ctx, key, permissions, permissionsCacheTTL); err != nil {
		// Log but do not fail the operation. The check will still succeed.
		s.logger.Error("Failed to set user permissions in cache", err, log.String("user_id", userID.String()))
	}

	permSet := make(map[core.PermissionName]bool, len(permissions))
	for _, p := range permissions {
		permSet[p] = true
	}
	return permSet, nil
}

// Authorize performs the main RBAC and ABAC check.
func (s *AuthorizationService) Authorize(ctx context.Context, check rbac.PermissionCheck) (bool, error) {
	// 1. Get the user's full permission set (from cache or DB).
	permissionSet, err := s.getUserPermissionsSet(ctx, check.ActorID)
	if err != nil {
		s.logger.Error("Failed to get user permissions for authorization check", err, log.String("user_id", check.ActorID.String()))
		return false, err // Propagate DB errors
	}

	// 2. Perform the basic RBAC check (is the permission in the set?).
	if _, ok := permissionSet[check.PermissionName]; !ok {
		return false, nil // RBAC check failed.
	}

	// 3. If RBAC passes, perform the fine-grained ABAC check.
	return s.evaluator.Evaluate(ctx, check)
}

// GetUserEffectivePermissions is used for discovery (e.g., to show a user what they can do).
func (s *AuthorizationService) GetUserEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]*rbac.Permission, error) {
	permissionSet, err := s.getUserPermissionsSet(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(permissionSet) == 0 {
		return []*rbac.Permission{}, nil
	}

	names := make([]core.PermissionName, 0, len(permissionSet))
	for name := range permissionSet {
		names = append(names, name)
	}
	// Deterministic order: the set is built from a map, whose iteration
	// order is randomized — sorting keeps the repo call (and its mocks)
	// stable across runs.
	slices.Sort(names)

	// This still hits the DB, but only to get details (description, category), not for the complex role join.
	return s.repo.GetPermissionsByNames(ctx, names)
}

func (s *AuthorizationService) buildUserRolesCacheKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s%s", userRolesCachePrefix, userID.String())
}

// GetUserRoles returns the roles assigned to a user, cache-aside.
func (s *AuthorizationService) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*rbac.Role, error) {
	key := s.buildUserRolesCacheKey(userID)

	// 1. Check Cache
	cachedData, err := s.cache.Get(ctx, key)
	if err == nil && cachedData != nil {
		var roles []*rbac.Role
		if json.Unmarshal(cachedData, &roles) == nil {
			return roles, nil
		}
	}

	// 2. Cache Miss: Fetch from Repository
	roles, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	// 3. Store in Cache (15 min TTL)
	if err := s.cache.Set(ctx, key, roles, permissionsCacheTTL); err != nil {
		s.logger.Error("Failed to cache user roles", err, log.String("user_id", userID.String()))
	}

	return roles, nil
}
