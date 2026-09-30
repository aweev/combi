package workflow_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/workflow"
	workflow_svc "github.com/combi/workflow"
)

func TestParticipantApplicationWorkflow_Success(t *testing.T) {
	// 1. Setup Test Suite
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()

	// 2. Setup Data
	appID := uuid.New()
	userID := uuid.New()
	input := workflow.ApplicationReviewInput{
		ApplicationID: appID,
		UserID:        userID,
	}

	// 3. Mock Activities
	// We use the string names of the activities or the function references.
	// Since we are inside the same package tree or importing it, we reference the struct methods.

	// Create a mock Activities struct to reference its methods?
	// Temporal mocks rely on the function signature.
	var a *workflow_svc.Activities

	// Expect: Notification -> Application Received
	env.OnActivity(a.SendNotification, mock.Anything, userID, "application-received", mock.Anything, "application", appID).
		Return(nil).Once()

	// Expect: Update Status -> Approved (Saga Step A)
	env.OnActivity(a.UpdateApplicationStatus, mock.Anything, appID, catalog.AppStatusApproved, mock.Anything, mock.Anything).
		Return(nil).Once()

	// Expect: Activate Profile (Saga Step B)
	env.OnActivity(a.ActivateParticipantProfile, mock.Anything, userID).
		Return(nil).Once()

	// Expect: Notification -> Approved (Saga Step C)
	env.OnActivity(a.SendNotification, mock.Anything, userID, "application-approved", mock.Anything, "application", appID).
		Return(nil).Once()

	// 4. Simulate Signals (Human Interaction)
	// We delay the signal slightly to simulate "waiting"
	env.RegisterDelayedCallback(func() {
		signalData := struct {
			Approved   bool
			ReviewerID uuid.UUID
			Reason     string
		}{
			Approved:   true,
			ReviewerID: uuid.New(),
			Reason:     "Looks great",
		}
		env.SignalWorkflow("APPLICATION_DECISION", signalData)
	}, 1*time.Millisecond)

	// 5. Execute Workflow
	env.ExecuteWorkflow(workflow_svc.ParticipantApplicationWorkflow, input)

	// 6. Assertions
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestParticipantApplicationWorkflow_Rejection(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()

	appID := uuid.New()
	userID := uuid.New()
	input := workflow.ApplicationReviewInput{ApplicationID: appID, UserID: userID}
	var a *workflow_svc.Activities

	// Initial notification
	env.OnActivity(a.SendNotification, mock.Anything, userID, "application-received", mock.Anything, "application", appID).Return(nil)

	// Update to Rejected
	env.OnActivity(a.UpdateApplicationStatus, mock.Anything, appID, catalog.AppStatusRejected, mock.Anything, mock.Anything).Return(nil)

	// Rejection Notification
	env.OnActivity(a.SendNotification, mock.Anything, userID, "application-rejected", mock.Anything, "application", appID).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("APPLICATION_DECISION", struct {
			Approved   bool
			ReviewerID uuid.UUID
			Reason     string
		}{Approved: false, Reason: "Missing docs"})
	}, 1*time.Millisecond)

	env.ExecuteWorkflow(workflow_svc.ParticipantApplicationWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}
