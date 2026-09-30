package project

import (
	"time"

	"github.com/google/uuid"
)

// const (
// 	// Project Lifecycle
// 	EventProjectCreated   core.EventAction = "project.created"
// 	EventProjectUpdated   core.EventAction = "project.updated"
// 	EventProjectArchived  core.EventAction = "project.archived"
// 	EventProjectCompleted core.EventAction = "project.completed"

// 	// Member Events
// 	EventProjectMemberAdded       core.EventAction = "project.member.added"
// 	EventProjectMemberRemoved     core.EventAction = "project.member.removed"
// 	EventProjectMemberRoleUpdated core.EventAction = "project.member.role_updated"

// 	// Task Events
// 	EventProjectTaskCreated       core.EventAction = "project.task.created"
// 	EventProjectTaskCompleted     core.EventAction = "project.task.completed"
// 	EventProjectTaskAssigned      core.EventAction = "project.task.assigned"
// 	EventProjectTaskStatusChanged core.EventAction = "project.task.status_changed"

// 	// Resource Events
// 	EventProjectFileUploaded core.EventAction = "project.file.uploaded"
// 	EventProjectFileDeleted  core.EventAction = "project.file.deleted"
// )

// HoursLoggedPayload is critical for Impact tracking.
type HoursLoggedPayload struct {
	LogID       uuid.UUID `json:"log_id"`
	UserID      uuid.UUID `json:"user_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	Hours       float64   `json:"hours"`
	DateWorked  time.Time `json:"date_worked"`
	Description string    `json:"description"`
}

type ProjectCompletedPayload struct {
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	ManagerID uuid.UUID `json:"manager_id"`
}
