package user

import "github.com/combi/pkg/domain/core"

const (
	// --- User-Facing Permissions ---
	PermUserProfileReadSelf   core.PermissionName = "user:profile:read:self"   // Allows reading one's own profile.
	PermUserProfileUpdateSelf core.PermissionName = "user:profile:update:self" // Allows updating one's own profile.
	PermUserConnectionManage  core.PermissionName = "user:connection:manage"   // Allows creating/deleting connections (following, etc.).

	// --- Admin-Level Permissions ---
	PermUserRead      core.PermissionName = "user:read:any"      // Allows listing and reading any user's profile.
	PermUserManage    core.PermissionName = "user:manage:any"    // Allows updating/managing any user's profile.
	PermUserAnonymize core.PermissionName = "user:anonymize:any" // Allows triggering the anonymization job for any user.
)
