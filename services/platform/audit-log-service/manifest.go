// audit-log-service — ported from api/internal/services/audit/container.go
// (buildManifest). The manifest describes the audit service's public
// capabilities and is consumed by the configuration service's registry.
package auditlog

import (
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
)

// BuildManifest returns the service manifest describing the audit service's
// public capabilities (consumed by the configuration service's registry).
func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "audit-service",
		DisplayName: "Audit & Compliance",
		Description: "Immutable logs and legal hold management.",
		Icon:        "shield-check",
		Events: []core.EventDefinition{
			{Key: core.EventLegalHoldApplied, Label: "Legal Hold Applied", Description: "Data deletion is frozen for a subject."},
			{Key: core.EventAnonymizationRequested, Label: "GDPR Request", Description: "User requested Right to be Forgotten."},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         audit.JobAuditExport,
				Label:       "Export Audit Logs",
				Description: "Generate CSV export of security logs.",
				InputSchema: map[string]string{"start_date": "date", "end_date": "date"},
			},
			{
				Key:         audit.JobComplianceAnonymizeUser,
				Label:       "Anonymize User",
				Description: "Execute GDPR erasure for a specific user ID.",
				InputSchema: map[string]string{"user_id": "uuid"},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: audit.PermAuditRead, Label: "View Audit Logs", Category: "Compliance"},
			{Key: audit.PermComplianceManage, Label: "Manage Compliance", Category: "Compliance"},
		},
	}
}
