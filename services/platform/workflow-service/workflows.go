package workflow

import (
	"fmt"
	"time"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/workflow"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	sdk "go.temporal.io/sdk/workflow"
)

// Task Queue Constants
const (
	GeneralTaskQueue     = "ngo_GENERAL_QUEUE"
	ApplicationTaskQueue = "ngo_APPLICATION_QUEUE"
)

// === WORKFLOW 1: Content Moderation ===
func ContentModerationWorkflow(ctx sdk.Context, input workflow.ContentModerationInput) error {
	ao := sdk.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = sdk.WithActivityOptions(ctx, ao)
	logger := sdk.GetLogger(ctx)
	logger.Info("Content Moderation Started", "ContentID", input.ContentID)

	var a *Activities

	// 1. Notify Admins
	if err := sdk.ExecuteActivity(ctx, a.SendNotification,
		uuid.Nil, // System notification (no specific recipient user ID for admins group logic here)
		"content-review-needed",
		map[string]interface{}{"contentId": input.ContentID},
		"content", input.ContentID).Get(ctx, nil); err != nil {
		logger.Error("Failed to send moderation notification", "Error", err)
		// Continue even if notification fails, the workflow must persist
	}

	// 2. Wait for Decision Signal
	var decision struct {
		Action string
		Reason *string
	}

	signalChan := sdk.GetSignalChannel(ctx, "MODERATION_DECISION")
	// 7 days to moderate, or auto-reject/escalate
	timer := sdk.NewTimer(ctx, 7*24*time.Hour)

	selector := sdk.NewSelector(ctx)
	selector.AddReceive(signalChan, func(c sdk.ReceiveChannel, more bool) {
		c.Receive(ctx, &decision)
	})
	selector.AddFuture(timer, func(f sdk.Future) {
		decision.Action = "TIMED_OUT"
	})

	selector.Select(ctx)

	// 3. Process Decision
	switch decision.Action {
	case "APPROVE":
		err := sdk.ExecuteActivity(ctx, a.UpdateContentStatus,
			input.ContentID, content.StatusPublished, nil).Get(ctx, nil)
		if err == nil {
			_ = sdk.ExecuteActivity(ctx, a.SendNotification,
				input.AuthorID, "content-approved", nil,
				"content", input.ContentID).Get(ctx, nil)
		}
		return err

	case "REJECT":
		err := sdk.ExecuteActivity(ctx, a.UpdateContentStatus,
			input.ContentID, content.StatusRejected, decision.Reason).Get(ctx, nil)
		if err == nil {
			_ = sdk.ExecuteActivity(ctx, a.SendNotification,
				input.AuthorID, "content-rejected",
				map[string]interface{}{"reason": decision.Reason},
				"content", input.ContentID).Get(ctx, nil)
		}
		return err

	case "TIMED_OUT":
		logger.Warn("Content moderation timed out", "ContentID", input.ContentID)
		// Auto-reject on timeout for safety
		reason := "Moderation timed out"
		return sdk.ExecuteActivity(ctx, a.UpdateContentStatus,
			input.ContentID, content.StatusRejected, &reason).Get(ctx, nil)
	}

	return nil
}

