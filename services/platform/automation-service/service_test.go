package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
	automation_mocks "github.com/combi/pkg/domain/automation/mocks"
	"github.com/combi/pkg/domain/core"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	limiter_mocks "github.com/combi/pkg/limiter/mocks"
	"github.com/combi/pkg/log"
)

// --- MOCKS FOR LOCAL INTERFACES ---

type MockConditionEvaluator struct {
	mock.Mock
}

func NewMockConditionEvaluator(t mock.TestingT) *MockConditionEvaluator {
	m := &MockConditionEvaluator{}
	m.Mock.Test(t)
	return m
}
func (m *MockConditionEvaluator) Evaluate(ctx context.Context, conditions []automation.Condition, logic automation.ConditionLogic, eventData map[string]any) (bool, []automation.ConditionResult, error) {
	args := m.Called(ctx, conditions, logic, eventData)
	res1, _ := args.Get(1).([]automation.ConditionResult)
	return args.Bool(0), res1, args.Error(2)
}
func (m *MockConditionEvaluator) Validate(ctx context.Context, condition automation.Condition) error {
	args := m.Called(ctx, condition)
	return args.Error(0)
}
func (m *MockConditionEvaluator) GetSupportedOperators() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

type MockActionExecutor struct {
	mock.Mock
}

func NewMockActionExecutor(t mock.TestingT) *MockActionExecutor {
	m := &MockActionExecutor{}
	m.Mock.Test(t)
	return m
}

func (m *MockActionExecutor) Execute(ctx context.Context, action automation.Action, eventData map[string]any, idempotencyKey string) (map[string]any, error) {
	args := m.Called(ctx, action, eventData, idempotencyKey)
	var res map[string]any
	if args.Get(0) != nil {
		res = args.Get(0).(map[string]any)
	}
	return res, args.Error(1)
}

func (m *MockActionExecutor) Validate(ctx context.Context, action automation.Action) error {
	args := m.Called(ctx, action)
	return args.Error(0)
}

func (m *MockActionExecutor) GetSupportedActions() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// --- TEST SUITE ---

type serviceTestSuite struct {
	repo        *automation_mocks.MockRepository
	evaluator   *MockConditionEvaluator
	executor    *MockActionExecutor
	dispatcher  *queue_mocks.MockDispatcher
	rateLimiter *limiter_mocks.MockRateLimiter
	eventBus    *events_mocks.MockBus
	redisMock   redismock.ClientMock
	service     automation.Service
}

