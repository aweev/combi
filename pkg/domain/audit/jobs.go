package audit

import "github.com/combi/pkg/domain/core"

const (
	// JobAuditExport is a long-running job to export a large set of audit events.
	JobAuditExport             core.JobType = "audit:export"
	JobComplianceAnonymizeUser core.JobType = "compliance:anonymize_user"
	JobAuditStoreEvent         core.JobType = "audit:store_event"

	// JobProcessProviderWebhooks processes incoming status updates from providers.
	JobProcessProviderWebhooks core.JobType = "notification:process_provider_webhooks"
)
