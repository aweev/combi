package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/workflow"
	"github.com/combi/pkg/log"
)

// StarterWorker is a background job processor that listens for 'workflow:start' jobs
// from the internal QueueService and uses the Temporal client to start new workflow executions.
type StarterWorker struct {
	temporalClient client.Client
	service        workflow.Service // Access domain logic via the service interface
	logger         log.Logger
}

// NewStarterWorker creates a new workflow starter worker.
func NewStarterWorker(temporalClient client.Client, service workflow.Service, logger log.Logger) *StarterWorker {
	return &StarterWorker{
		temporalClient: temporalClient,
		service:        service,
		logger:         logger.With(log.String("worker", string(workflow.JobStartWorkflow))),
	}
}

// JobType implements the queue.JobProcessor interface.
func (w *StarterWorker) JobType() core.JobType {
	return workflow.JobStartWorkflow
}

// ProcessJob consumes the queue payload and starts the workflow.
func (w *StarterWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// 1. Unmarshal Payload
	// Using the DTO defined in internal/domain/workflow/definitions.go
	var p workflow.StartWorkflowInput
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid StartWorkflowInput payload: %w", err))
	}

	logger := w.logger.With(
		log.String("job_id", job.ID.String()),
		log.String("workflow_name", p.WorkflowName),
		log.String("business_key", p.BusinessKey),
	)
	logger.Info("processing job to start workflow")

	// 2. Validate Business Key (UUID)
	businessKeyUUID, err := uuid.Parse(p.BusinessKey)
	if err != nil {
		logger.Error("invalid business key in payload, must be a UUID", err)
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid business_key format: %w", err))
	}

	// 3. Prepare Service Parameters
	// Note: We need the Definition ID to call StartWorkflow.
	// Since the Queue payload likely only sends the Name (string), we might need a lookup here.
	// However, for simplicity and to match the previous Service fix, let's assume the Service
	// can handle name-based resolution or we look it up first.

	// FIX: We need to lookup the definition ID by name first because StartWorkflowParams expects an ID.
	// This assumes you added GetDefinitionByName or similar to your Repository/Service.
	// If not, we have to iterate definitions.
	// OPTIMIZATION: In a real app, cache this mapping.
	defs, err := w.service.ListDefinitions(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list definitions to resolve name: %w", err)
	}

	var definitionID uuid.UUID
	found := false
	for _, d := range defs {
		if d.Name == p.WorkflowName {
			definitionID = d.ID
			found = true
			break
		}
	}
	if !found {
		return nil, queue.NewNonRetryableError(fmt.Errorf("unknown workflow definition: %s", p.WorkflowName))
	}

	params := workflow.StartWorkflowParams{
		WorkflowDefinitionID: definitionID,
		SubjectID:            businessKeyUUID,
		Input:                p.Input,
	}

	// 4. Start Workflow via Service
	instance, err := w.service.StartWorkflow(ctx, params)
	if err != nil {
		logger.Error("failed to start temporal workflow", err)
		return nil, fmt.Errorf("failed to start temporal workflow: %w", err)
	}

	logger.Info("successfully started workflow",
		log.String("workflow_id", instance.ID.String()),
		log.String("temporal_run_id", instance.RunID))

	// 5. Return Result
	result := map[string]string{
		"workflowInstanceId": instance.ID.String(),
		"temporalRunId":      instance.RunID,
	}
	resultBytes, _ := json.Marshal(result)
	return resultBytes, nil
}
