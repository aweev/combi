package audit

import "github.com/combi/pkg/domain/core"

const (
	// PermAuditRead allows viewing the audit trail.
	PermAuditRead core.PermissionName = "audit:read"
	// PermAuditExport allows exporting audit logs.
	PermAuditExport core.PermissionName = "audit:export"
	// PermComplianceManage allows managing legal holds and retention policies.
	PermComplianceManage core.PermissionName = "compliance:manage"
)
