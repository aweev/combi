package automation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// --- Event Worker ---
type EventWorker struct {
	workerService automation.Service
	logger        log.Logger
}

func NewEventWorker(svc automation.Service, logger log.Logger) *EventWorker {
	return &EventWorker{
		workerService: svc,
		logger:        logger.With(log.String("worker", string(automation.JobAutomationProcessEvent))),
	}
}
func (w *EventWorker) JobType() core.JobType { return automation.JobAutomationProcessEvent }

func (w *EventWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var event automation.Event
	if err := json.Unmarshal(job.Payload, &event); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid event payload: %w", err))
	}
	return nil, w.workerService.ProcessEvent(ctx, event)
}

// --- Action Worker ---
type ActionWorker struct {
	workerService automation.Service
	logger        log.Logger
}

func NewActionWorker(svc automation.Service, logger log.Logger) *ActionWorker {
	return &ActionWorker{
		workerService: svc,
		logger:        logger.With(log.String("worker", string(automation.JobAutomationExecuteAction))),
	}
}
func (w *ActionWorker) JobType() core.JobType { return automation.JobAutomationExecuteAction }

type actionPayload struct {
	ExecutionID uuid.UUID         `json:"execution_id"`
	EventData   map[string]any    `json:"event_data"` // Pass event data to the action
	Action      automation.Action `json:"action"`
}

func (w *ActionWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p actionPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid action payload: %w", err))
	}
	return nil, w.workerService.ExecuteSingleAction(ctx, p.ExecutionID, p.Action, p.EventData)
}

// --- Continuation Worker ---
type ContinuationWorker struct {
	workerService automation.Service
	logger        log.Logger
}

func NewContinuationWorker(svc automation.Service, logger log.Logger) *ContinuationWorker {
	return &ContinuationWorker{
		workerService: svc,
		logger:        logger.With(log.String("worker", string(automation.JobAutomationContinueExecution))),
	}
}
func (w *ContinuationWorker) JobType() core.JobType { return automation.JobAutomationContinueExecution }

type continuationPayload struct {
	ExecutionID  uuid.UUID           `json:"execution_id"`
	EventData    map[string]any      `json:"event_data"`
	ActionsToRun []automation.Action `json:"actions_to_run"`
}

func (w *ContinuationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p continuationPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid continuation payload: %w", err))
	}
	return nil, w.workerService.ContinueExecution(ctx, p.ExecutionID, p.EventData, p.ActionsToRun)
}
