// internal/domain/event/jobs.go
package event

import "github.com/combi/pkg/domain/core"

const (
	// JobEventSendReminders sends reminders to registered attendees before an event starts.
	JobEventSendReminders core.JobType = "event:send_reminders"
	// JobEventUpdateStatus transitions event statuses (e.g., from PUBLISHED to COMPLETED).
	JobEventUpdateStatus core.JobType = "event:update_status"
)