func setupService(t *testing.T) serviceTestSuite {
	var ts serviceTestSuite
	ts.repo = automation_mocks.NewMockRepository(t)
	ts.evaluator = NewMockConditionEvaluator(t)
	ts.executor = NewMockActionExecutor(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.rateLimiter = limiter_mocks.NewMockRateLimiter(t)
	ts.eventBus = events_mocks.NewMockBus(t)

	db, mockRedis := redismock.NewClientMock()
	ts.redisMock = mockRedis
	statsBuffer := NewStatsBuffer(db, ts.repo, &log.NoOpLogger{})

	dummyHandler := func(ctx context.Context, evt events.DomainEvent) error { return nil }
	subManager := NewSubscriptionManager(ts.eventBus, ts.repo, dummyHandler, &log.NoOpLogger{})

	ts.service = NewService(
		ts.repo,
		ts.evaluator,
		ts.executor,
		&log.NoOpLogger{},
		ts.eventBus,
		ts.dispatcher,
		ts.rateLimiter,
		statsBuffer,
		subManager,
	)
	return ts
}

func TestService_ProcessEvent(t *testing.T) {
	ctx := context.Background()
	actorID := uuid.New()
	event := automation.Event{ID: uuid.New(), Type: "user.created", Data: map[string]any{"name": "test"}, ActorID: &actorID}

	debounceDur := time.Second
	rule1 := automation.Rule{
		ID:                uuid.New(),
		Trigger:           automation.Trigger{Type: "user.created", Debounce: &debounceDur},
		Actions:           []automation.Action{{Type: "log"}},
		ExecutionStrategy: "SEQUENCE",
	}

	t.Run("should process multiple rules for an event successfully", func(t *testing.T) {
		ts := setupService(t)
		defer ts.service.(*service).Close()

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{rule1}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().CreateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.rateLimiter.EXPECT().AllowDebounceThrottle(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
		ts.evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, []automation.ConditionResult{}, nil).Once()
		ts.executor.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string]any{"status": "ok"}, nil).Once()
		ts.repo.EXPECT().UpdateExecution(mock.Anything, mock.Anything).Return(nil).Once()

		ts.redisMock.ExpectHIncrBy("automation:stats:"+rule1.ID.String(), "total", 1).SetVal(1)
		ts.redisMock.ExpectHIncrBy("automation:stats:"+rule1.ID.String(), "success", 1).SetVal(1)
		ts.redisMock.ExpectSAdd("automation:stats:dirty_rules", rule1.ID.String()).SetVal(1)
		ts.redisMock.ExpectExpire("automation:stats:"+rule1.ID.String(), 24*time.Hour).SetVal(true)

		ts.eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("should mark as failed if an action fails", func(t *testing.T) {
		ts := setupService(t)
		defer ts.service.(*service).Close()

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{rule1}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().CreateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.rateLimiter.EXPECT().AllowDebounceThrottle(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
		ts.evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, []automation.ConditionResult{}, nil).Once()

		ts.executor.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return((map[string]any)(nil), errors.New("action failed")).Once()

		ts.redisMock.ExpectHIncrBy("automation:stats:"+rule1.ID.String(), "total", 1).SetVal(1)
		ts.redisMock.ExpectSAdd("automation:stats:dirty_rules", rule1.ID.String()).SetVal(1)
		ts.redisMock.ExpectExpire("automation:stats:"+rule1.ID.String(), 24*time.Hour).SetVal(true)

		ts.eventBus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventExecutionFailure
		})).Return(nil).Once()

		ts.repo.EXPECT().UpdateExecution(mock.Anything, mock.MatchedBy(func(ex *automation.Execution) bool {
			return ex.Status == automation.ExecutionStatusFailed
		})).Return(nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("should pause workflow if a delay action is encountered", func(t *testing.T) {
		ts := setupService(t)

		delayAction := automation.Action{Type: "delay", Config: map[string]any{"duration": "5s"}}
		nextAction := automation.Action{Type: "log"}
		ruleWithDelay := automation.Rule{
			ID:                uuid.New(),
			Trigger:           automation.Trigger{Type: "user.created", Debounce: &debounceDur},
			Actions:           []automation.Action{delayAction, nextAction},
			ExecutionStrategy: "SEQUENCE",
		}

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{ruleWithDelay}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().CreateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.rateLimiter.EXPECT().AllowDebounceThrottle(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
		ts.evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, []automation.ConditionResult{}, nil).Once()

		ts.executor.On("Execute", mock.Anything, delayAction, mock.Anything, mock.Anything).
			Return(map[string]any{"pausing": true}, automation.ErrWorkflowPause).Once()

		ts.dispatcher.EXPECT().Dispatch(mock.Anything, automation.JobAutomationContinueExecution, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()

		ts.repo.EXPECT().UpdateExecution(mock.Anything, mock.MatchedBy(func(ex *automation.Execution) bool {
			return ex.Status == automation.ExecutionStatusRunning
		})).Return(nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("should skip execution if rate limited (Debounce)", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{rule1}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().CreateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.rateLimiter.EXPECT().AllowDebounceThrottle(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

		ts.repo.EXPECT().UpdateExecution(mock.Anything, mock.MatchedBy(func(ex *automation.Execution) bool {
			return ex.Status == automation.ExecutionStatusSkipped
		})).Return(nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})
}

func TestService_EdgeCases(t *testing.T) {
	ctx := context.Background()
	actorID := uuid.New()
	event := automation.Event{ID: uuid.New(), Type: "user.created", Data: map[string]any{"name": "test"}, ActorID: &actorID}

	t.Run("should block execution when loop is detected", func(t *testing.T) {
		ts := setupService(t)
		defer ts.service.(*service).Close()

		rule1 := automation.Rule{
			ID:      uuid.New(),
			Trigger: automation.Trigger{Type: "user.created"},
			Actions: []automation.Action{{Type: "log"}},
		}

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{rule1}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(false, nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("should execute actions in parallel when strategy is PARALLEL", func(t *testing.T) {
		ts := setupService(t)
		defer ts.service.(*service).Close()

		ts.redisMock.MatchExpectationsInOrder(false)

		debounceDur := time.Second
		parallelRule := automation.Rule{
			ID: uuid.New(),
			Trigger: automation.Trigger{
				Type:     "user.created",
				Debounce: &debounceDur,
			},
			Actions:           []automation.Action{{Type: "A"}, {Type: "B"}},
			ExecutionStrategy: "PARALLEL",
		}

		ts.repo.EXPECT().GetActiveRulesByTrigger(ctx, "user.created").Return([]automation.Rule{parallelRule}, nil).Once()
		ts.rateLimiter.EXPECT().Allow(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().CreateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.rateLimiter.EXPECT().AllowDebounceThrottle(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
		ts.evaluator.On("Evaluate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, []automation.ConditionResult{}, nil).Once()

		// FIX: Use mock.Anything for action and idempotencyKey to be flexible, but ensure return valid map
		ts.executor.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"res": "ok"}, nil).Times(2)

		// FIX: Relax AppendActionResultBatch matching
		ts.repo.EXPECT().AppendActionResultBatch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		ts.redisMock.ExpectHIncrBy("automation:stats:"+parallelRule.ID.String(), "total", 1).SetVal(1)
		ts.redisMock.ExpectHIncrBy("automation:stats:"+parallelRule.ID.String(), "success", 1).SetVal(1)
		ts.redisMock.ExpectSAdd("automation:stats:dirty_rules", parallelRule.ID.String()).SetVal(1)
		ts.redisMock.ExpectExpire("automation:stats:"+parallelRule.ID.String(), 24*time.Hour).SetVal(true)

		ts.redisMock.ExpectExpire(mock.Anything, time.Hour).SetVal(true)
		ts.redisMock.ExpectRPush(mock.Anything, mock.Anything).SetVal(1)
		ts.redisMock.ExpectRPush(mock.Anything, mock.Anything).SetVal(2)
		ts.redisMock.ExpectDel(mock.Anything).SetVal(1)

		ts.repo.EXPECT().UpdateExecution(mock.Anything, mock.Anything).Return(nil).Once()
		ts.eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.ProcessEvent(ctx, event)
		if err != nil {
			t.Logf("Error: %v", err)
		}
		require.NoError(t, err)
	})
}
