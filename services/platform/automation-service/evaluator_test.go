// internal/services/automation/evaluator_test.go
package automation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
)

func TestConditionEvaluator(t *testing.T) {
	evaluator := NewConditionEvaluator(&log.NoOpLogger{})
	ctx := context.Background()

	eventData := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"age":   30,
			"tags":  []any{"vip", "beta-tester"},
			"email": "alice@example.com",
		},
		"order": map[string]any{
			"total":    150.75,
			"currency": "USD",
			"items":    3,
		},
		"empty_string": "",
		"null_field":   nil,
	}

	testCases := []struct {
		name       string
		conditions []automation.Condition
		logic      automation.ConditionLogic
		expected   bool
	}{
		// --- Basic Equality & Comparison ---
		{
			name:       "should match equal string",
			conditions: []automation.Condition{{Field: "user.name", Operator: "eq", Value: "Alice"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should not match unequal string",
			conditions: []automation.Condition{{Field: "user.name", Operator: "eq", Value: "Bob"}},
			logic:      automation.ConditionLogicAND,
			expected:   false,
		},
		{
			name:       "should match with number type coercion",
			conditions: []automation.Condition{{Field: "user.age", Operator: "eq", Value: 30.0}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should match greater than",
			conditions: []automation.Condition{{Field: "order.total", Operator: "gt", Value: 100}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should not match less than",
			conditions: []automation.Condition{{Field: "user.age", Operator: "lt", Value: 25}},
			logic:      automation.ConditionLogicAND,
			expected:   false,
		},
		// --- String Operations ---
		{
			name:       "should match contains",
			conditions: []automation.Condition{{Field: "user.name", Operator: "contains", Value: "lic"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should match starts_with",
			conditions: []automation.Condition{{Field: "user.email", Operator: "starts_with", Value: "alice@"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should match regex",
			conditions: []automation.Condition{{Field: "user.email", Operator: "regex", Value: `^a.*@`}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		// --- Slice/Array Operations ---
		{
			name:       "should match 'in' for slice",
			conditions: []automation.Condition{{Field: "order.currency", Operator: "in", Value: []any{"USD", "EUR"}}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should not match 'not_in' for slice",
			conditions: []automation.Condition{{Field: "order.currency", Operator: "not_in", Value: []any{"USD", "EUR"}}},
			logic:      automation.ConditionLogicAND,
			expected:   false,
		},
		// --- Existence & Emptiness ---
		{
			name:       "should match 'exists' for existing field",
			conditions: []automation.Condition{{Field: "order.items", Operator: "exists"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should not match 'exists' for non-existing field",
			conditions: []automation.Condition{{Field: "user.address", Operator: "exists"}},
			logic:      automation.ConditionLogicAND,
			expected:   false,
		},
		{
			name:       "should match 'not_exists' for non-existing field",
			conditions: []automation.Condition{{Field: "user.address", Operator: "not_exists"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should match 'empty' for empty string",
			conditions: []automation.Condition{{Field: "empty_string", Operator: "empty"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		{
			name:       "should match 'not_empty' for non-empty string",
			conditions: []automation.Condition{{Field: "user.name", Operator: "not_empty"}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		// --- Negation ---
		{
			name:       "should correctly negate a true condition",
			conditions: []automation.Condition{{Field: "user.name", Operator: "eq", Value: "Alice", Negate: true}},
			logic:      automation.ConditionLogicAND,
			expected:   false,
		},
		{
			name:       "should correctly negate a false condition",
			conditions: []automation.Condition{{Field: "user.name", Operator: "eq", Value: "Bob", Negate: true}},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
		// --- Logic (AND/OR) ---
		{
			name: "AND logic should pass when all are true",
			conditions: []automation.Condition{
				{Field: "user.age", Operator: "gte", Value: 30},
				{Field: "order.total", Operator: "gt", Value: 150},
			},
			logic:    automation.ConditionLogicAND,
			expected: true,
		},
		{
			name: "AND logic should fail when one is false",
			conditions: []automation.Condition{
				{Field: "user.age", Operator: "gte", Value: 30},
				{Field: "order.total", Operator: "gt", Value: 200},
			},
			logic:    automation.ConditionLogicAND,
			expected: false,
		},
		{
			name: "OR logic should pass when one is true",
			conditions: []automation.Condition{
				{Field: "user.name", Operator: "eq", Value: "Bob"},
				{Field: "order.currency", Operator: "eq", Value: "USD"},
			},
			logic:    automation.ConditionLogicOR,
			expected: true,
		},
		{
			name: "OR logic should fail when all are false",
			conditions: []automation.Condition{
				{Field: "user.name", Operator: "eq", Value: "Bob"},
				{Field: "order.currency", Operator: "eq", Value: "EUR"},
			},
			logic:    automation.ConditionLogicOR,
			expected: false,
		},
		// --- Special Cases ---
		{
			name:       "should return true for empty condition set",
			conditions: []automation.Condition{},
			logic:      automation.ConditionLogicAND,
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match, results, err := evaluator.Evaluate(ctx, tc.conditions, tc.logic, eventData)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, match)
			// This assertion is tricky with short-circuiting, so we can relax it or be more specific.
			// For now, let's just ensure it's not longer than the number of conditions.
			assert.LessOrEqual(t, len(results), len(tc.conditions))
		})
	}

	// --- Error Condition Tests ---

	t.Run("should return error for invalid traversal path", func(t *testing.T) {
		// Here, 'user.name' exists but is a string, not a map, so we can't traverse it.
		conditions := []automation.Condition{{Field: "user.name.first", Operator: "eq", Value: "Alice"}}
		_, _, err := evaluator.Evaluate(ctx, conditions, automation.ConditionLogicAND, eventData)
		require.Error(t, err)

		// ADJUSTED ASSERTION: Check for the new, consistent error message.
		// The path traversed before failure was "user.name".
		assert.Contains(t, err.Error(), "cannot traverse non-map type at segment 'user.name'")
	})

	t.Run("should return error for missing intermediate field path", func(t *testing.T) {
		// 'user.address' does not exist at all.
		conditions := []automation.Condition{{Field: "user.address.city", Operator: "eq", Value: "New York"}}
		_, _, err := evaluator.Evaluate(ctx, conditions, automation.ConditionLogicAND, eventData)
		require.Error(t, err)

		// ADJUSTED ASSERTION: The simpler function will now just report "field not found".
		assert.Contains(t, err.Error(), "field not found: 'user.address.city'")
	})

	t.Run("should return error for invalid regex pattern", func(t *testing.T) {
		conditions := []automation.Condition{{Field: "user.email", Operator: "regex", Value: `[`}} // Invalid regex
		_, _, err := evaluator.Evaluate(ctx, conditions, automation.ConditionLogicAND, eventData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})

	t.Run("should validate supported operators", func(t *testing.T) {
		err := evaluator.Validate(ctx, automation.Condition{Field: "test", Operator: "eq", Value: "a"})
		assert.NoError(t, err)

		err = evaluator.Validate(ctx, automation.Condition{Field: "test", Operator: "invalid_op", Value: "a"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operator")
	})
}
