package feature_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/combi/pkg/domain/feature"
	feature_service "github.com/combi/feature"
)

// Helper for string pointer
func stringp(s string) *string { return &s }

func TestEngine_Evaluate(t *testing.T) {
	engine := feature_service.NewEngine()
	ctx := feature.EvaluationContext{EntityID: "user-123", Attributes: map[string]any{}}

	// Setup a flag with variations
	valTrue := json.RawMessage(`true`)
	valFalse := json.RawMessage(`false`)

	flag := &feature.Flag{
		Key:                "test.flag",
		IsActive:           true,
		DefaultVariationID: "var-false",
		OffVariationID:     "var-false",
		Variations: []feature.Variation{
			{ID: "var-true", Value: valTrue},
			{ID: "var-false", Value: valFalse},
		},
	}

	t.Run("Default Fallback", func(t *testing.T) {
		res := engine.Evaluate(flag, ctx)
		assert.Equal(t, "var-false", res.VariationID)
		assert.JSONEq(t, `false`, string(res.Value))
	})

	t.Run("Rule Matching", func(t *testing.T) {
		rule := feature.TargetingRule{
			ID:       "rule-1",
			Priority: 1,
			Conditions: []feature.Condition{
				{ContextKey: "entity_id", Operator: feature.OpIs, Value: json.RawMessage(`"user-123"`)},
			},
			Distribution: feature.Rollout{VariationID: stringp("var-true")},
		}
		flag.TargetingRules = []feature.TargetingRule{rule}

		res := engine.Evaluate(flag, ctx)
		assert.Equal(t, "var-true", res.VariationID)
		assert.JSONEq(t, `true`, string(res.Value))
		assert.NotNil(t, res.RuleID)
		assert.Equal(t, "rule-1", *res.RuleID)
	})

	t.Run("Inactive Flag returns OffVariation", func(t *testing.T) {
		// Copy flag to avoid mutating shared state if run in parallel
		inactiveFlag := *flag
		inactiveFlag.IsActive = false

		res := engine.Evaluate(&inactiveFlag, ctx)
		assert.Equal(t, "var-false", res.VariationID)
	})
}
