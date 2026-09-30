// internal/domain/organization/permissions.go
package organization

import "github.com/combi/pkg/domain/core"

const (
	PermOrganizationAdmin core.PermissionName = "org:admin"
	PermOrgUpdate         core.PermissionName = "org:update"
	PermMembersInvite     core.PermissionName = "members:invite"
	PermMembersList       core.PermissionName = "members:list"
	PermMembersRemove     core.PermissionName = "members:remove"
	PermMembersUpdateRole core.PermissionName = "members:update_role"
	PermRolesList         core.PermissionName = "roles:list"
	PermRolesCreate       core.PermissionName = "roles:create"
	PermRolesUpdate       core.PermissionName = "roles:update"
	PermRolesDelete       core.PermissionName = "roles:delete"
)
