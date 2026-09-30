// internal/domain/queue/events.go
package queue

import "github.com/combi/pkg/domain/core"

const (
	EventJobCreated        core.EventAction = "queue.job.created"
	EventJobCancelled      core.EventAction = "queue.job.cancelled"
	EventFailedJobRetried  core.EventAction = "queue.failed_job.retried"
	EventFailedJobDeleted  core.EventAction = "queue.failed_job.deleted"
	EventJobCompleted      core.EventAction = "queue.job.completed"
	EventScheduleCreated   core.EventAction = "queue.schedule.created"
	EventScheduleUpdated   core.EventAction = "queue.schedule.updated"
	EventScheduleDeleted   core.EventAction = "queue.schedule.deleted"
	EventQueuePaused       core.EventAction = "queue.paused"
	EventQueueResumed      core.EventAction = "queue.resumed"
	EventQueuePurged       core.EventAction = "queue.purged"
	EventJobStalled        core.EventAction = "queue.job.stalled"
	EventJobFailed         core.EventAction = "queue.job.failed"
	EventDLQMessageCreated core.EventAction = "queue.dlq.message_created"

	// --- Queue Management Events ---
	JobCreated            core.EventAction = "queue.job.created"
	JobCancelled          core.EventAction = "queue.job.cancelled"
	FailedJobRetried      core.EventAction = "queue.failed_job.retried"
	FailedJobDeleted      core.EventAction = "queue.failed_job.deleted"
	BulkFailedJobsRetried core.EventAction = "queue.failed_job.bulk_retried"
	BulkFailedJobsDeleted core.EventAction = "queue.failed_job.bulk_deleted"
	ScheduleCreated       core.EventAction = "queue.schedule.created"
	ScheduleUpdated       core.EventAction = "queue.schedule.updated"
	ScheduleDeleted       core.EventAction = "queue.schedule.deleted"
	ScheduleTriggered     core.EventAction = "queue.schedule.triggered"
	QueuePaused           core.EventAction = "queue.system.paused"
	QueueResumed          core.EventAction = "queue.system.resumed"
	QueuePurged           core.EventAction = "queue.system.purged"
	DLQMessageDeleted     core.EventAction = "queue.dlq.message_deleted"
	DLQMessageRequeued    core.EventAction = "queue.dlq.message_requeued"
)
