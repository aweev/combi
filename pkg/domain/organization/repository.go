package organization

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	OrganizationRepository
	MemberRepository
	RoleRepository
	InvitationRepository
	DomainRepository
	AuthorizerRepo
}

type OrganizationRepository interface {
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error)
	GetOrganizationByDomain(ctx context.Context, domain string) (*Organization, error)
	UpdateOrganization(ctx context.Context, org *Organization) error
	DeleteOrganization(ctx context.Context, id uuid.UUID) error
	ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]*Organization, error)
}

type DomainRepository interface {
	CreateDomain(ctx context.Context, domain *OrganizationDomain) error
	GetDomain(ctx context.Context, id uuid.UUID) (*OrganizationDomain, error)
	ListDomains(ctx context.Context, orgID uuid.UUID) ([]*OrganizationDomain, error)
	UpdateDomain(ctx context.Context, domain *OrganizationDomain) error
	DeleteDomain(ctx context.Context, id uuid.UUID) error
	GetPrimaryDomain(ctx context.Context, orgID uuid.UUID) (*OrganizationDomain, error)
}

type MemberRepository interface {
	CreateMember(ctx context.Context, member *Member) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error)
	GetMemberWithDetails(ctx context.Context, orgID, userID uuid.UUID) (*MemberDetails, error)
	ListMemberWithDetails(ctx context.Context, orgID uuid.UUID) ([]*MemberDetails, error)
	UpdateMember(ctx context.Context, member *Member) error
	DeleteMember(ctx context.Context, orgID, userID uuid.UUID) error
	CountMembers(ctx context.Context, orgID uuid.UUID) (int, error)
	CountMembersOwners(ctx context.Context, orgID uuid.UUID) (int, error)
}

type RoleRepository interface {
	CreateRole(ctx context.Context, role *Role) error
	GetRoleByID(ctx context.Context, id uuid.UUID) (*Role, error)
	GetRoleByName(ctx context.Context, orgID uuid.UUID, name string) (*Role, error)
	GetSystemRole(ctx context.Context, orgID uuid.UUID, name string) (*Role, error) // e.g., "Owner"
	ListRole(ctx context.Context, orgID uuid.UUID) ([]*Role, error)
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, id uuid.UUID) error
}

type InvitationRepository interface {
	CreateInvitation(ctx context.Context, inv *Invitation) error
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*Invitation, error)
	GetPendingInvitationsByEmail(ctx context.Context, orgID uuid.UUID, email string) (*Invitation, error)
	ListPendingInvitationsWithDetails(ctx context.Context, orgID uuid.UUID) ([]*InvitationDetails, error)
	UpdateInvitation(ctx context.Context, inv *Invitation) error

	// Fetches an invitation with all related details (organization, inviter, role).
	GetInvitationWithDetails(ctx context.Context, id uuid.UUID) (*InvitationDetails, error)
	// Finds all PENDING invitations for a given email address across all organizations.
	ListPendingInvitationsByEmail(ctx context.Context, email string) ([]*Invitation, error)
	// Atomically updates the status of expired invitations from PENDING to EXPIRED.
	ExpirePendingInvitations(ctx context.Context, now time.Time) (int64, error)
}

type AuthorizerRepo interface {
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error)
	GetRoleByID(ctx context.Context, id uuid.UUID) (*Role, error)
}
