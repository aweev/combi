package project

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	ProjectRepository
	MemberRepository
	TaskRepository
	MilestoneRepository
	FileRepository
	ActivityRepository
}

type ProjectRepository interface {
	CreateProject(ctx context.Context, proj *Project) error
	GetProject(ctx context.Context, id uuid.UUID) (*Project, error)
	GetProjectByNameAndOrg(ctx context.Context, name string, orgID *uuid.UUID) (*Project, error)
	UpdateProject(ctx context.Context, proj *Project) error
	ListProjects(ctx context.Context, filter ProjectFilter) ([]*Project, int, error)
}

type MemberRepository interface {
	CreateMember(ctx context.Context, member *ProjectMember) error
	GetMember(ctx context.Context, projectID, userID uuid.UUID) (*ProjectMember, error)
	UpdateMember(ctx context.Context, member *ProjectMember) error
	DeleteMember(ctx context.Context, projectID, userID uuid.UUID) error
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]*ProjectMember, error)
	CountAdmins(ctx context.Context, projectID uuid.UUID) (int, error)
}

type TaskRepository interface {
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, id uuid.UUID) (*Task, error)
	UpdateTask(ctx context.Context, task *Task) error
	ListTasks(ctx context.Context, projectID uuid.UUID, filter TaskFilter) ([]*Task, int, error)
	CountOpenTasksInMilestone(ctx context.Context, milestoneID uuid.UUID) (int, error)
}

type MilestoneRepository interface {
	CreateMilestone(ctx context.Context, m *Milestone) error
	GetMilestone(ctx context.Context, id uuid.UUID) (*Milestone, error)
	UpdateMilestone(ctx context.Context, m *Milestone) error
	ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*Milestone, error)
}

type FileRepository interface {
	CreateFile(ctx context.Context, file *ProjectFile) error
	GetFile(ctx context.Context, id uuid.UUID) (*ProjectFile, error)
	DeleteFile(ctx context.Context, id uuid.UUID) error
	ListFiles(ctx context.Context, projectID uuid.UUID) ([]*ProjectFile, error)
	CountFiles(ctx context.Context, projectID uuid.UUID) (int, error)
}

type ActivityRepository interface {
	CreateActivity(ctx context.Context, activity *Activity) error
	ListActivities(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Activity, int, error)
}
