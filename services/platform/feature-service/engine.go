package feature

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"github.com/combi/pkg/domain/feature"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Evaluate(flag *feature.Flag, ctx feature.EvaluationContext) feature.EvaluationResult {
	var varID string
	var ruleID *string

	// 1. Global Disabled check
	if !flag.IsActive || flag.Archived {
		varID = flag.OffVariationID
	} else {
		// 2. Rules
		matchedRule := false
		for _, rule := range flag.TargetingRules {
			if e.matchesRule(rule, ctx) {
				varID = e.resolveRollout(flag.Key, rule.Distribution, ctx.EntityID)
				ruleID = &rule.ID
				matchedRule = true
				break
			}
		}

		if !matchedRule {
			// 3. Default Rule (if present, complex distribution)
			if flag.DefaultRule != nil {
				varID = e.resolveRollout(flag.Key, *flag.DefaultRule, ctx.EntityID)
				ruleID = stringp("default_rule")
			} else {
				// 4. Fallback (Default Variation ID)
				varID = flag.DefaultVariationID
				ruleID = stringp("default")
			}
		}
	}

	// Resolve the actual JSON value for the variation
	var resolvedValue json.RawMessage
	for _, v := range flag.Variations {
		if v.ID == varID {
			resolvedValue = v.Value
			break
		}
	}

	// Fallback to OffVariation logic if ID not found (config error)
	if resolvedValue == nil {
		// Try to find the value corresponding to OffVariationID if it wasn't found in loop
		// This handles cases where OffVariationID points to a valid variation but the loop above missed it (unlikely)
		// OR if the varID computed was invalid.
		// Safety net: Use the raw OffVariation blob if defined on the flag
		if len(flag.OffVariation) > 0 {
			resolvedValue = flag.OffVariation
		} else {
			// Last resort: empty JSON object to prevent nil pointer crashes in clients
			resolvedValue = json.RawMessage("{}")
		}
	}

	return feature.EvaluationResult{
		Key:         flag.Key,
		Value:       resolvedValue,
		VariationID: varID,
		RuleID:      ruleID,
	}
}

func (e *Engine) matchesRule(rule feature.TargetingRule, ctx feature.EvaluationContext) bool {
	for _, cond := range rule.Conditions {
		// Get Attribute Value
		var val any
		switch cond.ContextKey {
		case "entity_id":
			val = ctx.EntityID
		case "entity_type":
			val = ctx.EntityType
		default:
			var ok bool
			val, ok = ctx.Attributes[cond.ContextKey]
			if !ok {
				return false // Missing attribute = no match
			}
		}

		match := e.evaluateCondition(cond, val)
		if cond.Negate {
			match = !match
		}
		if !match {
			return false
		}
	}
	return true
}

func (e *Engine) evaluateCondition(cond feature.Condition, val any) bool {
	sVal := fmt.Sprintf("%v", val)

	// Special handling for array-based operators (IS_IN, IS_NOT_IN)
	// The Condition Value here is expected to be a JSON Array: ["a", "b"]
	if cond.Operator == feature.OpIsIn || cond.Operator == feature.OpIsNotIn {
		var targetList []interface{}
		if err := json.Unmarshal(cond.Value, &targetList); err != nil {
			return false // Invalid config
		}

		found := false
		for _, item := range targetList {
			// Compare as strings for simplicity in this generic engine
			if fmt.Sprintf("%v", item) == sVal {
				found = true
				break
			}
		}

		if cond.Operator == feature.OpIsIn {
			return found
		}
		return !found // IS_NOT_IN
	}

	// For scalar operators, parse the single value
	var targetVal interface{}
	if err := json.Unmarshal(cond.Value, &targetVal); err != nil {
		return false
	}
	sTarget := fmt.Sprintf("%v", targetVal)

	switch cond.Operator {
	// --- String / Equality ---
	case feature.OpIs:
		return sVal == sTarget
	case feature.OpIsNot:
		return sVal != sTarget
	case feature.OpContains:
		return strings.Contains(sVal, sTarget)
	case feature.OpStartsWith:
		return strings.HasPrefix(sVal, sTarget)
	case feature.OpEndsWith:
		return strings.HasSuffix(sVal, sTarget)
	case feature.OpNotContains: // Incase you add this later
		return !strings.Contains(sVal, sTarget)

	// --- Numeric ---
	case feature.OpGt, feature.OpLt, feature.OpGte, feature.OpLte:
		nVal, err1 := strconv.ParseFloat(sVal, 64)
		nTarget, err2 := strconv.ParseFloat(sTarget, 64)

		if err1 == nil && err2 == nil {
			switch cond.Operator {
			case feature.OpGt:
				return nVal > nTarget
			case feature.OpLt:
				return nVal < nTarget
			case feature.OpGte:
				return nVal >= nTarget
			case feature.OpLte:
				return nVal <= nTarget
			}
		}
		// Fallback to lexical comparison if not numbers
		switch cond.Operator {
		case feature.OpGt:
			return sVal > sTarget
		case feature.OpLt:
			return sVal < sTarget
		case feature.OpGte:
			return sVal >= sTarget
		case feature.OpLte:
			return sVal <= sTarget
		}
		return false

	// --- Semantic Versioning ---
	case feature.OpSemverGt, feature.OpSemverLt, feature.OpSemverGte, feature.OpSemverLte, feature.OpSemverEq:
		// Normalize: ensure "v" prefix exists for the parser
		v1 := sVal
		if !strings.HasPrefix(v1, "v") {
			v1 = "v" + v1
		}
		v2 := sTarget
		if !strings.HasPrefix(v2, "v") {
			v2 = "v" + v2
		}

		// Validate
		if !semver.IsValid(v1) || !semver.IsValid(v2) {
			return false
		}

		// Compare returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
		cmp := semver.Compare(v1, v2)

		switch cond.Operator {
		case feature.OpSemverEq:
			return cmp == 0
		case feature.OpSemverGt:
			return cmp == 1
		case feature.OpSemverLt:
			return cmp == -1
		case feature.OpSemverGte:
			return cmp >= 0
		case feature.OpSemverLte:
			return cmp <= 0
		}
		return false

	default:
		return false
	}
}

func (e *Engine) resolveRollout(flagKey string, rollout feature.Rollout, entityID string) string {
	if rollout.VariationID != nil {
		return *rollout.VariationID
	}

	// Weighted Logic
	// Hash(FlagKey + EntityID) -> 0..10000
	hash := hash(flagKey + ":" + entityID)
	bucket := int(hash % 10000)

	sum := 0
	for _, wv := range rollout.Variations {
		sum += wv.Weight
		if bucket < sum {
			return wv.VariationID
		}
	}
	// Should not happen if weights sum to 10000
	return rollout.Variations[0].VariationID
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func stringp(s string) *string {
	return &s
}
