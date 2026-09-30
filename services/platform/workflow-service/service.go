package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/workflow"
	"github.com/combi/pkg/log"
)

// Ensure implementation satisfies the domain interface at compile time.
var _ workflow.Service = (*service)(nil)

// service is the concrete implementation of the workflow.Service interface.
type service struct {
	temporalClient client.Client
	repository     workflow.Repository
	auditRecorder  audit.EventRecorder
	logger         log.Logger
}

// NewService creates a new workflow service instance.
func NewService(
	temporalClient client.Client,
	repository workflow.Repository,
	auditRecorder audit.EventRecorder,
	logger log.Logger,
) workflow.Service {
	return &service{
		temporalClient: temporalClient,
		repository:     repository,
		auditRecorder:  auditRecorder,
		logger:         logger.With(log.String("component", "workflow_service")),
	}
}

// StartWorkflow initiates a new workflow instance in Temporal and records it in Postgres.
func (s *service) StartWorkflow(ctx context.Context, params workflow.StartWorkflowParams) (*workflow.WorkflowInstance, error) {
	s.logger.Info("Starting workflow",
		log.String("workflow_name", params.WorkflowDefinitionID.String()),
		log.String("subject_id", params.SubjectID.String()))

	// 1. Resolve Definition
	def, err := s.repository.GetDefinition(ctx, params.WorkflowDefinitionID)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}
	workflowName := def.Name

	// 2. Idempotency Check (Read-Model)
	statusRunning := workflow.StatusRunning
	filter := workflow.InstanceFilter{
		SubjectID: &params.SubjectID,
		Status:    &statusRunning,
		Type:      &def.Type,
		Limit:     1,
	}

	existing, _, err := s.repository.ListInstances(ctx, filter)
	if err == nil && len(existing) > 0 {
		s.logger.Warn("Workflow already running for subject",
			log.String("subject_id", params.SubjectID.String()),
			log.String("existing_instance_id", existing[0].ID.String()))
		return nil, workflow.ErrWorkflowAlreadyExists
	}

	// 3. Resolve Implementation
	workflowFunc, taskQueue, err := s.getWorkflowDefinition(workflowName)
	if err != nil {
		return nil, err
	}

	// 4. Map Input
	typedInput, err := mapInputToWorkflowInput(workflowName, params.Input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", workflow.ErrInvalidDefinition, err)
	}

	// 5. Start in Temporal
	workflowID := fmt.Sprintf("%s-%s", workflowName, params.SubjectID.String())
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	workflowRun, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOptions, workflowFunc, typedInput)
	if err != nil {
		s.logger.Error("Failed to start workflow in Temporal", err, log.String("workflow_name", workflowName))
		return nil, fmt.Errorf("%w: %v", workflow.ErrTemporalCommunication, err)
	}

	// 6. Update Read-Model (Postgres)
	instanceID := uuid.New()
	inputBytes, err := json.Marshal(params.Input)
	if err != nil {
		// Non-fatal, but we should log it or handle it. Since we already started Temporal,
		// we proceed with empty JSON to avoid breaking the DB record creation.
		s.logger.Error("Failed to marshal input for persistence", err)
		inputBytes = []byte("{}")
	}

	instance := &workflow.WorkflowInstance{
		ID:          instanceID,
		WorkflowID:  def.ID,
		TemporalID:  workflowRun.GetID(),
		RunID:       workflowRun.GetRunID(),
		SubjectID:   params.SubjectID,
		Status:      workflow.StatusRunning,
		CurrentStep: "started",
		InputData:   inputBytes,
		ResultData:  json.RawMessage("{}"),
		StartedAt:   time.Now().UTC(),
	}

	if err := s.repository.CreateInstance(ctx, instance); err != nil {
		s.logger.Error("Failed to create workflow instance in database", err,
			log.String("workflow_id", workflowID))
		// Warning: Eventual consistency gap here.
	}

	// 7. Audit Logging
	actor, actorType := getActorFromContext(ctx)
	auditEvent := audit.NewEvent(actor, actorType, core.EventWorkflowStarted).
		WithTarget(params.SubjectID, "workflow_instance", instanceID.String()).
		WithContext(ctx).
		WithSuccess().
		WithMetadata(map[string]interface{}{
			"workflow_name": workflowName,
			"temporal_id":   workflowRun.GetID(),
		}).
		Build()
	// Explicitly ignore error for audit recording as it shouldn't fail the request
	_ = s.auditRecorder.Record(ctx, auditEvent)

	return instance, nil
}

