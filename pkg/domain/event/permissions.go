// internal/domain/event/permissions.go
package event

import "github.com/combi/pkg/domain/core"

const (
	PermEventManage           core.PermissionName = "event:manage"
	PermEventRegistrationRead core.PermissionName = "event:registration:read"
	PermEventCheckIn          core.PermissionName = "event:checkin"
	PermEventVenueManage      core.PermissionName = "event:venue:manage"
	PermEventSpeakerManage    core.PermissionName = "event:speaker:manage"
)
