// internal/services/automation/execution_test.go
package automation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
)

func TestExecutionService_RetryExecution(t *testing.T) {
	ctx := context.Background()
	executionID := uuid.New()
	ruleID := uuid.New()
	eventID := uuid.New()
	triggerData, _ := json.Marshal(map[string]any{"key": "value"})

	failedExecution := &automation.Execution{
		ID:          executionID,
		RuleID:      ruleID,
		EventID:     eventID,
		Status:      automation.ExecutionStatusFailed,
		TriggerData: triggerData,
	}

	rule := &automation.Rule{
		ID:      ruleID,
		Trigger: automation.Trigger{Type: "user.created"},
	}

	t.Run("should succeed for a failed execution", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().GetExecution(ctx, executionID).Return(failedExecution, nil).Once()
		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(rule, nil).Once()

		ts.dispatcher.EXPECT().Dispatch(
			ctx,
			mock.AnythingOfType("core.JobType"), // More specific type
			mock.AnythingOfType("json.RawMessage"),
			mock.AnythingOfType("queue.DispatchOption"),
		).Return(nil, nil).Once()

		err := ts.service.RetryExecution(ctx, executionID)
		require.NoError(t, err)
	})

	t.Run("should fail for a non-failed execution", func(t *testing.T) {
		ts := setupService(t)
		completedExecution := &automation.Execution{Status: automation.ExecutionStatusCompleted}
		ts.repo.EXPECT().GetExecution(ctx, executionID).Return(completedExecution, nil).Once()

		err := ts.service.RetryExecution(ctx, executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can only retry executions with status 'FAILED'")
	})

	t.Run("should fail if the original rule is deleted", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().GetExecution(ctx, executionID).Return(failedExecution, nil).Once()
		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(nil, automation.ErrRuleNotFound).Once()

		err := ts.service.RetryExecution(ctx, executionID)
		require.Error(t, err)
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})
}

func TestExecutionService_CancelExecution(t *testing.T) {
	ctx := context.Background()
	executionID := uuid.New()

	t.Run("should succeed for a running execution", func(t *testing.T) {
		ts := setupService(t)
		runningExecution := &automation.Execution{ID: executionID, Status: automation.ExecutionStatusRunning, StartedAt: time.Now()}
		ts.repo.EXPECT().GetExecution(ctx, executionID).Return(runningExecution, nil).Once()
		ts.repo.EXPECT().UpdateExecution(ctx, mock.MatchedBy(func(ex *automation.Execution) bool {
			return ex.Status == automation.ExecutionStatusCancelled && ex.Error != nil && ex.CompletedAt != nil
		})).Return(nil).Once()

		err := ts.service.CancelExecution(ctx, executionID)
		require.NoError(t, err)
	})

	t.Run("should fail for a completed execution", func(t *testing.T) {
		ts := setupService(t)
		completedExecution := &automation.Execution{Status: automation.ExecutionStatusCompleted}
		ts.repo.EXPECT().GetExecution(ctx, executionID).Return(completedExecution, nil).Once()

		err := ts.service.CancelExecution(ctx, executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can only cancel 'RUNNING' or 'PENDING' executions")
	})
}

func TestExecutionService_GetStatistics(t *testing.T) {
	ctx := context.Background()
	ruleID := uuid.New()
	period := 24 * time.Hour

	t.Run("GetRuleStatistics should call repository with correct time range", func(t *testing.T) {
		ts := setupService(t)
		expectedStats := &automation.RuleStatistics{TotalExecutions: 10}
		ts.repo.EXPECT().GetRuleExecutionStats(ctx, ruleID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(expectedStats, nil).Once()

		stats, err := ts.service.GetRuleStatistics(ctx, ruleID, period)
		require.NoError(t, err)
		assert.Equal(t, int64(10), stats.TotalExecutions)
		assert.Equal(t, period, stats.Period)
	})

	t.Run("GetSystemStatistics should call repository with correct time range", func(t *testing.T) {
		ts := setupService(t)
		expectedStats := &automation.SystemStatistics{TotalExecutions: 100}
		ts.repo.EXPECT().GetSystemStats(ctx, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(expectedStats, nil).Once()

		stats, err := ts.service.GetSystemStatistics(ctx, period)
		require.NoError(t, err)
		assert.Equal(t, int64(100), stats.TotalExecutions)
		assert.Equal(t, period, stats.Period)
	})
}