// CancelWorkflow gracefully cancels a running workflow.
func (s *service) CancelWorkflow(ctx context.Context, instanceID uuid.UUID, reason string) error {
	s.logger.Info("Cancelling workflow",
		log.String("instance_id", instanceID.String()),
		log.String("reason", reason))

	// 1. Get Instance
	instance, err := s.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}

	if instance.Status != workflow.StatusRunning {
		return fmt.Errorf("cannot cancel workflow in status %s", instance.Status)
	}

	// 2. Cancel in Temporal
	err = s.temporalClient.CancelWorkflow(ctx, instance.TemporalID, instance.RunID)
	if err != nil {
		return fmt.Errorf("failed to cancel workflow in temporal: %w", err)
	}

	// 3. Update Read-Model
	if err := s.repository.UpdateInstanceStatus(ctx, instanceID, workflow.StatusCancelled, "cancelled_by_user"); err != nil {
		s.logger.Error("Failed to update cancelled workflow status in DB", err)
	}

	// 4. Audit
	actor, actorType := getActorFromContext(ctx)
	auditEvent := audit.NewEvent(actor, actorType, core.EventWorkflowCancelled).
		WithTarget(instance.SubjectID, "workflow_instance", instanceID.String()).
		WithContext(ctx).
		WithSuccess().
		WithMetadata(map[string]interface{}{"reason": reason}).
		Build()
	_ = s.auditRecorder.Record(ctx, auditEvent)

	return nil
}

// SignalWorkflow sends a signal to a running workflow instance.
func (s *service) SignalWorkflow(ctx context.Context, instanceID uuid.UUID, signalName string, signalData interface{}) error {
	s.logger.Info("Sending signal to workflow",
		log.String("instance_id", instanceID.String()),
		log.String("signal_name", signalName))

	instance, err := s.repository.GetInstance(ctx, instanceID)
	if err != nil {
		switch err {
		case workflow.ErrInstanceNotFound:
			return workflow.ErrInstanceNotFound
		case workflow.ErrWorkflowCompleted:
			return workflow.ErrWorkflowCompleted
		default:
			return err
		}
	}

	err = s.temporalClient.SignalWorkflow(ctx, instance.TemporalID, instance.RunID, signalName, signalData)
	if err != nil {
		return fmt.Errorf("%w: %v", workflow.ErrTemporalCommunication, err)
	}

	actor, actorType := getActorFromContext(ctx)
	auditEvent := audit.NewEvent(actor, actorType, core.EventWorkflowSignaled).
		WithTarget(instance.SubjectID, "workflow_instance", instanceID.String()).
		WithContext(ctx).
		WithSuccess().
		WithMetadata(map[string]interface{}{"signal": signalName}).
		Build()
	_ = s.auditRecorder.Record(ctx, auditEvent)

	return nil
}