// === WORKFLOW 2: Participant Application ===
func ParticipantApplicationWorkflow(ctx sdk.Context, input workflow.ApplicationReviewInput) error {
	ao := sdk.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = sdk.WithActivityOptions(ctx, ao)
	logger := sdk.GetLogger(ctx)

	var a *Activities

	// 1. Confirm Receipt
	_ = sdk.ExecuteActivity(ctx, a.SendNotification,
		input.UserID, "application-received", nil,
		"application", input.ApplicationID).Get(ctx, nil)

	// 2. Wait for Human Decision
	var decision struct {
		Approved   bool
		ReviewerID uuid.UUID
		Reason     string
	}
	sdk.GetSignalChannel(ctx, "APPLICATION_DECISION").Receive(ctx, &decision)

	if !decision.Approved {
		// REJECT PATH
		_ = sdk.ExecuteActivity(ctx, a.UpdateApplicationStatus,
			input.ApplicationID, catalog.AppStatusRejected,
			&decision.ReviewerID, &decision.Reason).Get(ctx, nil)

		_ = sdk.ExecuteActivity(ctx, a.SendNotification,
			input.UserID, "application-rejected",
			map[string]interface{}{"reason": decision.Reason},
			"application", input.ApplicationID).Get(ctx, nil)
		return nil
	}

	// APPROVE PATH (Saga Pattern)
	logger.Info("Application Approved, starting activation saga", "AppID", input.ApplicationID)

	compensations := []func(sdk.Context){}
	var workflowErr error

	// Defer compensation execution
	defer func() {
		if workflowErr != nil {
			// Create disconnected context so cancellation of parent doesn't stop compensation
			disconnectedCtx, _ := sdk.NewDisconnectedContext(ctx)
			for i := len(compensations) - 1; i >= 0; i-- {
				compensations[i](disconnectedCtx)
			}
		}
	}()

	// Step A: Update Status
	if err := sdk.ExecuteActivity(ctx, a.UpdateApplicationStatus,
		input.ApplicationID, catalog.AppStatusApproved,
		&decision.ReviewerID, nil).Get(ctx, nil); err != nil {
		workflowErr = err
		return err
	}
	// Compensate A: Revert to Pending
	compensations = append(compensations, func(c sdk.Context) {
		_ = sdk.ExecuteActivity(c, a.UpdateApplicationStatus, input.ApplicationID, catalog.AppStatusPending, nil, nil)
	})

	// Step B: Activate Profile (RBAC/User Status)
	if err := sdk.ExecuteActivity(ctx, a.ActivateParticipantProfile, input.UserID).Get(ctx, nil); err != nil {
		workflowErr = err
		return err
	}
	// Compensate B: Deactivate Profile
	compensations = append(compensations, func(c sdk.Context) {
		_ = sdk.ExecuteActivity(c, a.DeactivateParticipantProfile, input.UserID)
	})

	// Step C: Notify Success
	if err := sdk.ExecuteActivity(ctx, a.SendNotification,
		input.UserID, "application-approved", nil,
		"application", input.ApplicationID).Get(ctx, nil); err != nil {
		// If notification fails, we might choose NOT to fail the whole workflow (set workflowErr = nil)
		// But if strict consistency is required, we fail. Here we log and swallow error to avoid rollback.
		logger.Error("Failed to send success notification", "Error", err)
	}

	return nil
}

// === WORKFLOW 3: Grant Application ===
func GrantApplicationWorkflow(ctx sdk.Context, input workflow.GrantApplicationInput) error {
	ao := sdk.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = sdk.WithActivityOptions(ctx, ao)
	var a *Activities

	// Stage 1: Finance Review
	_ = sdk.ExecuteActivity(ctx, a.NotifyRoleGroup, "finance_manager", "grant-review-required",
		map[string]interface{}{"grantId": input.ApplicationID}).Get(ctx, nil)

	var financeDecision string
	sdk.GetSignalChannel(ctx, "FINANCE_DECISION").Receive(ctx, &financeDecision)

	if financeDecision != "APPROVE" {
		return nil // Rejection handling logic...
	}

	// Stage 2: Board Review
	_ = sdk.ExecuteActivity(ctx, a.NotifyRoleGroup, "board_member", "grant-board-review",
		map[string]interface{}{"grantId": input.ApplicationID}).Get(ctx, nil)

	var boardDecision string
	sdk.GetSignalChannel(ctx, "BOARD_DECISION").Receive(ctx, &boardDecision)

	if boardDecision != "APPROVE" {
		return nil
	}

	// --- IDEMPOTENCY FIX ---
	// Generate a deterministic key derived from the WorkflowID.
	// Since WorkflowID is unique per Grant Application attempt, this is safe.
	workflowInfo := sdk.GetInfo(ctx)
	idempotencyKey := fmt.Sprintf("grant-disburse-%s", workflowInfo.WorkflowExecution.ID)

	// Stage 3: Disburse Funds
	// We pass the key to the activity
	err := sdk.ExecuteActivity(ctx, a.FinalizeGrantApproval,
		input.ApplicationID,
		input.Amount,
		input.Currency,
		input.RecipientUserID,
		idempotencyKey,
	).Get(ctx, nil)

	return err
}

