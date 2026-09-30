package project

import "github.com/combi/pkg/domain/core"

const (
	// Org-Level Permission (Checked by Org Service / RBAC before creating)
	PermProjectCreate core.PermissionName = "project:create"

	// Project-Level Permissions (Checked against ProjectMemberRole)
	PermProjectRead          core.PermissionName = "project:read"
	PermProjectUpdate        core.PermissionName = "project:update"
	PermProjectDelete        core.PermissionName = "project:delete"
	PermProjectManageMembers core.PermissionName = "project:manage_members"

	PermProjectTaskCreate core.PermissionName = "project:task:create"
	PermProjectTaskUpdate core.PermissionName = "project:task:update"
	PermProjectTaskDelete core.PermissionName = "project:task:delete"

	PermProjectFileUpload core.PermissionName = "project:file:upload"
	PermProjectFileDelete core.PermissionName = "project:file:delete"
)
