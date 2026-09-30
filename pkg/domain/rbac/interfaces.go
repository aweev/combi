// internal/domain/rbac/interfaces.go
package rbac

import (
	"context"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// AuthorizationService is the primary "read" interface for performing permission checks.
// This is the main port used by application middleware and business logic to enforce access control.
type AuthorizationService interface {
	Authorize(ctx context.Context, check PermissionCheck) (bool, error)
	GetUserEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]*Permission, error)
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*Role, error)
}

// AdminService defines the administrative and operational contract for managing the RBAC system.
// This is used by UIs, CLIs, and other management tools.
type AdminService interface {
	// Role Management
	CreateRole(ctx context.Context, params CreateRoleParams) (*Role, error)
	GetRole(ctx context.Context, roleID uuid.UUID) (*Role, error)
	GetRoleBySlug(ctx context.Context, slug string) (*Role, error)
	ListRoles(ctx context.Context, filters RoleFilters) ([]*Role, int, error)
	UpdateRole(ctx context.Context, roleID uuid.UUID, params UpdateRoleParams) (*Role, error)
	DeleteRole(ctx context.Context, roleID uuid.UUID) error

	// Role<->Permission Management
	AddPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	RemovePermissionsFromRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]*Permission, error)

	// User<->Role Management
	AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) (*RoleAssignment, error)
	RevokeRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*Role, error)
	ListUsersInRole(ctx context.Context, roleSlug string) ([]UserRoleSummary, error)

	// Permission Discovery
	ListPermissions(ctx context.Context, filters PermissionFilters) ([]*Permission, int, error)
}

// --- System Components & Initialization ---

// PermissionRegistrar is an interface for a service that collects all permission
// definitions from across the application at startup and syncs them with the database.
type PermissionRegistrar interface {
	Register(def PermissionDefinition)
	SyncWithDataStore(ctx context.Context) error
	GetDefinitions() []PermissionDefinition
}

// --- DTOs for Service Methods ---

type CreateRoleParams struct {
	Name          string      `json:"name" validate:"required,min=3,max=100"`
	Slug          string      `json:"slug" validate:"required,slug,min=3,max=100"`
	Description   string      `json:"description" validate:"max=500"`
	PermissionIDs []uuid.UUID `json:"permission_ids,omitempty"`
}

type UpdateRoleParams struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=500"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type RoleFilters struct {
	Type   *RoleType `json:"type,omitempty"`
	Search string    `json:"search,omitempty"`
	Limit  int       `json:"limit" validate:"min=1,max=100"`
	Offset int       `json:"offset" validate:"min=0"`
}

type PermissionFilters struct {
	Category *string `json:"category,omitempty"`
	Search   string  `json:"search,omitempty"`
	Limit    int     `json:"limit" validate:"min=1,max=100"`
	Offset   int     `json:"offset" validate:"min=0"`
}

// PermissionDefinition is a DTO used to register a permission's metadata.
// This is the "source of truth" defined in application code.
type PermissionDefinition struct {
	Name        core.PermissionName
	Description string
	Category    string // For grouping in UIs, e.g., "Financial", "User Management"
}

// PermissionCheck represents a single authorization request to the AuthorizationService.
type PermissionCheck struct {
	ActorID         uuid.UUID
	PermissionName  core.PermissionName
	ResourceContext map[string]any // Optional context for ABAC policy evaluation (e.g., resource owner ID)
}