// === WORKFLOW 4: Volunteer Onboarding ===
func VolunteerOnboardingWorkflow(ctx sdk.Context, input workflow.VolunteerOnboardingInput) error {
	ao := sdk.ActivityOptions{StartToCloseTimeout: 5 * time.Minute}
	ctx = sdk.WithActivityOptions(ctx, ao)
	var a *Activities

	// 1. Background Check
	var checkResult string
	err := sdk.ExecuteActivity(ctx, a.RunBackgroundCheck, input.UserID).Get(ctx, &checkResult)

	if err != nil || checkResult != "CLEARED" {
		// Handle Failure
		_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.UserID, "bg-check-failed", nil, "user", input.UserID)
		return nil
	}

	// 2. Create Training Project
	var proj project.Project
	if err := sdk.ExecuteActivity(ctx, a.CreateTrainingProject, input.UserID).Get(ctx, &proj); err != nil {
		return err
	}

	// 3. Welcome Notification
	_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.UserID, "volunteer-welcome",
		map[string]interface{}{"projectId": proj.ID},
		"user", input.UserID)

	return nil
}

// === WORKFLOW 5: Partnership Proposal ===
func PartnershipProposalWorkflow(ctx sdk.Context, input workflow.PartnershipProposalInput) error {
	ao := sdk.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = sdk.WithActivityOptions(ctx, ao)
	logger := sdk.GetLogger(ctx)
	var a *Activities

	// 1. Wait for Review
	var decision string
	sdk.GetSignalChannel(ctx, "PROPOSAL_REVIEW").Receive(ctx, &decision)

	if decision != "PROCEED" {
		logger.Info("Proposal declined, sending notification")
		_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.ContactUserID, "partnership-rejected", nil, "user", input.ContactUserID)
		return nil
	}

	// 2. Create Org
	var org organization.Organization
	if err := sdk.ExecuteActivity(ctx, a.CreatePartnerOrganization,
		input.OrgName, input.OrgSlug, input.ContactUserID).Get(ctx, &org); err != nil {
		return err
	}

	// 3. Generate Contract
	var contractURL string
	if err := sdk.ExecuteActivity(ctx, a.GeneratePartnershipContract, org.ID, input.OrgName).Get(ctx, &contractURL); err != nil {
		// Compensation: Cleanup the created Org if contract generation fails
		_ = sdk.ExecuteActivity(ctx, a.ArchiveOrganization, org.ID)
		return err
	}

	// 4. Notify
	_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.ContactUserID, "partnership-contract",
		map[string]interface{}{"url": contractURL, "orgId": org.ID},
		"user", input.ContactUserID)

	// 5. Wait for Signature (with Timeout)
	var signed bool
	selector := sdk.NewSelector(ctx)

	selector.AddReceive(sdk.GetSignalChannel(ctx, "CONTRACT_SIGNED"), func(c sdk.ReceiveChannel, more bool) {
		c.Receive(ctx, &signed)
	})
	// 30 days to sign
	selector.AddFuture(sdk.NewTimer(ctx, 30*24*time.Hour), func(f sdk.Future) {
		signed = false
	})

	selector.Select(ctx)

	if signed {
		logger.Info("Contract signed, activating partnership", "OrgID", org.ID)
		if err := sdk.ExecuteActivity(ctx, a.ActivateOrganization, org.ID).Get(ctx, nil); err != nil {
			return err
		}

		// Final Success Notification
		_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.ContactUserID, "partnership-active", nil, "organization", org.ID)

	} else {
		logger.Warn("Contract signature timed out, archiving org", "OrgID", org.ID)
		_ = sdk.ExecuteActivity(ctx, a.ArchiveOrganization, org.ID).Get(ctx, nil)

		_ = sdk.ExecuteActivity(ctx, a.SendNotification, input.ContactUserID, "partnership-timed-out", nil, "user", input.ContactUserID)
	}

	return nil
}
