// internal/domain/rbac/models.go
package rbac

import (
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// --- Enums ---

type RoleType string

const (
	RoleTypeSystem RoleType = "SYSTEM" // Built-in, immutable via the API.
	RoleTypeCustom RoleType = "CUSTOM" // Created and managed by administrators.
)

type PermissionEffect string

const (
	PermissionEffectAllow PermissionEffect = "ALLOW"
	PermissionEffectDeny  PermissionEffect = "DENY"
)

// --- ABAC (Attribute-Based Access Control) Models ---
// These models extend the core RBAC system to allow for more fine-grained, conditional access.

// Policy represents a conditional rule that grants or denies permissions.
type Policy struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name" gorm:"uniqueIndex"`
	Description string           `json:"description"`
	Effect      PermissionEffect `json:"effect"`               // ALLOW or DENY.
	Actions     []string         `json:"actions"`              // Permission name patterns, e.g., "documents:*"
	Resources   []string         `json:"resources"`            // Resource ARN patterns, e.g., "arn:app:document:123"
	Conditions  json.RawMessage  `json:"conditions,omitempty"` // Stored as JSONB for flexibility
	Priority    int              `json:"priority"`             // Higher priority policies are evaluated first.
	IsActive    bool             `json:"is_active"`
	CreatedBy   *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Permission represents a single, atomic action that can be performed.
type Permission struct {
	ID          uuid.UUID           `json:"id" db:"id"`
	Name        core.PermissionName `json:"name" db:"name"`
	Description string              `json:"description" db:"description"`
	Category    string              `json:"category" db:"category"`
	IsSystem    bool                `json:"is_system" db:"is_system"`
	CreatedAt   time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at" db:"updated_at"`
}

type Role struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description string     `json:"description" db:"description"`
	Type        RoleType   `json:"type" db:"type"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type RoleAssignment struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	RoleID    uuid.UUID  `json:"role_id" db:"role_id"`
	GrantedBy *uuid.UUID `json:"granted_by,omitempty" db:"granted_by"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// IsActive checks if the assignment is currently valid.
func (ra *RoleAssignment) IsActive(role *Role) bool {
	return role != nil && role.IsActive
}

type UserRoleSummary struct {
	UserID     uuid.UUID `json:"user_id" db:"id"`
	FirstName  string    `json:"first_name" db:"first_name"`
	LastName   string    `json:"last_name" db:"last_name"`
	Email      string    `json:"email" db:"email"`
	AssignedAt time.Time `json:"assigned_at" db:"created_at"` // From user_roles table
}