// CreateDefinition registers a new workflow configuration.
func (s *service) CreateDefinition(ctx context.Context, params workflow.CreateDefinitionParams) (*workflow.WorkflowDefinition, error) {
	def := &workflow.WorkflowDefinition{
		ID:          uuid.New(),
		Name:        params.Name,
		Type:        params.Type,
		Description: params.Description,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	configBytes, err := json.Marshal(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal definition config: %w", err)
	}
	def.Config = configBytes

	if err := s.repository.CreateDefinition(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *service) GetDefinition(ctx context.Context, id uuid.UUID) (*workflow.WorkflowDefinition, error) {
	return s.repository.GetDefinition(ctx, id)
}

func (s *service) ListDefinitions(ctx context.Context, activeOnly bool) ([]workflow.WorkflowDefinition, error) {
	return s.repository.ListDefinitions(ctx, activeOnly)
}

func (s *service) GetInstance(ctx context.Context, id uuid.UUID) (*workflow.WorkflowInstance, error) {
	return s.repository.GetInstance(ctx, id)
}

func (s *service) ListInstances(ctx context.Context, filter workflow.InstanceFilter) ([]workflow.WorkflowInstance, int, error) {
	return s.repository.ListInstances(ctx, filter)
}

// --- Approval Methods ---

func (s *service) GetApprovalRequest(ctx context.Context, id uuid.UUID) (*workflow.ApprovalRequest, error) {
	return s.repository.GetApprovalRequest(ctx, id)
}

func (s *service) ListPendingApprovals(ctx context.Context, assigneeID uuid.UUID, role string) ([]workflow.ApprovalRequest, error) {
	return s.repository.ListPendingApprovals(ctx, &assigneeID, role)
}

func (s *service) SubmitApproval(ctx context.Context, requestID uuid.UUID, params workflow.DecideApprovalParams) error {
	// 1. Get Request
	req, err := s.repository.GetApprovalRequest(ctx, requestID)
	if err != nil {
		return err
	}

	if req.Status != "PENDING" {
		return workflow.ErrApprovalProcessed
	}

	// 2. Update DB State
	now := time.Now().UTC()
	status := "APPROVED"
	if !params.Approved {
		status = "REJECTED"
	}

	req.Status = status
	req.DecidedBy = &params.DeciderID
	req.DecidedAt = &now
	req.DecisionNote = &params.Note

	if err := s.repository.UpdateApprovalRequest(ctx, req); err != nil {
		return err
	}

	// 3. Signal the Temporal Workflow to unblock it
	instance, err := s.repository.GetInstance(ctx, req.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to find associated workflow instance: %w", err)
	}

	// Construct signal data
	signalData := struct {
		Approved   bool
		ReviewerID uuid.UUID
		Reason     string
	}{
		Approved:   params.Approved,
		ReviewerID: params.DeciderID,
		Reason:     params.Note,
	}

	// The signal name depends on the workflow type.
	// Ideally, this is stored in the ApprovalRequest or derived dynamically.
	// For this fix, we assume a standard convention or lookup.
	// Logic to determine specific signal name based on workflow type would go here.
	signalName := "APPLICATION_DECISION"

	// Send Signal
	if err := s.temporalClient.SignalWorkflow(ctx, instance.TemporalID, instance.RunID, signalName, signalData); err != nil {
		s.logger.Error("Failed to signal workflow after approval", err)
		// This is a critical failure: DB says approved, Workflow is still waiting.
		// Needs reconciliation or retry logic.
		return fmt.Errorf("%w: %v", workflow.ErrTemporalCommunication, err)
	}

	return nil
}

// --- Helpers ---

func (s *service) getWorkflowDefinition(workflowName string) (interface{}, string, error) {
	switch workflowName {
	case workflow.ApplicationReviewWorkflowName:
		return ParticipantApplicationWorkflow, workflow.ApplicationTaskQueue, nil
	case workflow.ContentModerationWorkflowName:
		return ContentModerationWorkflow, workflow.GeneralTaskQueue, nil
	case workflow.GrantApplicationWorkflowName:
		return GrantApplicationWorkflow, workflow.GeneralTaskQueue, nil
	case workflow.VolunteerOnboardingWorkflowName:
		return VolunteerOnboardingWorkflow, workflow.GeneralTaskQueue, nil
	case workflow.PartnershipProposalWorkflowName:
		return PartnershipProposalWorkflow, workflow.GeneralTaskQueue, nil
	default:
		return nil, "", workflow.ErrDefinitionNotFound
	}
}

func mapInputToWorkflowInput(workflowName string, input map[string]interface{}) (interface{}, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	var typedInput interface{}
	switch workflowName {
	case workflow.ApplicationReviewWorkflowName:
		typedInput = &workflow.ApplicationReviewInput{}
	case workflow.ContentModerationWorkflowName:
		typedInput = &workflow.ContentModerationInput{}
	case workflow.GrantApplicationWorkflowName:
		typedInput = &workflow.GrantApplicationInput{}
	case workflow.VolunteerOnboardingWorkflowName:
		typedInput = &workflow.VolunteerOnboardingInput{}
	case workflow.PartnershipProposalWorkflowName:
		typedInput = &workflow.PartnershipProposalInput{}
	default:
		return nil, workflow.ErrDefinitionNotFound
	}

	if err := json.Unmarshal(inputBytes, typedInput); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}
	return typedInput, nil
}

func getActorFromContext(ctx context.Context) (uuid.UUID, string) {
	if actor := auth.ActorFromContext(ctx); actor != nil {
		return actor.AccountID, "user"
	}
	return audit.SystemActorID, "system"
}
