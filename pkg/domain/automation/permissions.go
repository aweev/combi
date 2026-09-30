package automation

import "github.com/combi/pkg/domain/core"

// Permissions for the automation service.
const (
	// PermAutomationSystemRead allows viewing system-wide statistics.
	PermAutomationSystemRead core.PermissionName = "automation:system:read"

	// PermAutomationRuleRead allows viewing rule configurations and their specific stats.
	PermAutomationRuleRead core.PermissionName = "automation:rule:read"
	// PermAutomationRuleManage allows creating, updating, deleting, enabling, and disabling rules.
	PermAutomationRuleManage core.PermissionName = "automation:rule:manage"

	// PermAutomationExecutionRead allows viewing execution history for all rules.
	PermAutomationExecutionRead core.PermissionName = "automation:execution:read"
	// PermAutomationExecutionOperate allows retrying and cancelling executions.
	PermAutomationExecutionOperate core.PermissionName = "automation:execution:operate"

	// PermAutomationSystemOperate allows performing high-level system operations like ad-hoc action execution.
	PermAutomationSystemOperate core.PermissionName = "automation:system:operate"
)
