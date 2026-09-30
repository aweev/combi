package notification

import "github.com/combi/pkg/domain/core"

const (
	// PermNotificationAdminRead allows viewing templates, providers, and stats.
	PermNotificationAdminRead core.PermissionName = "notification:admin:read"

	// PermNotificationTemplateManage allows creating, updating, and deleting templates.
	PermNotificationTemplateManage core.PermissionName = "notification:template:manage"

	// PermNotificationProviderManage allows creating, updating, and deleting providers.
	PermNotificationProviderManage core.PermissionName = "notification:provider:manage"
)
