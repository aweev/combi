package project_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/project"
	project_mocks "github.com/combi/pkg/domain/project/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	project_service "github.com/combi/project"
)

type serviceTestSuite struct {
	repo    *project_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	bus     *events_mocks.MockBus
	authz   *rbac_mocks.MockAuthorizationService
	service *project_service.Service
	actor   *auth.Actor
	ctx     context.Context
}

func setupTest(t *testing.T) *serviceTestSuite {
	s := &serviceTestSuite{
		repo:  project_mocks.NewMockRepository(t),
		audit: audit_mocks.NewMockEventRecorder(t),
		bus:   events_mocks.NewMockBus(t),
		authz: rbac_mocks.NewMockAuthorizationService(t),
		actor: &auth.Actor{UserID: uuid.New()},
	}

	s.service = project_service.NewService(
		s.repo, s.audit, s.bus, s.authz, &log.NoOpLogger{},
	)
	s.ctx = auth.NewContextWithActor(context.Background(), s.actor)

	return s
}

// Helper for mocking transactions
func (s *serviceTestSuite) mockTx() {
	s.repo.EXPECT().WithTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(project.Repository) error) error {
			return fn(s.repo)
		})
}

// --- Project Lifecycle ---

func TestService_CreateProject(t *testing.T) {
	s := setupTest(t)
	params := project.CreateProjectParams{
		Name: "New Initiative",
		Type: project.TypeCorporateCollab,
	}

	t.Run("Success", func(t *testing.T) {
		s.mockTx()

		// 1. Create Project
		s.repo.EXPECT().CreateProject(mock.Anything, mock.MatchedBy(func(p *project.Project) bool {
			return p.Name == "New Initiative" && p.CreatorID == s.actor.UserID && p.Status == project.StatusPlanning
		})).Return(nil).Once()

		// 2. Add Member (Admin)
		s.repo.EXPECT().CreateMember(mock.Anything, mock.MatchedBy(func(m *project.ProjectMember) bool {
			return m.UserID == s.actor.UserID && m.Role == project.RoleAdmin
		})).Return(nil).Once()

		// 3. Log Activity
		s.repo.EXPECT().CreateActivity(mock.Anything, mock.MatchedBy(func(a *project.Activity) bool {
			return a.Type == project.ActivityProjectCreated
		})).Return(nil).Once()

		// 4. Audit & Events
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventProjectCreated
		})).Return(nil).Once()

		s.bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e any) bool {
			return true // Simplified check
		})).Return(nil).Once()

		p, err := s.service.CreateProject(s.ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, p)
	})
}

func TestService_GetProject(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()

	t.Run("Success (Member)", func(t *testing.T) {
		// Mock Access Check
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleMember}, nil).Once()

		s.repo.EXPECT().GetProject(mock.Anything, pID).
			Return(&project.Project{ID: pID, Name: "Found"}, nil).Once()

		p, err := s.service.GetProject(s.ctx, pID)
		require.NoError(t, err)
		assert.Equal(t, "Found", p.Name)
	})

	t.Run("Access Denied (Not Member)", func(t *testing.T) {
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(nil, project.ErrMemberNotFound).Once()

		_, err := s.service.GetProject(s.ctx, pID)
		assert.ErrorIs(t, err, project.ErrAccessDenied)
	})
}

// --- Member Management ---

func TestService_AddMember(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()
	newUserID := uuid.New()

	t.Run("Success (Admin Adding Member)", func(t *testing.T) {
		s.mockTx()

		// Access Check (Must be Admin to manage members)
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleAdmin}, nil).Once()

		s.repo.EXPECT().CreateMember(mock.Anything, mock.MatchedBy(func(m *project.ProjectMember) bool {
			return m.UserID == newUserID && m.Role == project.RoleMember
		})).Return(nil).Once()

		s.repo.EXPECT().CreateActivity(mock.Anything, mock.MatchedBy(func(a *project.Activity) bool {
			return a.Type == project.ActivityMemberAdded
		})).Return(nil).Once()

		s.bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := s.service.AddMember(s.ctx, pID, newUserID, project.RoleMember)
		require.NoError(t, err)
	})

	t.Run("Forbidden (Member Adding Member)", func(t *testing.T) {
		// Access Check (Member role is insufficient for ManageMembers)
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleMember}, nil).Once()

		_, err := s.service.AddMember(s.ctx, pID, newUserID, project.RoleMember)
		assert.ErrorIs(t, err, project.ErrAccessDenied)
	})
}

