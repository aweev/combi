package messaging

import "github.com/combi/pkg/domain/core"

const (
	// PermChatUse allows basic messaging features.
	PermChatUse core.PermissionName = "chat:use"

	// PermChatModerator allows deleting other people's messages and banning users from channels.
	PermChatModerator core.PermissionName = "chat:moderate"
)
