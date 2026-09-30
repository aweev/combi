package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/workflow"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func newTestDefinition(t *testing.T) *workflow.WorkflowDefinition {
	return &workflow.WorkflowDefinition{
		ID:          uuid.New(),
		Name:        "Test Workflow",
		Type:        workflow.TypeVolunteerOnboarding,
		Version:     1,
		Description: "A test definition",
		IsActive:    true,
		Config:      json.RawMessage(`{"auto_approve": false}`),
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

func newTestInstance(t *testing.T, defID uuid.UUID, subjectID uuid.UUID) *workflow.WorkflowInstance {
	return &workflow.WorkflowInstance{
		ID:          uuid.New(),
		WorkflowID:  defID,
		TemporalID:  "wf-" + uuid.NewString(),
		RunID:       uuid.NewString(),
		SubjectID:   subjectID,
		Status:      workflow.StatusRunning,
		CurrentStep: "initial_check",
		InputData:   json.RawMessage(`{"applicant": "John Doe"}`),
		ResultData:  json.RawMessage(`{}`),
		StartedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

// --- Tests ---

func TestWorkflowRepository_DefinitionCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewWorkflowRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Create and Get Definition", func(t *testing.T) {
		def := newTestDefinition(t)
		err := repo.CreateDefinition(ctx, def)
		require.NoError(t, err)

		got, err := repo.GetDefinition(ctx, def.ID)
		require.NoError(t, err)
		assert.Equal(t, def.ID, got.ID)
		assert.Equal(t, def.Name, got.Name)
		assert.JSONEq(t, string(def.Config), string(got.Config))
	})

	t.Run("List Definitions", func(t *testing.T) {
		def1 := newTestDefinition(t)
		def1.Name = "Active Def"

		def2 := newTestDefinition(t)
		def2.Name = "Inactive Def"
		def2.IsActive = false

		require.NoError(t, repo.CreateDefinition(ctx, def1))
		require.NoError(t, repo.CreateDefinition(ctx, def2))

		// List Active Only
		active, err := repo.ListDefinitions(ctx, true)
		require.NoError(t, err)

		// Ensure we find the active one and DO NOT find the inactive one
		foundActive := false
		foundInactive := false
		for _, d := range active {
			if d.ID == def1.ID {
				foundActive = true
			}
			if d.ID == def2.ID {
				foundInactive = true
			}
		}
		assert.True(t, foundActive, "Should find active definition")
		assert.False(t, foundInactive, "Should not return inactive definition")

		// List All
		all, err := repo.ListDefinitions(ctx, false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)
	})
}

func TestWorkflowRepository_InstanceCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewWorkflowRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup dependency
	def := newTestDefinition(t)
	require.NoError(t, repo.CreateDefinition(ctx, def))

	t.Run("Create and Get Instance", func(t *testing.T) {
		inst := newTestInstance(t, def.ID, uuid.New())
		err := repo.CreateInstance(ctx, inst)
		require.NoError(t, err)

		got, err := repo.GetInstance(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, inst.TemporalID, got.TemporalID)
		assert.Equal(t, inst.SubjectID, got.SubjectID)
	})

	t.Run("Update Instance Status", func(t *testing.T) {
		inst := newTestInstance(t, def.ID, uuid.New())
		require.NoError(t, repo.CreateInstance(ctx, inst))

		err := repo.UpdateInstanceStatus(ctx, inst.ID, workflow.StatusCompleted, "finished")
		require.NoError(t, err)

		got, err := repo.GetInstance(ctx, inst.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, got.Status)
		assert.Equal(t, "finished", got.CurrentStep)
		assert.NotNil(t, got.CompletedAt)
	})
}

