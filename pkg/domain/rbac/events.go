// internal/domain/rbac/events.go
package rbac

import "github.com/combi/pkg/domain/core"

const (
	EventRoleCreated               core.EventAction = "rbac.role.created"
	EventRoleUpdated               core.EventAction = "rbac.role.updated"
	EventRoleDeleted               core.EventAction = "rbac.role.deleted"
	EventUserRoleAssigned          core.EventAction = "rbac.user_role.assigned"
	EventUserRoleRemoved           core.EventAction = "rbac.user_role.removed"
	EventPermissionGrantedToRole   core.EventAction = "rbac.permission.granted"
	EventPermissionRevokedFromRole core.EventAction = "rbac.permission.revoked"
)
