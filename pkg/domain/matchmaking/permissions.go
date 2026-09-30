package matchmaking

import "github.com/combi/pkg/domain/core"

const (
	// PermMatchRequest allows a user to ask for a mentor.
	PermMatchRequest      core.PermissionName = "match:request"
	PermMatchRequestAdmin core.PermissionName = "match:request:admin"
	// PermMatchManage allows admins to trigger algorithms and manually pair users.
	PermMatchManage core.PermissionName = "match:manage"
)
