package user

import "github.com/combi/pkg/domain/core"

const (

	// JobCleanupPendingUsers finds and removes user accounts that were never verified.
	JobCleanupPendingUsers core.JobType = "user:cleanup_pending"
	JobUserFileCleanup     core.JobType = "user:cleanup_files"
)
