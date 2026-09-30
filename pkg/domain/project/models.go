package project

import (
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type ProjectStatus string

const (
	StatusPlanning   ProjectStatus = "PLANNING"
	StatusInProgress ProjectStatus = "IN_PROGRESS"
	StatusCompleted  ProjectStatus = "COMPLETED"
	StatusOnHold     ProjectStatus = "ON_HOLD"
	StatusArchived   ProjectStatus = "ARCHIVED"
	StatusCancelled  ProjectStatus = "CANCELLED"
)

type ProjectType string

const (
	TypeCorporateCollab ProjectType = "CORPORATE_COLLAB" // Partner collaboration
	TypeVolunteerInit   ProjectType = "VOLUNTEER_INIT"   // Volunteer Group
	TypeInternal        ProjectType = "INTERNAL"         // Staff only
)

type ProjectMemberRole string

const (
	RoleAdmin       ProjectMemberRole = "ADMIN"       // Can manage project settings and members.
	RoleMember      ProjectMemberRole = "MEMBER"      // Can create/edit tasks and upload files.
	RoleCommenter   ProjectMemberRole = "COMMENTER"   // Can only comment and view.
	RoleStakeholder ProjectMemberRole = "STAKEHOLDER" // View-only access.
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "TODO"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusInReview   TaskStatus = "IN_REVIEW"
	TaskStatusDone       TaskStatus = "DONE"
)

type ActivityType string

const (
	ActivityProjectCreated       ActivityType = "PROJECT_CREATED"
	ActivityProjectStatusChanged ActivityType = "PROJECT_STATUS_CHANGED"
	ActivityMemberAdded          ActivityType = "MEMBER_ADDED"
	ActivityMemberRemoved        ActivityType = "MEMBER_REMOVED"
	ActivityTaskCreated          ActivityType = "TASK_CREATED"
	ActivityTaskStatusChanged    ActivityType = "TASK_STATUS_CHANGED"
	ActivityTaskAssigned         ActivityType = "TASK_ASSIGNED"
	ActivityFileUploaded         ActivityType = "FILE_UPLOADED"
	ActivityFileDeleted          ActivityType = "FILE_DELETED"
)

// --- Core Models ---

// Project is the central entity for a collaborative workspace.
type Project struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	OrganizationID *uuid.UUID    `json:"org_id" db:"org_id"` // Optional link to an organization (for partners)
	Name           string        `json:"name" db:"name"`
	Description    string        `json:"description" db:"description"`
	Type           ProjectType   `json:"type" db:"type"`
	Status         ProjectStatus `json:"status" db:"status"`
	CreatorID      uuid.UUID     `json:"creator_id" db:"creator_id"`

	StartDate *time.Time `json:"start_date" db:"start_date"`
	EndDate   *time.Time `json:"end_date" db:"end_date"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ProjectMember represents a user's membership and role within a specific project.
type ProjectMember struct {
	ProjectID uuid.UUID         `json:"project_id" db:"project_id"`
	UserID    uuid.UUID         `json:"user_id" db:"user_id"`
	Role      ProjectMemberRole `json:"role" db:"role"`
	JoinedAt  time.Time         `json:"joined_at" db:"joined_at"`
}

// Milestone represents a major checkpoint in the project timeline.
type Milestone struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ProjectID   uuid.UUID  `json:"project_id" db:"project_id"`
	Title       string     `json:"title" db:"title"`
	DueDate     time.Time  `json:"due_date" db:"due_date"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	IsCritical  bool       `json:"is_critical" db:"is_critical"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Task represents a single actionable item within a project.
type Task struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ProjectID   uuid.UUID  `json:"project_id" db:"project_id"`
	MilestoneID *uuid.UUID `json:"milestone_id" db:"milestone_id"`
	Title       string     `json:"title" db:"title"`
	Description *string    `json:"description" db:"description"`
	Status      TaskStatus `json:"status" db:"status"`
	Priority    int        `json:"priority" db:"priority"` // 1 (Low) - 5 (Critical)
	CreatorID   uuid.UUID  `json:"creator_id" db:"creator_id"`
	AssigneeID  *uuid.UUID `json:"assignee_id" db:"assignee_id"`
	DueDate     *time.Time `json:"due_date" db:"due_date"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// ProjectFile links a file from the StorageService to a project.
type ProjectFile struct {
	ID          uuid.UUID `json:"id" db:"id"`
	ProjectID   uuid.UUID `json:"project_id" db:"project_id"`
	FileID      uuid.UUID `json:"file_id" db:"file_id"` // UUID from StorageService
	FileName    string    `json:"file_name" db:"file_name"`
	FileType    string    `json:"file_type" db:"file_type"`
	URL         string    `json:"url" db:"url"` // Cached/Direct URL
	UploaderID  uuid.UUID `json:"uploader_id" db:"uploader_id"`
	Description *string   `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Activity represents an event or change that occurred within a project, forming a timeline.
type Activity struct {
	ID         uuid.UUID              `json:"id" db:"id"`
	ProjectID  uuid.UUID              `json:"project_id" db:"project_id"`
	ActorID    uuid.UUID              `json:"actor_id" db:"actor_id"`
	Type       ActivityType           `json:"type" db:"type"`
	TargetType *string                `json:"target_type" db:"target_type"` // e.g., "TASK", "FILE"
	TargetID   *uuid.UUID             `json:"target_id" db:"target_id"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// --- View Models / DTOs for complex queries ---

// ProjectDetails is a rich DTO used for API responses.
type ProjectDetails struct {
	Project
	Members    []*ProjectMember `json:"members"`
	Tasks      []*Task          `json:"tasks"`
	Files      []*ProjectFile   `json:"files"`
	Milestones []*Milestone     `json:"milestones"`
	Creator    *UserSummary     `json:"creator"`
}

// UserSummary avoids dependency on full User Service.
type UserSummary struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
}
