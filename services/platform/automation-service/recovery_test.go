package automation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/combi/pkg/domain/automation"
	automation_mocks "github.com/combi/pkg/domain/automation/mocks"
	"github.com/combi/pkg/log"
)

// Simple mock for Service methods needed by Recovery
type MockServiceForRecovery struct {
	mock.Mock
}

func (m *MockServiceForRecovery) ContinueExecution(ctx context.Context, id uuid.UUID, d map[string]any, a []automation.Action) error {
	return m.Called(ctx, id, d, a).Error(0)
}

// Add empty stubs for interface compliance if needed by your compiler,
// or use the generated mock if it includes all methods.
// Assuming we use the full generated mock from the service_test.go context:
// svcMock := &automation_mocks.MockService{} (if you generated Service mock)
// Here we use a struct embedding the full mock if available, or just stubs.

func TestRecoveryService_RecoverStaleExecutions(t *testing.T) {
	ctx := context.Background()
	repo := automation_mocks.NewMockRepository(t)
	// For this test, we mock the Automation Service interface passed to Recovery
	// We need a partial mock or generated mock of automation.Service
	svcMock := automation_mocks.NewMockService(t) // Assuming this exists or generated via mockery

	logger := &log.NoOpLogger{}
	recovery := NewRecoveryService(repo, svcMock, logger)

	ruleID := uuid.New()
	rule := &automation.Rule{
		ID: ruleID,
		Actions: []automation.Action{
			{Type: "delay", Config: map[string]any{"duration": "10s"}},
			{Type: "email"},
		},
	}

	t.Run("crash detected", func(t *testing.T) {
		execID := uuid.New()
		staleExec := automation.Execution{
			ID: execID, RuleID: ruleID, Status: automation.ExecutionStatusRunning,
			StartedAt:     time.Now().Add(-10 * time.Minute),
			UpdatedAt:     time.Now().Add(-10 * time.Minute),
			ActionsResult: []automation.ActionResult{},
		}

		repo.EXPECT().FindStaleExecutions(ctx, mock.Anything, 50).Return([]automation.Execution{staleExec}, nil).Once()

		repo.EXPECT().UpdateExecution(ctx, mock.MatchedBy(func(e *automation.Execution) bool {
			return e.ID == execID && e.Status == automation.ExecutionStatusFailed &&
				*e.Error == "Execution timed out (system crash or stall detected)"
		})).Return(nil).Once()

		recovery.RecoverStaleExecutions(ctx)
	})

	t.Run("zombie pause detected", func(t *testing.T) {
		execID := uuid.New()
		triggerData, _ := json.Marshal(map[string]any{"a": 1})

		staleExec := automation.Execution{
			ID: execID, RuleID: ruleID, Status: automation.ExecutionStatusRunning,
			UpdatedAt:   time.Now().Add(-5 * time.Minute),
			TriggerData: triggerData,
			ActionsResult: []automation.ActionResult{
				{Action: automation.Action{Type: "delay", Config: map[string]any{"duration": "1s"}}, Success: true},
			},
		}

		repo.EXPECT().FindStaleExecutions(ctx, mock.Anything, 50).Return([]automation.Execution{staleExec}, nil).Once()
		repo.EXPECT().GetRule(ctx, ruleID).Return(rule, nil).Once()

		// Expect resume
		svcMock.EXPECT().ContinueExecution(ctx, execID, mock.Anything, []automation.Action{{Type: "email"}}).Return(nil).Once()

		recovery.RecoverStaleExecutions(ctx)
	})
}