func TestWorkflowRepository_ListInstances(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewWorkflowRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup: 1 Def, 2 Instances (One running, one completed)
	def := newTestDefinition(t)
	require.NoError(t, repo.CreateDefinition(ctx, def))

	subjectID := uuid.New()

	inst1 := newTestInstance(t, def.ID, subjectID)
	require.NoError(t, repo.CreateInstance(ctx, inst1))

	inst2 := newTestInstance(t, def.ID, uuid.New()) // Diff subject
	inst2.Status = workflow.StatusCompleted
	require.NoError(t, repo.CreateInstance(ctx, inst2))

	t.Run("Filter by SubjectID", func(t *testing.T) {
		list, total, err := repo.ListInstances(ctx, workflow.InstanceFilter{SubjectID: &subjectID, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, inst1.ID, list[0].ID)
	})

	t.Run("Filter by Status", func(t *testing.T) {
		status := workflow.StatusCompleted
		list, total, err := repo.ListInstances(ctx, workflow.InstanceFilter{Status: &status, Limit: 10})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		// Should find inst2
		found := false
		for _, i := range list {
			if i.ID == inst2.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Filter by Type", func(t *testing.T) {
		wType := workflow.TypeVolunteerOnboarding
		list, total, err := repo.ListInstances(ctx, workflow.InstanceFilter{Type: &wType, Limit: 10})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2) // Both instances are of this type

		// Negative test
		wrongType := workflow.TypeGrantApproval
		list, total, err = repo.ListInstances(ctx, workflow.InstanceFilter{Type: &wrongType, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, list)
	})
}

func TestWorkflowRepository_Approvals(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewWorkflowRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup
	def := newTestDefinition(t)
	require.NoError(t, repo.CreateDefinition(ctx, def))
	inst := newTestInstance(t, def.ID, uuid.New())
	require.NoError(t, repo.CreateInstance(ctx, inst))

	req := &workflow.ApprovalRequest{
		ID:           uuid.New(),
		InstanceID:   inst.ID,
		AssigneeRole: "manager",
		Message:      "Please approve",
		Status:       "PENDING",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	t.Run("Create and Get Approval", func(t *testing.T) {
		err := repo.CreateApprovalRequest(ctx, req)
		require.NoError(t, err)

		got, err := repo.GetApprovalRequest(ctx, req.ID)
		require.NoError(t, err)
		assert.Equal(t, req.Message, got.Message)
	})

	t.Run("List Pending Approvals (Role Logic)", func(t *testing.T) {
		// 1. Specific User ID check
		userID := uuid.New()

		// Create a request assigned SPECIFICALLY to this user
		reqSpecific := &workflow.ApprovalRequest{
			ID:           uuid.New(),
			InstanceID:   inst.ID,
			AssigneeRole: "manager",
			AssigneeID:   &userID,
			Message:      "For your eyes only",
			Status:       "PENDING",
			CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.CreateApprovalRequest(ctx, reqSpecific))

		// 2. Query as that user, with role 'manager'
		pending, err := repo.ListPendingApprovals(ctx, &userID, "manager")
		require.NoError(t, err)

		// Should find BOTH the specific one AND the role-based one created in previous test
		// (assuming the first one didn't have a specific assignee)
		foundSpecific := false
		foundRole := false
		for _, p := range pending {
			if p.ID == reqSpecific.ID {
				foundSpecific = true
			}
			if p.ID == req.ID {
				foundRole = true
			}
		}
		assert.True(t, foundSpecific, "Should find specifically assigned task")
		assert.True(t, foundRole, "Should find generic role task")
	})

	t.Run("Decide Approval", func(t *testing.T) {
		deciderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Microsecond)
		note := "Looks good"

		req.Status = "APPROVED"
		req.DecidedBy = &deciderID
		req.DecidedAt = &now
		req.DecisionNote = &note

		err := repo.UpdateApprovalRequest(ctx, req)
		require.NoError(t, err)

		got, err := repo.GetApprovalRequest(ctx, req.ID)
		require.NoError(t, err)
		assert.Equal(t, "APPROVED", got.Status)
		assert.Equal(t, note, *got.DecisionNote)
		assert.NotNil(t, got.DecidedAt)
	})
}
