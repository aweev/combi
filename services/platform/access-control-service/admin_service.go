// access-control-service (package acl) — ported from
// api/internal/services/rbac/admin_service.go.
//
// AdminService provides the administrative and operational contract for
// managing the RBAC system: roles, role<->permission mappings, and
// user<->role assignments, with audit recording and cache invalidation.
package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
)

// Statically verify that it implements the domain interface.
var _ rbac.AdminService = (*AdminService)(nil)

// AdminService provides the administrative and operational contract for managing the RBAC system.
type AdminService struct {
	repo       rbac.Repository
	cache      cache.Service
	dispatcher queue.Dispatcher    // nil until a queue host is mounted; nil-safe
	audit      audit.EventRecorder // nil until the audit service is ported; nil-safe
	logger     log.Logger
}

// NewAdminService creates a new RBAC administration service.
//
// dispatcher and auditRecorder may be nil: the chassis host does not yet
// provide a queue dispatcher or audit recorder. Mutating operations still
// succeed; cache invalidation and audit trail are skipped (logged at debug)
// until the dependency is late-bound via SetDispatcher / SetAuditRecorder.
func NewAdminService(
	repo rbac.Repository,
	cacheSvc cache.Service,
	dispatcher queue.Dispatcher,
	auditRecorder audit.EventRecorder,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:       repo,
		cache:      cacheSvc,
		dispatcher: dispatcher,
		audit:      auditRecorder,
		logger:     logger.With(log.String("component", "rbac_admin_service")),
	}
}

// SetDispatcher late-binds the queue dispatcher (e.g. when the Temporal or
// queue host lands).
func (s *AdminService) SetDispatcher(d queue.Dispatcher) {
	s.dispatcher = d
}

// SetAuditRecorder late-binds the audit event recorder (e.g. when the
// audit-log-service module is mounted in the same host).
func (s *AdminService) SetAuditRecorder(rec audit.EventRecorder) {
	s.audit = rec
}

// record emits the audit event if a recorder is bound; failures are logged,
// never propagated — an audit outage must not fail the business operation.
func (s *AdminService) record(ctx context.Context, event audit.Event) {
	if s.audit == nil {
		s.logger.Debug("audit recorder not bound; skipping audit event",
			log.String("action", string(event.Action)))
		return
	}
	if err := s.audit.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *AdminService) buildUserPermissionsCacheKey(userID uuid.UUID) string {
	return fmt.Sprintf("rbac:user:%s:permissions", userID.String())
}

// clearUserPermissions invalidates a single user's cached permission set.
func (s *AdminService) clearUserPermissions(ctx context.Context, userID uuid.UUID) {
	cacheKey := s.buildUserPermissionsCacheKey(userID)
	if err := s.cache.Delete(ctx, cacheKey); err != nil {
		s.logger.Error("Failed to clear user permission cache", err, log.String("user_id", userID.String()))
	}
}

// dispatchCacheInvalidationJob creates a background job to invalidate caches for all users in a given role.
func (s *AdminService) dispatchCacheInvalidationJob(ctx context.Context, roleID uuid.UUID) {
	if s.dispatcher == nil {
		// No queue host mounted yet: fall back to a debug note. Cache
		// entries expire by TTL (15 min), so staleness is bounded.
		s.logger.Debug("dispatcher not bound; skipping RBAC cache invalidation job",
			log.String("role_id", roleID.String()))
		return
	}

	payload, _ := json.Marshal(map[string]string{"role_id": roleID.String()})

	// Dispatch a low-priority job to handle the cache invalidation asynchronously.
	_, err := s.dispatcher.Dispatch(ctx, rbac.JobRBACInvalidateUserCache, payload,
		queue.WithQueue("low"),
		queue.WithUniqueKey(fmt.Sprintf("invalidate-rbac-role-%s", roleID.String()), 1*time.Minute), // Debounce
	)
	if err != nil {
		s.logger.Error("Failed to dispatch RBAC cache invalidation job", err, log.String("role_id", roleID.String()))
	}
}

// --- User <-> Role Management ---

func (s *AdminService) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) (*rbac.RoleAssignment, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	assignment := &rbac.RoleAssignment{
		ID:        uuid.New(),
		UserID:    userID,
		RoleID:    roleID,
		GrantedBy: &actor.AccountID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.AssignRoleToUser(ctx, assignment); err != nil {
		return nil, err
	}

	s.clearUserPermissions(ctx, userID)

	s.record(ctx, audit.NewEvent(actor.AccountID, "user", rbac.EventUserRoleAssigned).
		WithSuccess().
		WithTarget(userID, "user", "").
		WithMetadata(map[string]interface{}{"role_id": roleID}).
		WithContext(ctx).
		Build())

	return assignment, nil
}

func (s *AdminService) RevokeRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	if err := s.repo.RevokeRoleFromUser(ctx, userID, roleID); err != nil {
		return err
	}

	s.clearUserPermissions(ctx, userID)

	s.record(ctx, audit.NewEvent(actor.AccountID, "user", rbac.EventUserRoleRemoved).
		WithSuccess().
		WithTarget(userID, "user", "").
		WithMetadata(map[string]interface{}{"role_id": roleID}).
		WithContext(ctx).
		Build())

	return nil
}

// --- Role Management ---

