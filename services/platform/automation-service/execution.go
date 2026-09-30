// internal/services/automation/execution.go
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
)

// GetExecution retrieves a single execution record by its ID.
func (s *service) GetExecution(ctx context.Context, id uuid.UUID) (*automation.Execution, error) {
	return s.repo.GetExecution(ctx, id)
}

// ListExecutions retrieves a paginated and filtered list of execution records.
func (s *service) ListExecutions(ctx context.Context, filter automation.ExecutionFilter) ([]automation.Execution, int64, error) {
	return s.repo.ListExecutions(ctx, filter)
}

// RetryExecution retries a failed execution by dispatching the original event back to the queue.
func (s *service) RetryExecution(ctx context.Context, id uuid.UUID) error {
	logger := log.FromContext(ctx).With(log.String("execution_id", id.String()))

	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get execution for retry: %w", err)
	}

	if execution.Status != automation.ExecutionStatusFailed {
		return fmt.Errorf("can only retry executions with status 'FAILED', but status was '%s'", execution.Status)
	}

	// We need the original rule to get the trigger type.
	rule, err := s.repo.GetRule(ctx, execution.RuleID)
	if err != nil {
		// If the rule has been deleted, we cannot retry.
		return fmt.Errorf("failed to get associated rule '%s' for retry: %w", execution.RuleID, err)
	}

	var triggerData map[string]any
	if err := json.Unmarshal(execution.TriggerData, &triggerData); err != nil {
		return fmt.Errorf("cannot retry: failed to unmarshal original trigger data: %w", err)
	}

	event := automation.Event{
		ID:        execution.EventID,
		Type:      rule.Trigger.Type,
		Source:    "retry",
		Timestamp: time.Now().UTC(),
		Data:      triggerData,
	}

	// Dispatching the event is the standard, reliable way to trigger an execution.
	// This ensures the retry goes through the same robust, queue-based workflow.
	if err := s.BroadcastEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to dispatch event for execution retry: %w", err)
	}

	logger.Info("execution retry requested by dispatching event to queue")

	// Optionally, you could update the original execution's status or link it to the new one.
	// For simplicity, we treat it as a fresh attempt.

	return nil
}

// CancelExecution cancels a running execution.
// Note: This is a best-effort operation. If the execution is already in a terminal state
// or the action it's performing doesn't respect context cancellation, it may not stop immediately.
func (s *service) CancelExecution(ctx context.Context, id uuid.UUID) error {
	logger := log.FromContext(ctx).With(log.String("execution_id", id.String()))

	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		return err
	}

	// We can only cancel executions that are not yet in a terminal state.
	switch execution.Status {
	case automation.ExecutionStatusRunning, automation.ExecutionStatusPending:
		// These are valid states to cancel.
	default:
		return fmt.Errorf("can only cancel 'RUNNING' or 'PENDING' executions, but status was '%s'", execution.Status)
	}

	execution.Status = automation.ExecutionStatusCancelled
	execution.CompletedAt = ptr(time.Now().UTC())
	// Duration is calculated from the original start time.
	if execution.CompletedAt != nil {
		execution.Duration = execution.CompletedAt.Sub(execution.StartedAt)
	}
	// Add an error message to explain why it was completed.
	execution.Error = ptr("Execution was cancelled by a user request.")

	if err := s.repo.UpdateExecution(ctx, execution); err != nil {
		return fmt.Errorf("failed to update execution status to cancelled: %w", err)
	}

	logger.Info("execution cancelled successfully")
	return nil
}

// GetRuleStatistics retrieves performance statistics for a specific rule over a given period.
func (s *service) GetRuleStatistics(ctx context.Context, id uuid.UUID, period time.Duration) (*automation.RuleStatistics, error) {
	to := time.Now().UTC()
	from := to.Add(-period)

	stats, err := s.repo.GetRuleExecutionStats(ctx, id, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule statistics: %w", err)
	}

	stats.Period = period
	return stats, nil
}

// GetSystemStatistics retrieves system-wide automation statistics over a given period.
func (s *service) GetSystemStatistics(ctx context.Context, period time.Duration) (*automation.SystemStatistics, error) {
	to := time.Now().UTC()
	from := to.Add(-period)

	stats, err := s.repo.GetSystemStats(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get system statistics: %w", err)
	}

	stats.Period = period
	return stats, nil
}
