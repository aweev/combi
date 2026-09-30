package notification

import "github.com/combi/pkg/domain/core"

const (
	// JobSendNotification processes and sends a single notification.
	// This is the primary worker job.
	JobSendNotification core.JobType = "notification:send"

	// JobProcessScheduled finds due notifications and dispatches `notification:send` jobs for them.
	// This worker is triggered by a system-wide cron schedule.
	JobProcessScheduled core.JobType = "notification:process_scheduled"

	 // JobProcessProviderWebhooks processes incoming status updates from providers.
	JobProcessProviderWebhooks core.JobType = "notification:process_provider_webhooks"
	
	// JobCleanupOld records to keep the database lean.
	// This worker is triggered by a system-wide cron schedule.
	JobCleanupOld core.JobType = "notification:cleanup"
)
