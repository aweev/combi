package reporting

import "github.com/combi/pkg/domain/core"

const (
	// Emitted when a report request is queued.
	// Consumers: None (internal trigger for worker).
	EventReportRequested core.EventAction = "report.requested"

	// Emitted when a report is ready for download.
	// Consumers: NotificationService (email user with link).
	EventReportCompleted core.EventAction = "report.completed"

	// Emitted when generation fails.
	// Consumers: NotificationService (alert user).
	EventReportFailed core.EventAction = "report.failed"
)
