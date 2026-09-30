package organization

import (
	"context"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// Service defines the primary contract for managing organizations and their members.
type Service interface {
	// Organization Management
	CreateOrganization(ctx context.Context, params CreateOrganizationParams) (*Organization, error)
	GetOrganization(ctx context.Context, orgID uuid.UUID) (*Organization, error)
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]*Organization, error)
	UpdateOrganization(ctx context.Context, orgID uuid.UUID, params UpdateOrganizationParams) (*Organization, error)
	DeleteOrganization(ctx context.Context, orgID uuid.UUID) error
	TransferOwnership(ctx context.Context, orgID, newOwnerUserID uuid.UUID) error

	// Member Management
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*MemberDetails, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberDetails, error)
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	UpdateMemberRole(ctx context.Context, orgID, userID, roleID uuid.UUID) (*Member, error)
	LeaveOrganization(ctx context.Context, orgID uuid.UUID) error

	// Invitation Management
	InviteMember(ctx context.Context, orgID uuid.UUID, params InviteMemberParams) (*Invitation, error)
	ListPendingInvitations(ctx context.Context, orgID uuid.UUID) ([]*InvitationDetails, error)
	CancelInvitation(ctx context.Context, orgID, invitationID uuid.UUID) error
	GetInvitationDetailsByToken(ctx context.Context, token string) (*InvitationDetails, error)

	// User-facing acceptance (requires token validation)
	AcceptInvitation(ctx context.Context, token string, acceptingUserID uuid.UUID) error
	// System-facing acceptance (trusted, skips token check) - NEW
	AcceptInvitationInternal(ctx context.Context, invitationID, acceptingUserID uuid.UUID) error

	DeclineInvitation(ctx context.Context, token string) error

	// Domain Management
	AddDomain(ctx context.Context, orgID uuid.UUID, params AddDomainParams) (*OrganizationDomain, error)
	ListDomains(ctx context.Context, orgID uuid.UUID) ([]*OrganizationDomain, error)
	RemoveDomain(ctx context.Context, orgID, domainID uuid.UUID) error
	SetPrimaryDomain(ctx context.Context, orgID, domainID uuid.UUID) error
	VerifyDomain(ctx context.Context, orgID, domainID uuid.UUID) error
	ResolveOrganizationByDomain(ctx context.Context, domain string) (*Organization, error)
}

// RoleAdminService defines the contract for managing roles.
type RoleAdminService interface {
	ListRoles(ctx context.Context, orgID uuid.UUID) ([]*Role, error)
	CreateRole(ctx context.Context, orgID uuid.UUID, params CreateRoleParams) (*Role, error)
	UpdateRole(ctx context.Context, orgID, roleID uuid.UUID, params UpdateRoleParams) (*Role, error)
	DeleteRole(ctx context.Context, orgID, roleID uuid.UUID) error
}

// --- DTOs for Service Methods ---

type CreateOrganizationParams struct {
	OwnerUserID uuid.UUID        `validate:"required"`
	Name        string           `validate:"required,min=2"`
	Slug        string           `validate:"required,slug"`
	Type        OrganizationType `validate:"required"`
	Status      OrganizationStatus
}

// --- DTOs for Service Methods ---

type UpdateOrganizationParams struct {
	Name     *string `validate:"omitempty,min=2"`
	Slug     *string `validate:"omitempty,slug"`
	Status   *OrganizationStatus
	AvatarID *uuid.UUID
}

type InviteMemberParams struct {
	Email  core.Email `validate:"required,email"`
	RoleID uuid.UUID  `validate:"required"`
}

type CreateRoleParams struct {
	Name        string   `validate:"required,min=2"`
	Description string   `validate:"omitempty"`
	Permissions []string `validate:"required,min=1"`
}

type UpdateRoleParams struct {
	Name        *string  `validate:"omitempty,min=2"`
	Description *string  `validate:"omitempty"`
	Permissions []string `validate:"omitempty,min=1"`
}

type AddDomainParams struct {
	Domain    string `validate:"required,hostname"`
	IsPrimary bool
}
