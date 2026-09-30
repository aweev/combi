package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewProjectRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Helper to create a user ID
	userID := uuid.New()

	t.Run("Project_Lifecycle", func(t *testing.T) {
		orgID := uuid.New()
		pID := uuid.New()

		proj := &project.Project{
			ID:             pID,
			OrganizationID: &orgID,
			Name:           "Alpha Project",
			Description:    "Test desc",
			Type:           project.TypeInternal,
			Status:         project.StatusPlanning,
			CreatorID:      userID,
		}

		// Create
		require.NoError(t, repo.CreateProject(ctx, proj))

		// Duplicate check
		assert.ErrorIs(t, repo.CreateProject(ctx, proj), project.ErrDuplicateProjectName)

		// Get
		got, err := repo.GetProject(ctx, pID)
		require.NoError(t, err)
		assert.Equal(t, "Alpha Project", got.Name)

		// Get by Name & Org
		gotByName, err := repo.GetProjectByNameAndOrg(ctx, "Alpha Project", &orgID)
		require.NoError(t, err)
		assert.Equal(t, pID, gotByName.ID)

		// Update
		got.Status = project.StatusInProgress
		require.NoError(t, repo.UpdateProject(ctx, got))

		// List
		list, count, err := repo.ListProjects(ctx, project.ProjectFilter{
			Status: func() *project.ProjectStatus { s := project.StatusInProgress; return &s }(),
			Limit:  10,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, pID, list[0].ID)
	})

	t.Run("Members", func(t *testing.T) {
		pID := uuid.New()
		repo.CreateProject(ctx, &project.Project{ID: pID, Name: "M", Type: "I", Status: "P", CreatorID: userID})

		memberID := uuid.New()
		member := &project.ProjectMember{
			ProjectID: pID,
			UserID:    memberID,
			Role:      project.RoleMember,
		}

		// Add
		require.NoError(t, repo.CreateMember(ctx, member))

		// Get
		got, err := repo.GetMember(ctx, pID, memberID)
		require.NoError(t, err)
		assert.Equal(t, project.RoleMember, got.Role)

		// Count Admins (should be 0)
		count, _ := repo.CountAdmins(ctx, pID)
		assert.Equal(t, 0, count)

		// Update to Admin
		member.Role = project.RoleAdmin
		require.NoError(t, repo.UpdateMember(ctx, member))
		count, _ = repo.CountAdmins(ctx, pID)
		assert.Equal(t, 1, count)

		// List
		list, err := repo.ListMembers(ctx, pID)
		require.NoError(t, err)
		assert.Len(t, list, 1)

		// Delete
		require.NoError(t, repo.DeleteMember(ctx, pID, memberID))
		_, err = repo.GetMember(ctx, pID, memberID)
		assert.ErrorIs(t, err, project.ErrMemberNotFound)
	})

	t.Run("Tasks_And_Milestones", func(t *testing.T) {
		pID := uuid.New()
		repo.CreateProject(ctx, &project.Project{ID: pID, Name: "TM", Type: "I", Status: "P", CreatorID: userID})

		// Create Milestone
		mID := uuid.New()
		milestone := &project.Milestone{
			ID:        mID,
			ProjectID: pID,
			Title:     "V1 Launch",
			DueDate:   time.Now().Add(24 * time.Hour),
		}
		require.NoError(t, repo.CreateMilestone(ctx, milestone))

		// Create Task linked to Milestone
		taskID := uuid.New()
		task := &project.Task{
			ID:          taskID,
			ProjectID:   pID,
			MilestoneID: &mID,
			Title:       "Design DB",
			Status:      project.TaskStatusTodo,
			CreatorID:   userID,
			Priority:    1,
		}
		require.NoError(t, repo.CreateTask(ctx, task))

		// Count Open Tasks
		open, err := repo.CountOpenTasksInMilestone(ctx, mID)
		require.NoError(t, err)
		assert.Equal(t, 1, open)

		// Close Task
		task.Status = project.TaskStatusDone
		require.NoError(t, repo.UpdateTask(ctx, task))

		open, _ = repo.CountOpenTasksInMilestone(ctx, mID)
		assert.Equal(t, 0, open)
	})

	t.Run("Files", func(t *testing.T) {
		pID := uuid.New()
		repo.CreateProject(ctx, &project.Project{ID: pID, Name: "F", Type: "I", Status: "P", CreatorID: userID})

		fID := uuid.New()
		file := &project.ProjectFile{
			ID:         fID,
			ProjectID:  pID,
			FileID:     uuid.New(),
			FileName:   "spec.pdf",
			FileType:   "application/pdf",
			URL:        "http://files/spec.pdf",
			UploaderID: userID,
		}

		require.NoError(t, repo.CreateFile(ctx, file))

		got, err := repo.GetFile(ctx, fID)
		require.NoError(t, err)
		assert.Equal(t, "spec.pdf", got.FileName)

		count, err := repo.CountFiles(ctx, pID)
		assert.Equal(t, 1, count)
		assert.NoError(t, err)

		require.NoError(t, repo.DeleteFile(ctx, fID))
	})

	t.Run("Activity_Log", func(t *testing.T) {
		pID := uuid.New()
		repo.CreateProject(ctx, &project.Project{ID: pID, Name: "A", Type: "I", Status: "P", CreatorID: userID})

		targetID := uuid.New()
		act := &project.Activity{
			ID:         uuid.New(),
			ProjectID:  pID,
			ActorID:    userID,
			Type:       project.ActivityTaskCreated,
			TargetType: func() *string { s := "TASK"; return &s }(),
			TargetID:   &targetID,
			Metadata:   map[string]interface{}{"foo": "bar"},
		}

		require.NoError(t, repo.CreateActivity(ctx, act))

		list, total, err := repo.ListActivities(ctx, pID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, "bar", list[0].Metadata["foo"])
	})
}
