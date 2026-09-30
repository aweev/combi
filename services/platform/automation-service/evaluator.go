// internal/services/automation/evaluator.go
package automation

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
)

var _ automation.ConditionEvaluator = (*conditionEvaluator)(nil)

type conditionEvaluator struct {
	logger log.Logger
}

// NewConditionEvaluator creates a new condition evaluator.
func NewConditionEvaluator(logger log.Logger) automation.ConditionEvaluator {
	return &conditionEvaluator{
		logger: logger,
	}
}

func (e *conditionEvaluator) Evaluate(
	ctx context.Context,
	conditions []automation.Condition,
	logic automation.ConditionLogic,
	eventData map[string]any,
) (bool, []automation.ConditionResult, error) {
	if len(conditions) == 0 {
		return true, []automation.ConditionResult{}, nil // A rule with no conditions is always met.
	}

	results := make([]automation.ConditionResult, 0, len(conditions))

	for _, condition := range conditions {
		// Evaluate a single condition
		match, err := e.evaluateSingle(ctx, condition, eventData)

		// Build the detailed result for this condition
		result := automation.ConditionResult{
			Condition: condition,
			Matched:   match,
		}
		if err != nil {
			// An evaluation error is a hard failure, not just a non-match.
			// We record the error in the result and stop evaluation.
			errMsg := err.Error()
			result.Error = &errMsg
			results = append(results, result)
			return false, results, fmt.Errorf("evaluation failed for field '%s': %w", condition.Field, err)
		}
		results = append(results, result)

		// Apply logic with short-circuiting
		if logic == automation.ConditionLogicAND {
			if !match {
				// For AND, the first non-match means we're done.
				// We need to evaluate the rest just to populate the results slice for visibility.
				// This part is a bit tricky: do we stop or show all results? Showing all is better for debugging.
				// Let's adjust the logic slightly to always evaluate all, then determine the final result.
				// (The original logic was better for performance, let's revert to that with a note)

				// Sticking with performant short-circuiting:
				return false, results, nil
			}
		} else { // OR logic
			if match {
				// For OR, the first match means we're done.
				return true, results, nil
			}
		}
	}

	// If we've looped through all conditions without short-circuiting:
	// - For AND, it means all were true.
	// - For OR, it means none were true.
	if logic == automation.ConditionLogicAND {
		return true, results, nil
	}
	return false, results, nil
}

// evaluateSingle evaluates one condition against the event data.
func (e *conditionEvaluator) evaluateSingle(ctx context.Context, condition automation.Condition, eventData map[string]any) (bool, error) {
	// Extract the actual value from the event data using the field path.
	actualValue, err := e.extractValue(eventData, condition.Field)
	// For 'exists'/'not_exists', a "field not found" error is part of the logic, not a failure.
	if err != nil && condition.Operator != "exists" && condition.Operator != "not_exists" {
		return false, fmt.Errorf("failed to extract field '%s': %w", condition.Field, err)
	}

	// Perform comparison based on the operator.
	result, err := e.compare(actualValue, condition.Operator, condition.Value)
	if err != nil {
		return false, fmt.Errorf("comparison failed for field '%s' with operator '%s': %w", condition.Field, condition.Operator, err)
	}

	// Apply negation if specified.
	if condition.Negate {
		return !result, nil
	}

	return result, nil
}

