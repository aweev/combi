package rbac

import "github.com/combi/pkg/domain/core"

const (
	// JobRBACInvalidateUserCache is a background job to clear the cached permission set for users.
	// This is triggered when a role they are assigned to is modified.
	JobRBACInvalidateUserCache core.JobType = "rbac:invalidate_user_cache"
)
