// access-control-service (package acl) — ported from
// api/internal/services/rbac/cache_invalidation_worker.go.
//
// CacheInvalidationWorker handles the `rbac:invalidate_user_cache` job
// type: when a role's permission set changes, it clears the cached
// permission sets of every user holding that role.
package acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
)

// CacheInvalidationWorker handles the `rbac:invalidate_user_cache` job type.
type CacheInvalidationWorker struct {
	repo   rbac.Repository
	admin  *AdminService // We need the admin service for its cache-clearing helper
	logger log.Logger
}

// NewCacheInvalidationWorker creates a new worker instance.
func NewCacheInvalidationWorker(repo rbac.Repository, adminService *AdminService, logger log.Logger) *CacheInvalidationWorker {
	return &CacheInvalidationWorker{
		repo:   repo,
		admin:  adminService,
		logger: logger.With(log.String("worker", string(rbac.JobRBACInvalidateUserCache))),
	}
}

func (w *CacheInvalidationWorker) JobType() core.JobType {
	return rbac.JobRBACInvalidateUserCache
}

type invalidatePayload struct {
	RoleID string `json:"role_id"`
}

func (w *CacheInvalidationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p invalidatePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload for rbac cache invalidation: %w", err))
	}

	roleID, err := uuid.Parse(p.RoleID)
	if err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid role_id in payload: %w", err))
	}

	role, err := w.repo.GetRole(ctx, roleID)
	if err != nil {
		// If the role doesn't exist, there's nothing to do.
		if err == rbac.ErrRoleNotFound {
			w.logger.Warn("Cache invalidation job ran for a role that no longer exists.", log.String("role_id", p.RoleID))
			return nil, nil
		}
		// For other DB errors, retry.
		return nil, err
	}

	// Fetch all users in the role. In a system with millions of users, this would
	// need to be paginated. For now, this is sufficient.
	users, err := w.repo.ListUsersByRoleSlug(ctx, role.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to list users for role slug '%s': %w", role.Slug, err)
	}

	if len(users) == 0 {
		w.logger.Info("No users found for role, no caches to invalidate.", log.String("role_slug", role.Slug))
		return nil, nil
	}

	w.logger.Info("Starting cache invalidation for users in role.",
		log.String("role_slug", role.Slug),
		log.Int("user_count", len(users)),
	)

	// Invalidate the cache for each user.
	for _, u := range users {
		w.admin.clearUserPermissions(ctx, u.UserID)
	}

	w.logger.Info("Successfully invalidated caches.",
		log.String("role_slug", role.Slug),
		log.Int("user_count", len(users)),
	)

	result, _ := json.Marshal(map[string]int{"invalidated_user_count": len(users)})
	return result, nil
}
