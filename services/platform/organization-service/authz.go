package organization

import (
	"context"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/organization"
)

// AuthorizerRepo defines the subset of repository methods needed by the Authorizer.
type AuthorizerRepo interface {
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organization.Member, error)
	GetRoleByID(ctx context.Context, id uuid.UUID) (*organization.Role, error)
}

// Authorizer centralizes permission checks for the organization domain.
type Authorizer struct {
	repo AuthorizerRepo
}

func NewAuthorizer(repo AuthorizerRepo) *Authorizer {
	return &Authorizer{repo: repo}
}

// Can performs a permission check for a given actor within an organization context.
func (a *Authorizer) Can(ctx context.Context, actor *auth.Actor, orgID uuid.UUID, permission core.PermissionName) (bool, error) {
	if actor == nil {
		return false, auth.ErrUnauthorized
	}

	// 1. Check if the user is a member of the organization.
	member, err := a.repo.GetMember(ctx, orgID, actor.UserID)
	if err != nil {
		return false, organization.ErrNotOrganizationMember.WithUnderlying(err)
	}

	// 2. Get the user's role within this specific organization.
	role, err := a.repo.GetRoleByID(ctx, member.RoleID)
	if err != nil {
		return false, err
	}

	// 3. (CORRECTED) The `IsActive` check is removed as it's not on this model.
	// The core logic is just checking the permission list.

	// 4. Check if the role's permissions list contains the required permission
	//    OR the 'org:admin' super-permission.
	for _, p := range role.Permissions {
		if p == string(permission) || p == string(organization.PermOrganizationAdmin) {
			return true, nil
		}
	}

	// 5. If no specific permission is found, deny access.
	return false, nil
}
