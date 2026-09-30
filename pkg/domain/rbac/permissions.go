// internal/domain/rbac/permissions.go
package rbac

import "github.com/combi/pkg/domain/core"

// These are permissions required to manage the RBAC system itself.
const (
	PermRBACRoleRead       core.PermissionName = "rbac:role:read"
	PermRBACRoleManage     core.PermissionName = "rbac:role:manage"
	PermRBACUserRoleAssign core.PermissionName = "rbac:user_role:assign"
)
