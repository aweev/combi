// internal/services/automation/workers_test.go
package automation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// --- Mock for the local automation.Service interface ---
type MockWorkerService struct {
	mock.Mock
}

func NewMockWorkerService(t mock.TestingT) *MockWorkerService {
	m := &MockWorkerService{}
	m.Mock.Test(t)
	return m
}

// Implement only the methods used by the workers
func (m *MockWorkerService) ProcessEvent(ctx context.Context, event automation.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
func (m *MockWorkerService) ExecuteSingleAction(ctx context.Context, executionID uuid.UUID, action automation.Action, eventData map[string]any) error {
	args := m.Called(ctx, executionID, action, eventData)
	return args.Error(0)
}
func (m *MockWorkerService) ContinueExecution(ctx context.Context, executionID uuid.UUID, eventData map[string]any, actionsToRun []automation.Action) error {
	args := m.Called(ctx, executionID, eventData, actionsToRun)
	return args.Error(0)
}

// Add dummy implementations for other Service methods to satisfy the interface
func (m *MockWorkerService) CreateRule(ctx context.Context, params automation.CreateRuleParams) (*automation.Rule, error) {
	return nil, nil
}
func (m *MockWorkerService) GetRule(ctx context.Context, id uuid.UUID) (*automation.Rule, error) {
	return nil, nil
}
func (m *MockWorkerService) UpdateRule(ctx context.Context, ruleID uuid.UUID, params automation.UpdateRuleParams) (*automation.Rule, error) {
	return nil, nil
}
func (m *MockWorkerService) DeleteRule(ctx context.Context, id uuid.UUID) error { return nil }
func (m *MockWorkerService) ListRules(ctx context.Context, filter automation.RuleFilter) ([]automation.Rule, int64, error) {
	return nil, 0, nil
}
func (m *MockWorkerService) EnableRule(ctx context.Context, id uuid.UUID) error  { return nil }
func (m *MockWorkerService) DisableRule(ctx context.Context, id uuid.UUID) error { return nil }
func (m *MockWorkerService) GetExecution(ctx context.Context, id uuid.UUID) (*automation.Execution, error) {
	return nil, nil
}
func (m *MockWorkerService) ListExecutions(ctx context.Context, filter automation.ExecutionFilter) ([]automation.Execution, int64, error) {
	return nil, 0, nil
}
func (m *MockWorkerService) RetryExecution(ctx context.Context, id uuid.UUID) error  { return nil }
func (m *MockWorkerService) CancelExecution(ctx context.Context, id uuid.UUID) error { return nil }
func (m *MockWorkerService) TestRule(ctx context.Context, id uuid.UUID, testEvent automation.Event) (*automation.TestResult, error) {
	return nil, nil
}
func (m *MockWorkerService) ValidateCondition(ctx context.Context, condition automation.Condition) error {
	return nil
}
func (m *MockWorkerService) ValidateAction(ctx context.Context, action automation.Action) error {
	return nil
}
func (m *MockWorkerService) GetRuleStatistics(ctx context.Context, id uuid.UUID, period time.Duration) (*automation.RuleStatistics, error) {
	return nil, nil
}
func (m *MockWorkerService) GetSystemStatistics(ctx context.Context, period time.Duration) (*automation.SystemStatistics, error) {
	return nil, nil
}
func (m *MockWorkerService) ExecuteAction(ctx context.Context, actionType string, actionConfig map[string]interface{}) error {
	return nil
}

func TestEventWorker(t *testing.T) {
	ctx := context.Background()
	event := automation.Event{ID: uuid.New(), Type: "user.created"}
	payload, _ := json.Marshal(event)
	job := &queue.Job{Payload: payload}

	t.Run("should call service ProcessEvent on valid job", func(t *testing.T) {
		mockSvc := NewMockWorkerService(t)
		worker := NewEventWorker(mockSvc, &log.NoOpLogger{})

		mockSvc.On("ProcessEvent", ctx, mock.MatchedBy(func(e automation.Event) bool {
			return e.ID == event.ID
		})).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("should return non-retryable error on invalid payload", func(t *testing.T) {
		mockSvc := NewMockWorkerService(t)
		worker := NewEventWorker(mockSvc, &log.NoOpLogger{})
		invalidJob := &queue.Job{Payload: []byte("invalid")}

		_, err := worker.ProcessJob(ctx, invalidJob)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable))
	})
}

func TestActionWorker(t *testing.T) {
	ctx := context.Background()
	p := actionPayload{ExecutionID: uuid.New(), Action: automation.Action{Type: "log"}}
	payload, _ := json.Marshal(p)
	job := &queue.Job{Payload: payload}

	t.Run("should call service ExecuteSingleAction on valid job", func(t *testing.T) {
		mockSvc := NewMockWorkerService(t)
		worker := NewActionWorker(mockSvc, &log.NoOpLogger{})

		mockSvc.On("ExecuteSingleAction", ctx, p.ExecutionID, p.Action, p.EventData).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})
}

func TestContinuationWorker(t *testing.T) {
	ctx := context.Background()
	p := continuationPayload{ExecutionID: uuid.New(), ActionsToRun: []automation.Action{{Type: "log"}}}
	payload, _ := json.Marshal(p)
	job := &queue.Job{Payload: payload}

	t.Run("should call service ContinueExecution on valid job", func(t *testing.T) {
		mockSvc := NewMockWorkerService(t)
		worker := NewContinuationWorker(mockSvc, &log.NoOpLogger{})

		mockSvc.On("ContinueExecution", ctx, p.ExecutionID, p.EventData, p.ActionsToRun).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})
}
