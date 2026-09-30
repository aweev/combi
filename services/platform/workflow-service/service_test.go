package workflow_test

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/workflow"

	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	workflow_mocks "github.com/combi/pkg/domain/workflow/mocks"
	"github.com/combi/pkg/log"
	workflow_service "github.com/combi/workflow"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"
)

type serviceTestSuite struct {
	repo     *workflow_mocks.MockRepository
	temporal *mocks.Client // Temporal SDK Mock
	audit    *audit_mocks.MockEventRecorder
	service  workflow.Service
	actor    *auth.Actor
	ctx      context.Context
}

func setupTest(t *testing.T) *serviceTestSuite {
	s := &serviceTestSuite{
		repo:     workflow_mocks.NewMockRepository(t),
		temporal: new(mocks.Client),
		audit:    audit_mocks.NewMockEventRecorder(t),
		actor:    &auth.Actor{UserID: uuid.New(), AccountID: uuid.New()},
	}

	s.service = workflow_service.NewService(
		s.temporal,
		s.repo,
		s.audit,
		&log.NoOpLogger{},
	)
	s.ctx = auth.NewContextWithActor(context.Background(), s.actor)

	return s
}

// --- Definitions ---

func TestService_CreateDefinition(t *testing.T) {
	s := setupTest(t)
	params := workflow.CreateDefinitionParams{
		Name:        "ParticipantApplicationWorkflow", // Must match known name in service
		Type:        workflow.TypeVolunteerOnboarding,
		Description: "Test Def",
		Config:      map[string]any{"foo": "bar"},
	}

	s.repo.EXPECT().CreateDefinition(mock.Anything, mock.MatchedBy(func(d *workflow.WorkflowDefinition) bool {
		return d.Name == params.Name && d.Type == params.Type
	})).Return(nil).Once()

	def, err := s.service.CreateDefinition(s.ctx, params)
	require.NoError(t, err)
	assert.Equal(t, params.Name, def.Name)
}

// --- Execution ---

func TestService_StartWorkflow(t *testing.T) {
	s := setupTest(t)
	defID := uuid.New()
	subjectID := uuid.New()

	params := workflow.StartWorkflowParams{
		WorkflowDefinitionID: defID,
		SubjectID:            subjectID,
		Input:                map[string]any{"application_id": uuid.New().String()},
	}

	def := &workflow.WorkflowDefinition{
		ID:   defID,
		Name: workflow.ApplicationReviewWorkflowName, // Valid Name
		Type: workflow.TypeVolunteerOnboarding,
	}

	// 1. Get Def
	s.repo.EXPECT().GetDefinition(mock.Anything, defID).Return(def, nil).Once()

	// 2. Idempotency Check (Empty list = go ahead)
	s.repo.EXPECT().ListInstances(mock.Anything, mock.Anything).Return([]workflow.WorkflowInstance{}, 0, nil).Once()

	// 3. Temporal Execution
	mockRun := new(mocks.WorkflowRun)
	mockRun.On("GetID").Return("wf-123")
	mockRun.On("GetRunID").Return("run-123")

	s.temporal.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(mockRun, nil).Once()

	// 4. Create Instance
	s.repo.EXPECT().CreateInstance(mock.Anything, mock.MatchedBy(func(i *workflow.WorkflowInstance) bool {
		return i.SubjectID == subjectID && i.TemporalID == "wf-123"
	})).Return(nil).Once()

	// 5. Audit
	s.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

	inst, err := s.service.StartWorkflow(s.ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "wf-123", inst.TemporalID)
}

func TestService_StartWorkflow_Idempotency(t *testing.T) {
	s := setupTest(t)
	defID := uuid.New()
	subjectID := uuid.New()

	def := &workflow.WorkflowDefinition{ID: defID, Name: "Test"}

	s.repo.EXPECT().GetDefinition(mock.Anything, defID).Return(def, nil).Once()

	// Idempotency: Return existing running instance
	existing := []workflow.WorkflowInstance{{ID: uuid.New()}}
	s.repo.EXPECT().ListInstances(mock.Anything, mock.MatchedBy(func(f workflow.InstanceFilter) bool {
		return *f.SubjectID == subjectID
	})).Return(existing, 1, nil).Once()

	_, err := s.service.StartWorkflow(s.ctx, workflow.StartWorkflowParams{WorkflowDefinitionID: defID, SubjectID: subjectID})
	assert.ErrorIs(t, err, workflow.ErrWorkflowAlreadyExists)
}

func TestService_CancelWorkflow(t *testing.T) {
	s := setupTest(t)
	instID := uuid.New()
	inst := &workflow.WorkflowInstance{
		ID:         instID,
		Status:     workflow.StatusRunning,
		TemporalID: "wf-123",
		RunID:      "run-123",
	}

	s.repo.EXPECT().GetInstance(mock.Anything, instID).Return(inst, nil).Once()

	s.temporal.On("CancelWorkflow", mock.Anything, "wf-123", "run-123").Return(nil).Once()

	s.repo.EXPECT().UpdateInstanceStatus(mock.Anything, instID, workflow.StatusCancelled, mock.Anything).Return(nil).Once()

	s.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

	err := s.service.CancelWorkflow(s.ctx, instID, "User Request")
	require.NoError(t, err)
}

// --- Approvals ---

func TestService_SubmitApproval(t *testing.T) {
	s := setupTest(t)
	reqID := uuid.New()
	instID := uuid.New()

	approvalReq := &workflow.ApprovalRequest{
		ID:         reqID,
		InstanceID: instID,
		Status:     "PENDING",
	}

	inst := &workflow.WorkflowInstance{
		ID:         instID,
		TemporalID: "wf-123",
		RunID:      "run-123",
	}

	params := workflow.DecideApprovalParams{
		DeciderID: s.actor.UserID,
		Approved:  true,
		Note:      "Looks good",
	}

	t.Run("Success", func(t *testing.T) {
		// 1. Get Request
		s.repo.EXPECT().GetApprovalRequest(mock.Anything, reqID).Return(approvalReq, nil).Once()

		// 2. Update DB
		s.repo.EXPECT().UpdateApprovalRequest(mock.Anything, mock.MatchedBy(func(r *workflow.ApprovalRequest) bool {
			return r.Status == "APPROVED" && *r.DecidedBy == s.actor.UserID
		})).Return(nil).Once()

		// 3. Get Instance (for Temporal IDs)
		s.repo.EXPECT().GetInstance(mock.Anything, instID).Return(inst, nil).Once()

		// 4. Signal Temporal
		s.temporal.On("SignalWorkflow", mock.Anything, "wf-123", "run-123", "APPLICATION_DECISION", mock.Anything).
			Return(nil).Once()

		err := s.service.SubmitApproval(s.ctx, reqID, params)
		require.NoError(t, err)
	})

	t.Run("Already Processed", func(t *testing.T) {
		doneReq := *approvalReq
		doneReq.Status = "APPROVED"
		s.repo.EXPECT().GetApprovalRequest(mock.Anything, reqID).Return(&doneReq, nil).Once()

		err := s.service.SubmitApproval(s.ctx, reqID, params)
		assert.ErrorIs(t, err, workflow.ErrApprovalProcessed)
	})
}
