// internal/domain/rbac/repository.go
package rbac

import (
	"context"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// Repository is the master persistence interface for the RBAC domain.
// It composes smaller, more focused repository interfaces.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	PermissionRepository
	RoleRepository
	RoleAssignmentRepository
}

type PermissionRepository interface {
	UpsertPermissions(ctx context.Context, permissions []*Permission) error
	GetPermissionByName(ctx context.Context, name core.PermissionName) (*Permission, error)
	ListPermissions(ctx context.Context, filters PermissionFilters) ([]*Permission, int, error)
	GetPermissionsByNames(ctx context.Context, names []core.PermissionName) ([]*Permission, error)
	UpdatePermission(ctx context.Context, perm *Permission) error
}

type RoleRepository interface {
	CreateRole(ctx context.Context, role *Role) error
	GetRole(ctx context.Context, id uuid.UUID) (*Role, error)
	GetRoleBySlug(ctx context.Context, slug string) (*Role, error)
	ListRoles(ctx context.Context, filters RoleFilters) ([]*Role, int, error)
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Role<->Permission Mappings
	AddPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	RemovePermissionsFromRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	GetRolePermissionNames(ctx context.Context, roleID uuid.UUID) ([]core.PermissionName, error)
}

type RoleAssignmentRepository interface {
	AssignRoleToUser(ctx context.Context, assignment *RoleAssignment) error
	RevokeRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error
	GetUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*Role, error)
	CountUsersInRole(ctx context.Context, roleID uuid.UUID) (int, error)
	ListUsersByRoleSlug(ctx context.Context, roleSlug string) ([]UserRoleSummary, error)
	// Complex Queries
	GetUserEffectivePermissionNames(ctx context.Context, userID uuid.UUID) ([]core.PermissionName, error)
}
