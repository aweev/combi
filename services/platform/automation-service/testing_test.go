// internal/services/automation/testing_test.go
package automation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
)

// NOTE: This file also re-uses the setupService and local mocks from service_test.go

func TestTestingService_TestRule(t *testing.T) {
	ctx := context.Background()
	ruleID := uuid.New()
	event := automation.Event{
		Type: "user.created",
		Data: map[string]any{
			"email": "test@example.com",
			"plan":  "premium",
		},
	}

	rule := &automation.Rule{
		ID:      ruleID,
		Trigger: automation.Trigger{Type: "user.created"},
		Conditions: []automation.Condition{
			{Field: "plan", Operator: "eq", Value: "premium"},
		},
		Actions: []automation.Action{
			{
				Type: "log",
				Config: map[string]any{
					"message": "Welcome premium user {{.Data.email}}!",
				},
			},
		},
	}

	t.Run("should simulate a successful execution when conditions are met", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(rule, nil).Once()
		// Mock the evaluator to return a match
		conditionResult := []automation.ConditionResult{{Matched: true}}
		ts.evaluator.On("Evaluate", ctx, rule.Conditions, rule.Logic, event.Data).Return(true, conditionResult, nil).Once()

		result, err := ts.service.TestRule(ctx, ruleID, event)
		require.NoError(t, err)

		assert.True(t, result.WouldTrigger)
		assert.Equal(t, conditionResult, result.ConditionsResult)
		require.Len(t, result.ActionsSimulated, 1)

		// Check the action preview rendering
		preview := result.ActionsSimulated[0].Preview
		renderedConfig := preview["rendered_config"].(map[string]any)
		assert.Equal(t, "Welcome premium user test@example.com!", renderedConfig["message"])
	})

	t.Run("should simulate non-execution when conditions are not met", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(rule, nil).Once()
		// Mock the evaluator to return no match
		conditionResult := []automation.ConditionResult{{Matched: false}}
		ts.evaluator.On("Evaluate", ctx, rule.Conditions, rule.Logic, event.Data).Return(false, conditionResult, nil).Once()

		result, err := ts.service.TestRule(ctx, ruleID, event)
		require.NoError(t, err)

		assert.True(t, result.WouldTrigger)
		assert.Equal(t, conditionResult, result.ConditionsResult)
		assert.Len(t, result.ActionsSimulated, 0) // No actions should be simulated
	})

	t.Run("should simulate non-trigger when trigger filters do not match", func(t *testing.T) {
		ts := setupService(t)

		filteredRule := &automation.Rule{
			ID: ruleID,
			Trigger: automation.Trigger{
				Type:    "user.created",
				Filters: map[string]any{"plan": "enterprise"}, // Event has "premium"
			},
		}
		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(filteredRule, nil).Once()
		// Evaluator should not be called if trigger filters fail

		result, err := ts.service.TestRule(ctx, ruleID, event)
		require.NoError(t, err)
		assert.False(t, result.WouldTrigger)
		assert.Nil(t, result.ConditionsResult)
		assert.Nil(t, result.ActionsSimulated)
	})

	t.Run("should return error if rule not found", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().GetRule(ctx, ruleID).Return(nil, errors.New("not found")).Once()

		_, err := ts.service.TestRule(ctx, ruleID, event)
		require.Error(t, err)
	})
}

func TestTestingService_Validation(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()

	t.Run("ValidateCondition should pass through to evaluator", func(t *testing.T) {
		condition := automation.Condition{Field: "test", Operator: "eq", Value: 1}
		ts.evaluator.On("Validate", ctx, condition).Return(nil).Once()

		err := ts.service.ValidateCondition(ctx, condition)
		assert.NoError(t, err)
	})

	t.Run("ValidateAction should pass through to executor", func(t *testing.T) {
		action := automation.Action{Type: "log"}
		ts.executor.On("Validate", ctx, action).Return(nil).Once()

		err := ts.service.ValidateAction(ctx, action)
		assert.NoError(t, err)
	})
}
