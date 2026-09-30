// Ported from api/internal/services/rbac/policy_evaluator_test.go.
package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
)

// policyEvaluatorTestSuite holds the mocked repository for testing policies.
type policyEvaluatorTestSuite struct {
	repo      *rbac_mocks.MockRepository
	evaluator *PolicyEvaluator
}

// setupEvaluator initializes a new test suite.
func setupEvaluator(t *testing.T) policyEvaluatorTestSuite {
	t.Helper()
	var ts policyEvaluatorTestSuite
	ts.repo = rbac_mocks.NewMockRepository(t)
	ts.evaluator = NewPolicyEvaluator(ts.repo)
	return ts
}

func TestPolicyEvaluator_Evaluate(t *testing.T) {
	ctx := context.Background()
	actorID := uuid.New()
	otherID := uuid.New()

	t.Run("should return true if no specific policy is registered for a permission", func(t *testing.T) {
		ts := setupEvaluator(t)
		check := rbac.PermissionCheck{
			PermissionName: core.PermissionName("some:unregistered:permission"),
		}

		allowed, err := ts.evaluator.Evaluate(ctx, check)

		require.NoError(t, err)
		assert.True(t, allowed, "Evaluate should default to true when no policies are registered")
	})

	t.Run("should return true when a registered policy passes", func(t *testing.T) {
		ts := setupEvaluator(t)
		check := rbac.PermissionCheck{
			ActorID:        actorID,
			PermissionName: core.PermissionName("users:profile:read"),
			// Resource context for the isResourceOwner policy
			ResourceContext: map[string]any{"user_id": actorID.String()},
		}

		allowed, err := ts.evaluator.Evaluate(ctx, check)

		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should return false when a registered policy fails", func(t *testing.T) {
		ts := setupEvaluator(t)
		check := rbac.PermissionCheck{
			ActorID:        actorID,
			PermissionName: core.PermissionName("users:profile:read"),
			// Actor is trying to access a resource owned by someone else.
			ResourceContext: map[string]any{"user_id": otherID.String()},
		}

		allowed, err := ts.evaluator.Evaluate(ctx, check)

		require.NoError(t, err)
		assert.False(t, allowed, "Policy check should fail for a non-owner")
	})

	t.Run("should return false if any one of multiple policies fails", func(t *testing.T) {
		ts := setupEvaluator(t)
		permWithTwoPolicies := core.PermissionName("test:multi-policy")

		// Register two policies for the same permission
		// One will pass, one will fail.
		passingPolicy := func(ctx context.Context, repo rbac.Repository, check rbac.PermissionCheck) (bool, error) {
			return true, nil
		}
		failingPolicy := func(ctx context.Context, repo rbac.Repository, check rbac.PermissionCheck) (bool, error) {
			return false, nil
		}
		ts.evaluator.register([]core.PermissionName{permWithTwoPolicies}, passingPolicy)
		ts.evaluator.register([]core.PermissionName{permWithTwoPolicies}, failingPolicy)

		check := rbac.PermissionCheck{PermissionName: permWithTwoPolicies}

		allowed, err := ts.evaluator.Evaluate(ctx, check)

		require.NoError(t, err)
		assert.False(t, allowed, "Evaluation should fail if any policy fails")
	})

	t.Run("should return an error if a policy function itself returns an error", func(t *testing.T) {
		ts := setupEvaluator(t)
		permWithError := core.PermissionName("test:error-policy")
		policyErr := errors.New("policy database error")

		erroringPolicy := func(ctx context.Context, repo rbac.Repository, check rbac.PermissionCheck) (bool, error) {
			return false, policyErr
		}
		ts.evaluator.register([]core.PermissionName{permWithError}, erroringPolicy)

		check := rbac.PermissionCheck{PermissionName: permWithError}
		_, err := ts.evaluator.Evaluate(ctx, check)

		require.Error(t, err)
		assert.ErrorIs(t, err, policyErr, "The underlying error from the policy should be propagated")
	})
}

// Test individual policy functions directly.
func TestPolicyImplementations(t *testing.T) {
	ctx := context.Background()
	actorID := uuid.New()
	resourceID := uuid.New()

	t.Run("isResourceOwner policy", func(t *testing.T) {
		t.Run("should pass when actor ID matches user_id in context", func(t *testing.T) {
			check := rbac.PermissionCheck{
				ActorID:         actorID,
				ResourceContext: map[string]any{"user_id": actorID.String()},
			}
			allowed, err := isResourceOwner(ctx, nil, check) // Repo is not used in this policy
			require.NoError(t, err)
			assert.True(t, allowed)
		})

		t.Run("should fail when actor ID does not match user_id in context", func(t *testing.T) {
			check := rbac.PermissionCheck{
				ActorID:         actorID,
				ResourceContext: map[string]any{"user_id": resourceID.String()},
			}
			allowed, err := isResourceOwner(ctx, nil, check)
			require.NoError(t, err)
			assert.False(t, allowed)
		})

		t.Run("should return error if user_id is missing from context", func(t *testing.T) {
			check := rbac.PermissionCheck{
				ActorID:         actorID,
				ResourceContext: map[string]any{}, // Missing user_id
			}
			_, err := isResourceOwner(ctx, nil, check)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "requires 'user_id' in resource context")
		})

		t.Run("should return error if user_id has invalid format", func(t *testing.T) {
			check := rbac.PermissionCheck{
				ActorID:         actorID,
				ResourceContext: map[string]any{"user_id": "not-a-uuid"},
			}
			_, err := isResourceOwner(ctx, nil, check)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid 'user_id' format")
		})
	})
}
