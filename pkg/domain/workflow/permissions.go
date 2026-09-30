package workflow

import "github.com/combi/pkg/domain/core"

const (
	// PermWorkflowManage allows cancelling or forcing workflows.
	PermWorkflowManage core.PermissionName = "workflow:manage"

	// PermWorkflowApprove allows deciding on approval requests.
	// (Note: Specific approvals usually check specific roles like "finance_manager", this is a generic override).
	PermWorkflowApprove core.PermissionName = "workflow:approve"
)
