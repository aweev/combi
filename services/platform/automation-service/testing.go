// internal/services/automation/testing.go
package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
)

// TestRule simulates the execution of a rule with a sample event without performing
// any actual actions. It provides a detailed result of what would have happened.
func (s *service) TestRule(ctx context.Context, id uuid.UUID, testEvent automation.Event) (*automation.TestResult, error) {
	logger := log.FromContext(ctx).With(log.String("rule_id", id.String()))
	startTime := time.Now()

	// 1. Get the latest version of the rule from the repository.
	rule, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule for testing: %w", err)
	}

	// 2. Prepare the test event.
	if testEvent.ID == uuid.Nil {
		testEvent.ID = uuid.New()
	}
	if testEvent.Timestamp.IsZero() {
		testEvent.Timestamp = time.Now().UTC()
	}
	if testEvent.Data == nil {
		testEvent.Data = make(map[string]any)
	}

	result := &automation.TestResult{
		RuleID: rule.ID,
		Event:  testEvent,
	}
	defer func() {
		result.ExecutionTime = time.Since(startTime)
	}()

	// 3. Simulate Trigger Filter Matching.
	if !s.matchesTriggerFilters(testEvent, rule.Trigger) {
		result.WouldTrigger = false
		logger.Info("test rule simulation: would not trigger due to filter mismatch")
		return result, nil
	}
	result.WouldTrigger = true

	// 4. Simulate Condition Evaluation.
	conditionsMet, conditionsResult, err := s.evaluator.Evaluate(ctx, rule.Conditions, rule.Logic, testEvent.Data)
	result.ConditionsResult = conditionsResult
	if err != nil {
		logger.Error("test rule simulation: condition evaluation failed", err)
		return nil, fmt.Errorf("condition evaluation failed during test: %w", err)
	}

	if !conditionsMet {
		result.ActionsSimulated = []automation.ActionSimulation{} // No actions would run.
		logger.Info("test rule simulation: conditions not met, actions would not execute")
		return result, nil
	}

	// 5. Simulate Action Execution.
	result.ActionsSimulated = s.simulateActions(ctx, rule.Actions, testEvent.Data)
	logger.Info("test rule simulation completed",
		log.Bool("would_trigger", result.WouldTrigger),
		log.Int("conditions_checked", len(result.ConditionsResult)),
		log.Int("actions_simulated", len(result.ActionsSimulated)),
	)

	return result, nil
}

// simulateActions generates previews for what each action would do with the given event data.
func (s *service) simulateActions(ctx context.Context, actions []automation.Action, eventData map[string]any) []automation.ActionSimulation {
	simulations := make([]automation.ActionSimulation, len(actions))
	for i, action := range actions {
		simulations[i] = automation.ActionSimulation{
			Action:       action,
			WouldExecute: true,
			Preview:      s.generateActionPreview(ctx, action, eventData),
		}
	}
	return simulations
}

// generateActionPreview creates a map showing the action's configuration after
// template fields have been rendered with the sample event data.
func (s *service) generateActionPreview(ctx context.Context, action automation.Action, eventData map[string]any) map[string]any {
	preview := make(map[string]any)
	preview["action_type"] = action.Type

	renderedConfig := make(map[string]any)
	for key, value := range action.Config {
		// Use the newly exported RenderTemplate function for consistency.
		rendered, err := RenderTemplate(value, eventData)
		if err != nil {
			renderedConfig[key] = fmt.Sprintf("TEMPLATE_ERROR: %s", err.Error())
		} else {
			renderedConfig[key] = rendered
		}
	}

	preview["rendered_config"] = renderedConfig
	return preview
}

// ValidateCondition is a pass-through to the evaluator's validation logic.
func (s *service) ValidateCondition(ctx context.Context, condition automation.Condition) error {
	return s.evaluator.Validate(ctx, condition)
}

// ValidateAction is a pass-through to the executor's validation logic.
func (s *service) ValidateAction(ctx context.Context, action automation.Action) error {
	return s.executor.Validate(ctx, action)
}
