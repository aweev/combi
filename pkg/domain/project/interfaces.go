package project

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service defines the core contract for managing collaborative projects.
type Service interface {
	// --- Project Lifecycle ---
	CreateProject(ctx context.Context, params CreateProjectParams) (*Project, error)
	GetProject(ctx context.Context, projectID uuid.UUID) (*Project, error)
	GetProjectDetails(ctx context.Context, projectID uuid.UUID) (*ProjectDetails, error)
	ListProjects(ctx context.Context, filter ProjectFilter) ([]*Project, int, error)
	UpdateProject(ctx context.Context, projectID uuid.UUID, params UpdateProjectParams) (*Project, error)
	ArchiveProject(ctx context.Context, projectID uuid.UUID) (*Project, error)

	// --- Member Management ---
	AddMember(ctx context.Context, projectID, userID uuid.UUID, role ProjectMemberRole) (*ProjectMember, error)
	RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error
	UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role ProjectMemberRole) (*ProjectMember, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]*ProjectMember, error)
	IsMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error)

	// --- Task Management ---
	CreateTask(ctx context.Context, params CreateTaskParams) (*Task, error)
	GetTask(ctx context.Context, taskID uuid.UUID) (*Task, error)
	UpdateTask(ctx context.Context, taskID uuid.UUID, params UpdateTaskParams) (*Task, error)
	MoveTaskStatus(ctx context.Context, taskID uuid.UUID, newStatus TaskStatus) (*Task, error)
	ListTasks(ctx context.Context, projectID uuid.UUID, filter TaskFilter) ([]*Task, int, error)

	// --- Milestone Management ---
	CreateMilestone(ctx context.Context, params CreateMilestoneParams) (*Milestone, error)
	UpdateMilestone(ctx context.Context, id uuid.UUID, params UpdateMilestoneParams) (*Milestone, error)
	CompleteMilestone(ctx context.Context, id uuid.UUID) error
	ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*Milestone, error)

	// --- File Management ---
	AttachFile(ctx context.Context, params AttachFileParams) (*ProjectFile, error)
	ListFiles(ctx context.Context, projectID uuid.UUID) ([]*ProjectFile, error)
	RemoveFile(ctx context.Context, fileID uuid.UUID) error

	// --- Activity ---
	ListActivity(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Activity, int, error)
}

// --- DTOs ---

type CreateProjectParams struct {
	CreatorID      uuid.UUID
	OrganizationID *uuid.UUID
	Name           string
	Description    string
	Type           ProjectType
	StartDate      *time.Time
	EndDate        *time.Time
}

type UpdateProjectParams struct {
	Name        *string
	Description *string
	Status      *ProjectStatus
	StartDate   *time.Time
	EndDate     *time.Time
}

type CreateTaskParams struct {
	CreatorID   uuid.UUID
	ProjectID   uuid.UUID
	MilestoneID *uuid.UUID
	Title       string
	Description *string
	AssigneeID  *uuid.UUID
	Priority    int
	DueDate     *time.Time
}

type UpdateTaskParams struct {
	Title       *string
	Description *string
	Status      *TaskStatus
	AssigneeID  *uuid.UUID
	Priority    *int
	DueDate     *time.Time
}

type CreateMilestoneParams struct {
	ProjectID  uuid.UUID
	Title      string
	DueDate    time.Time
	IsCritical bool
}

type UpdateMilestoneParams struct {
	Title      *string
	DueDate    *time.Time
	IsCritical *bool
}

type AttachFileParams struct {
	UploaderID  uuid.UUID
	ProjectID   uuid.UUID
	FileID      uuid.UUID
	FileName    string
	FileType    string
	URL         string
	Description *string
}

type ProjectFilter struct {
	OrganizationID *uuid.UUID
	MemberID       *uuid.UUID // Filter projects where this user is a member
	Status         *ProjectStatus
	Limit          int
	Offset         int
}

type TaskFilter struct {
	Status      *TaskStatus
	AssigneeID  *uuid.UUID
	MilestoneID *uuid.UUID
	Limit       int
	Offset      int
}