func TestService_RemoveMember(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()
	targetID := uuid.New()

	t.Run("Fail: Last Admin", func(t *testing.T) {
		s.mockTx()

		// Access Check
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleAdmin}, nil).Once()

		// Logic Check: Target is Admin, Count is 1
		s.repo.EXPECT().GetMember(mock.Anything, pID, targetID).
			Return(&project.ProjectMember{Role: project.RoleAdmin}, nil).Once()
		s.repo.EXPECT().CountAdmins(mock.Anything, pID).Return(1, nil).Once()

		err := s.service.RemoveMember(s.ctx, pID, targetID)
		assert.ErrorIs(t, err, project.ErrCannotRemoveLastAdmin)
	})
}

// --- Task Management ---

func TestService_CreateTask(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()
	params := project.CreateTaskParams{
		ProjectID: pID,
		Title:     "New Task",
		CreatorID: s.actor.UserID,
	}

	t.Run("Success", func(t *testing.T) {
		s.mockTx()

		// Access Check (Member can create tasks)
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleMember}, nil).Once()

		s.repo.EXPECT().CreateTask(mock.Anything, mock.MatchedBy(func(task *project.Task) bool {
			return task.Title == "New Task" && task.Status == project.TaskStatusTodo
		})).Return(nil).Once()

		s.repo.EXPECT().CreateActivity(mock.Anything, mock.MatchedBy(func(a *project.Activity) bool {
			return a.Type == project.ActivityTaskCreated
		})).Return(nil).Once()

		_, err := s.service.CreateTask(s.ctx, params)
		require.NoError(t, err)
	})
}

func TestService_MoveTaskStatus(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()
	taskID := uuid.New()
	task := &project.Task{ID: taskID, ProjectID: pID, Status: project.TaskStatusTodo}

	t.Run("Success", func(t *testing.T) {
		s.mockTx()

		// 1. Get Task
		s.repo.EXPECT().GetTask(mock.Anything, taskID).Return(task, nil).Once()

		// 2. Check Access
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleMember}, nil).Once()

		// 3. Update Task
		s.repo.EXPECT().UpdateTask(mock.Anything, mock.MatchedBy(func(t *project.Task) bool {
			return t.Status == project.TaskStatusDone
		})).Return(nil).Once()

		// 4. Log Activity
		s.repo.EXPECT().CreateActivity(mock.Anything, mock.MatchedBy(func(a *project.Activity) bool {
			return a.Type == project.ActivityTaskStatusChanged
		})).Return(nil).Once()

		_, err := s.service.MoveTaskStatus(s.ctx, taskID, project.TaskStatusDone)
		require.NoError(t, err)
	})
}

// --- Milestone ---

func TestService_CompleteMilestone(t *testing.T) {
	s := setupTest(t)
	pID := uuid.New()
	mID := uuid.New()
	m := &project.Milestone{ID: mID, ProjectID: pID}

	t.Run("Success", func(t *testing.T) {
		s.repo.EXPECT().GetMilestone(mock.Anything, mID).Return(m, nil).Once()
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleAdmin}, nil).Once()

		// Tasks Check
		s.repo.EXPECT().CountOpenTasksInMilestone(mock.Anything, mID).Return(0, nil).Once()

		// Update
		s.repo.EXPECT().UpdateMilestone(mock.Anything, mock.MatchedBy(func(mil *project.Milestone) bool {
			return mil.CompletedAt != nil
		})).Return(nil).Once()

		err := s.service.CompleteMilestone(s.ctx, mID)
		require.NoError(t, err)
	})

	t.Run("Fail: Open Tasks", func(t *testing.T) {
		s.repo.EXPECT().GetMilestone(mock.Anything, mID).Return(m, nil).Once()
		s.repo.EXPECT().GetMember(mock.Anything, pID, s.actor.UserID).
			Return(&project.ProjectMember{Role: project.RoleAdmin}, nil).Once()

		s.repo.EXPECT().CountOpenTasksInMilestone(mock.Anything, mID).Return(2, nil).Once()

		err := s.service.CompleteMilestone(s.ctx, mID)
		assert.ErrorIs(t, err, project.ErrMilestoneIncomplete)
	})
}
