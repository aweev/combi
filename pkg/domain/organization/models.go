package organization

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/user"
	"github.com/google/uuid"
)

// --- Enums ---

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "PENDING"
	InvitationStatusAccepted InvitationStatus = "ACCEPTED"
	InvitationStatusDeclined InvitationStatus = "DECLINED"
	InvitationStatusExpired  InvitationStatus = "EXPIRED"
	InvitationStatusCanceled InvitationStatus = "CANCELED"
)

type OrganizationType string

const (
	// Add more types as your business logic requires
	TypeCompany  OrganizationType = "COMPANY"
	TypePartner  OrganizationType = "PARTNER"
	TypeInternal OrganizationType = "INTERNAL"
)

type OrganizationStatus string

const (
	StatusActive   OrganizationStatus = "ACTIVE"
	StatusInactive OrganizationStatus = "INACTIVE"
	StatusPending  OrganizationStatus = "PENDING"
)

// --- Core Models ---

type Organization struct {
	ID        uuid.UUID          `json:"id" db:"id"`
	OwnerID   uuid.UUID          `json:"owner_id" db:"owner_id"`
	Name      string             `json:"name" db:"name"`
	Slug      string             `json:"slug" db:"slug"`
	Type      OrganizationType   `json:"type" db:"type"`
	Status    OrganizationStatus `json:"status" db:"status"`
	AvatarID  *uuid.UUID         `json:"avatar_id,omitempty" db:"avatar_id"`
	Version   int                `json:"-" db:"version"`
	CreatedAt time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" db:"updated_at"`
}

type OrganizationDomain struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	OrganizationID uuid.UUID  `json:"organization_id" db:"organization_id"`
	Domain         string     `json:"domain" db:"domain"`
	IsPrimary      bool       `json:"is_primary" db:"is_primary"`
	VerifiedAt     *time.Time `json:"verified_at,omitempty" db:"verified_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type Role struct {
	ID             uuid.UUID `json:"id" db:"id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	Name           string    `json:"name" db:"name"`
	Description    string    `json:"description" db:"description"`
	IsSystemRole   bool      `json:"is_system_role" db:"is_system_role"`
	Permissions    []string  `json:"permissions" db:"permissions"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type Member struct {
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	RoleID         uuid.UUID `json:"role_id" db:"role_id"`
	JoinedAt       time.Time `json:"joined_at" db:"joined_at"`
}

type Invitation struct {
	ID             uuid.UUID        `json:"id" db:"id"`
	OrganizationID uuid.UUID        `json:"organization_id" db:"organization_id"`
	Email          string           `json:"email" db:"email"`
	RoleID         uuid.UUID        `json:"role_id" db:"role_id"`
	InvitedBy      uuid.UUID        `json:"invited_by" db:"invited_by"`
	AcceptedBy     *uuid.UUID       `json:"accepted_by,omitempty" db:"accepted_by"`
	Status         InvitationStatus `json:"status" db:"status"`
	TokenHash      string           `json:"-" db:"token_hash"`
	ExpiresAt      time.Time        `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at" db:"updated_at"`
}

// --- View Models / DTOs for complex queries ---

// MemberDetails is a rich DTO combining data from multiple tables.
type MemberDetails struct {
	// Member Info
	UserID   uuid.UUID `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
	// User Info (Composed from User domain)
	User *user.User `json:"user"`
	// Role Info
	Role *Role `json:"role"`
}

// InvitationDetails is a rich DTO for API responses.
type InvitationDetails struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Email          string           `json:"email"`
	Status         InvitationStatus `json:"status"`
	ExpiresAt      time.Time        `json:"expires_at"`
	// Invited By (User Info)
	Inviter *user.User `json:"inviter"`
	// Role to be granted
	Role         *Role         `json:"role"`
	Organization *Organization `json:"organization"`
}

// --- Org Context (for middleware) ---

// OrgContext represents the actor's specific context within an organization.
type OrgContext struct {
	OrgID       uuid.UUID
	Member      *Member
	Role        *Role
	Permissions map[string]bool // A set of permission names for fast lookups
}

type contextKey string

const orgContextKey = contextKey("org_context")

func NewContextWithOrg(ctx context.Context, orgCtx *OrgContext) context.Context {
	return context.WithValue(ctx, orgContextKey, orgCtx)
}
func OrgFromContext(ctx context.Context) *OrgContext {
	orgCtx, _ := ctx.Value(orgContextKey).(*OrgContext)
	return orgCtx
}