// Validate checks if a condition's configuration is valid.
func (e *conditionEvaluator) Validate(ctx context.Context, condition automation.Condition) error {
	if condition.Field == "" {
		return fmt.Errorf("condition field is required")
	}
	if condition.Operator == "" {
		return fmt.Errorf("condition operator is required")
	}

	supportedOperators := e.GetSupportedOperators()
	found := false
	for _, op := range supportedOperators {
		if op == condition.Operator {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unsupported operator: '%s'. Supported operators are: %v", condition.Operator, supportedOperators)
	}
	return nil
}

// GetSupportedOperators returns a list of all supported comparison operators.
func (e *conditionEvaluator) GetSupportedOperators() []string {
	return []string{
		"eq", "ne", "gt", "gte", "lt", "lte",
		"in", "not_in",
		"contains", "not_contains", "starts_with", "ends_with",
		"regex", "not_regex",
		"exists", "not_exists",
		"empty", "not_empty",
	}
}

// extractValue safely navigates a nested map[string]any using a dot-separated path.
func (e *conditionEvaluator) extractValue(data map[string]any, fieldPath string) (any, error) {
	parts := strings.Split(fieldPath, ".")
	var current any = data

	for i, part := range parts {
		currentMap, ok := current.(map[string]any)
		if !ok {
			// This error triggers if ANY segment in the path is not a map.
			// It's the only error check we need for traversal.
			traversedPath := strings.Join(parts[:i], ".")
			return nil, fmt.Errorf("cannot traverse non-map type at segment '%s' in path '%s'", traversedPath, fieldPath)
		}

		value, exists := currentMap[part]
		if !exists {
			return nil, fmt.Errorf("field not found: '%s'", fieldPath)
		}
		current = value
	}
	return current, nil
}

// compare performs the actual comparison between the extracted value and the expected value.
func (e *conditionEvaluator) compare(actual any, operator string, expected any) (bool, error) {
	switch operator {
	case "eq":
		return e.compareEqual(actual, expected), nil
	case "ne":
		return !e.compareEqual(actual, expected), nil
	case "gt":
		return e.compareNumeric(actual, expected, func(a, b float64) bool { return a > b })
	case "gte":
		return e.compareNumeric(actual, expected, func(a, b float64) bool { return a >= b })
	case "lt":
		return e.compareNumeric(actual, expected, func(a, b float64) bool { return a < b })
	case "lte":
		return e.compareNumeric(actual, expected, func(a, b float64) bool { return a <= b })
	case "in":
		return e.compareIn(actual, expected, false)
	case "not_in":
		return e.compareIn(actual, expected, true)
	case "contains":
		return e.compareContains(actual, expected, false)
	case "not_contains":
		return e.compareContains(actual, expected, true)
	case "starts_with":
		return e.compareStringFunc(actual, expected, strings.HasPrefix)
	case "ends_with":
		return e.compareStringFunc(actual, expected, strings.HasSuffix)
	case "regex":
		return e.compareRegex(actual, expected, false)
	case "not_regex":
		return e.compareRegex(actual, expected, true)
	case "exists":
		return actual != nil, nil
	case "not_exists":
		return actual == nil, nil
	case "empty":
		return e.isEmpty(actual), nil
	case "not_empty":
		return !e.isEmpty(actual), nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// --- Comparison Helper Functions ---

func (e *conditionEvaluator) compareEqual(actual, expected any) bool {
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}
	// Try a direct comparison first. This handles strings, bools, etc.
	if reflect.DeepEqual(actual, expected) {
		return true
	}
	// Handle numeric type coercion (e.g., int 10 should equal float64 10.0).
	actualFloat, errA := e.toFloat64(actual)
	expectedFloat, errB := e.toFloat64(expected)
	if errA == nil && errB == nil {
		return actualFloat == expectedFloat
	}
	return false
}

func (e *conditionEvaluator) compareNumeric(actual, expected any, op func(a, b float64) bool) (bool, error) {
	actualFloat, err := e.toFloat64(actual)
	if err != nil {
		return false, err
	}
	expectedFloat, err := e.toFloat64(expected)
	if err != nil {
		return false, err
	}
	return op(actualFloat, expectedFloat), nil
}

func (e *conditionEvaluator) compareIn(actual, expected any, negate bool) (bool, error) {
	expectedVal := reflect.ValueOf(expected)
	if expectedVal.Kind() != reflect.Slice {
		return false, fmt.Errorf("expected value must be an array/slice for 'in'/'not_in' operator, got %T", expected)
	}
	for i := 0; i < expectedVal.Len(); i++ {
		if e.compareEqual(actual, expectedVal.Index(i).Interface()) {
			return !negate, nil // Found a match. If not negated, true. If negated, false.
		}
	}
	return negate, nil // No match found. If not negated, false. If negated, true.
}

func (e *conditionEvaluator) compareContains(actual, expected any, negate bool) (bool, error) {
	actualStr, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("actual value must be a string for 'contains' operator")
	}
	expectedStr, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("expected value must be a string for 'contains' operator")
	}
	result := strings.Contains(actualStr, expectedStr)
	return result != negate, nil
}

func (e *conditionEvaluator) compareStringFunc(actual, expected any, f func(s, substr string) bool) (bool, error) {
	actualStr, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("actual value must be a string for this operator")
	}
	expectedStr, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("expected value must be a string for this operator")
	}
	return f(actualStr, expectedStr), nil
}

func (e *conditionEvaluator) compareRegex(actual, expected any, negate bool) (bool, error) {
	actualStr, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("actual value must be a string for 'regex' operator")
	}
	expectedStr, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("expected value (pattern) must be a string for 'regex' operator")
	}

	matched, err := regexp.MatchString(expectedStr, actualStr)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern '%s': %w", expectedStr, err)
	}
	return matched != negate, nil
}

func (e *conditionEvaluator) isEmpty(value any) bool {
	if value == nil {
		return true
	}
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	}
	return false
}

func (e *conditionEvaluator) toFloat64(value any) (float64, error) {
	if value == nil {
		return 0, fmt.Errorf("cannot convert nil to a number")
	}
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(val.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(val.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return val.Float(), nil
	case reflect.String:
		// Also handle ISO 8601 time strings by converting to Unix timestamp
		if t, err := time.Parse(time.RFC3339, val.String()); err == nil {
			return float64(t.Unix()), nil
		}
	}
	return 0, fmt.Errorf("cannot convert type %T to a number", value)
}
