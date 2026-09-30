package auth

import "github.com/combi/pkg/domain/core"

const (
	// PermAuthAccountManage allows suspending, unsuspending, and managing accounts.
	PermAuthAccountManage core.PermissionName = "auth:account:manage"
	// PermAuthSessionRead allows viewing active sessions for any user.
	PermAuthSessionRead core.PermissionName = "auth:session:read"
	// PermAuthSessionRevoke allows revoking sessions for any user.
	PermAuthSessionRevoke      core.PermissionName = "auth:session:revoke"
	PermAuthAccountImpersonate core.PermissionName = "auth:account:impersonate"
)
