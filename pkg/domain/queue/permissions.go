package queue

import "github.com/combi/pkg/domain/core"

type Permission core.PermissionName

const (

	// Read permissions
	PermQueueJobRead       core.PermissionName = "queue:job:read"
	PermQueueFailedJobRead core.PermissionName = "queue:failed_job:read"
	PermQueueSystemRead    core.PermissionName = "queue:system:read" // For stats, workers, etc.

	// Write/Operate permissions
	PermQueueJobCreate       core.PermissionName = "queue:job:create"
	PermQueueJobCancel       core.PermissionName = "queue:job:cancel"
	PermQueueFailedJobRetry  core.PermissionName = "queue:failed_job:retry"
	PermQueueFailedJobDelete core.PermissionName = "queue:failed_job:delete"
	PermQueueScheduleManage  core.PermissionName = "queue:schedule:manage" // Create, update, delete, trigger
	PermQueueSystemOperate   core.PermissionName = "queue:system:operate"  // Pause, resume, purge, DLQ ops
)