func (s *AdminService) CreateRole(ctx context.Context, params rbac.CreateRoleParams) (*rbac.Role, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	roleSlug := slug.Make(params.Slug)
	if roleSlug == "" {
		roleSlug = slug.Make(params.Name)
	}

	// Check for existing role within a transaction for consistency.
	role := &rbac.Role{
		ID:          uuid.New(),
		Name:        params.Name,
		Slug:        roleSlug,
		Description: params.Description,
		Type:        rbac.RoleTypeCustom,
		IsActive:    true,
		CreatedBy:   &actor.UserID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	err := s.repo.WithTx(ctx, func(txRepo rbac.Repository) error {
		existing, _ := txRepo.GetRoleBySlug(ctx, roleSlug)
		if existing != nil {
			return rbac.ErrRoleAlreadyExists
		}
		if err := txRepo.CreateRole(ctx, role); err != nil {
			return err
		}
		if len(params.PermissionIDs) > 0 {
			if err := txRepo.AddPermissionsToRole(ctx, role.ID, params.PermissionIDs); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to create role in transaction", err)
		return nil, err
	}

	s.record(ctx, audit.NewEvent(actor.UserID, "user", rbac.EventRoleCreated).
		WithSuccess().
		WithTarget(role.ID, "role", role.Name).
		WithContext(ctx).
		Build())

	return role, nil
}

func (s *AdminService) GetRole(ctx context.Context, roleID uuid.UUID) (*rbac.Role, error) {
	return s.repo.GetRole(ctx, roleID)
}

func (s *AdminService) GetRoleBySlug(ctx context.Context, slug string) (*rbac.Role, error) {
	return s.repo.GetRoleBySlug(ctx, slug)
}

func (s *AdminService) ListRoles(ctx context.Context, filters rbac.RoleFilters) ([]*rbac.Role, int, error) {
	return s.repo.ListRoles(ctx, filters)
}

func (s *AdminService) UpdateRole(ctx context.Context, roleID uuid.UUID, params rbac.UpdateRoleParams) (*rbac.Role, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, err
	}

	if role.Type == rbac.RoleTypeSystem {
		return nil, rbac.ErrSystemRoleImmutable
	}

	// Track if a change occurred that requires cache invalidation
	invalidationNeeded := false
	if params.IsActive != nil && role.IsActive != *params.IsActive {
		role.IsActive = *params.IsActive
		invalidationNeeded = true
	}
	if params.Name != nil {
		role.Name = *params.Name
	}
	if params.Description != nil {
		role.Description = *params.Description
	}
	role.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateRole(ctx, role); err != nil {
		return nil, err
	}

	if invalidationNeeded {
		s.logger.Info("Role active status changed, dispatching cache invalidation job.", log.String("role_id", roleID.String()))
		s.dispatchCacheInvalidationJob(ctx, roleID)
	}

	s.record(ctx, audit.NewEvent(actor.UserID, "user", rbac.EventRoleUpdated).
		WithSuccess().
		WithTarget(role.ID, "role", role.Name).
		WithContext(ctx).
		Build())

	return role, nil
}

func (s *AdminService) DeleteRole(ctx context.Context, roleID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		return err
	}

	if role.Type == rbac.RoleTypeSystem {
		return rbac.ErrSystemRoleImmutable
	}

	count, err := s.repo.CountUsersInRole(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return rbac.ErrRoleInUse
	}

	if err := s.repo.DeleteRole(ctx, roleID); err != nil {
		return err
	}

	s.record(ctx, audit.NewEvent(actor.UserID, "user", rbac.EventRoleDeleted).
		WithSuccess().
		WithTarget(role.ID, "role", role.Name).
		WithContext(ctx).
		Build())

	return nil
}

func (s *AdminService) AddPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	if err := s.repo.AddPermissionsToRole(ctx, roleID, permissionIDs); err != nil {
		return err
	}

	s.logger.Info("Permissions added to role, dispatching cache invalidation job.", log.String("role_id", roleID.String()))
	s.dispatchCacheInvalidationJob(ctx, roleID)

	s.record(ctx, audit.NewEvent(actor.AccountID, "user", rbac.EventPermissionGrantedToRole).
		WithSuccess().
		WithTarget(roleID, "role", "").
		WithMetadata(map[string]interface{}{"permission_ids": permissionIDs}).
		WithContext(ctx).
		Build())

	return nil
}

func (s *AdminService) RemovePermissionsFromRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	if err := s.repo.RemovePermissionsFromRole(ctx, roleID, permissionIDs); err != nil {
		return err
	}

	s.logger.Info("Permissions removed from role, dispatching cache invalidation job.", log.String("role_id", roleID.String()))
	s.dispatchCacheInvalidationJob(ctx, roleID)

	s.record(ctx, audit.NewEvent(actor.AccountID, "user", rbac.EventPermissionRevokedFromRole).
		WithSuccess().
		WithTarget(roleID, "role", "").
		WithMetadata(map[string]interface{}{"permission_ids": permissionIDs}).
		WithContext(ctx).
		Build())

	return nil
}

func (s *AdminService) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*rbac.Permission, error) {
	permNames, err := s.repo.GetRolePermissionNames(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if len(permNames) == 0 {
		return []*rbac.Permission{}, nil
	}
	return s.repo.GetPermissionsByNames(ctx, permNames)
}

func (s *AdminService) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*rbac.Role, error) {
	return s.repo.GetUserRoles(ctx, userID)
}

func (s *AdminService) ListUsersInRole(ctx context.Context, roleSlug string) ([]rbac.UserRoleSummary, error) {
	// The complex query logic is now correctly encapsulated in the repository.
	return s.repo.ListUsersByRoleSlug(ctx, roleSlug)
}

// --- Permission Discovery ---

func (s *AdminService) ListPermissions(ctx context.Context, filters rbac.PermissionFilters) ([]*rbac.Permission, int, error) {
	return s.repo.ListPermissions(ctx, filters)
}
