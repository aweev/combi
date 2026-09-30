// access-control-service (package acl) — ported from
// api/internal/services/rbac/policy_evaluator.go.
//
// PolicyEvaluator handles attribute-based access control (ABAC) checks on
// top of the RBAC permission set.
package acl

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
)

// PolicyCheckFunc is the signature for a function that performs an attribute-based check.
type PolicyCheckFunc func(ctx context.Context, repo rbac.Repository, check rbac.PermissionCheck) (bool, error)

// PolicyEvaluator handles attribute-based access control (ABAC) checks.
type PolicyEvaluator struct {
	policies map[core.PermissionName][]PolicyCheckFunc
	repo     rbac.Repository
}

func NewPolicyEvaluator(repo rbac.Repository) *PolicyEvaluator {
	evaluator := &PolicyEvaluator{
		policies: make(map[core.PermissionName][]PolicyCheckFunc),
		repo:     repo,
	}

	// --- Register All ABAC Policies Here ---
	// Example: Any action on a user's own profile requires ownership.
	evaluator.register([]core.PermissionName{"users:profile:read", "users:profile:update"}, isResourceOwner)

	return evaluator
}

func (p *PolicyEvaluator) register(permissions []core.PermissionName, checkFunc PolicyCheckFunc) {
	for _, perm := range permissions {
		p.policies[perm] = append(p.policies[perm], checkFunc)
	}
}

// Evaluate checks if an action is allowed based on registered policies.
// This is only called AFTER the main RBAC check has already passed.
func (p *PolicyEvaluator) Evaluate(ctx context.Context, check rbac.PermissionCheck) (bool, error) {
	policiesToCheck, ok := p.policies[check.PermissionName]
	if !ok {
		return true, nil // No specific policy, allow based on RBAC check.
	}

	for _, policyFunc := range policiesToCheck {
		allowed, err := policyFunc(ctx, p.repo, check)
		if err != nil {
			// Log the context error but return 'false' to fail closed securely.
			// We wrap the error so the upper layer knows it was a context issue.
			return false, fmt.Errorf("ABAC policy execution error: %w", err)
		}
		if !allowed {
			// Policy explicitly denied access.
			return false, nil
		}
	}

	return true, nil
}

const AdminRoleSlug = "admin"

// Policy represents a single, enforceable access control rule as registered
// in code. (Distinct from the domain rbac.Policy ABAC model, which is the
// database-backed conditional-rule representation.)
// In a more advanced system, these would be loaded from a database or config file.
type Policy struct {
	Name        string
	Permissions []core.PermissionName
	Check       PolicyCheckFunc
}

// --- Policy Implementations ---

// isResourceOwner is a policy that checks if the actor's ID matches the resource ID.
// It's a common policy for "self-service" actions.
func isResourceOwner(ctx context.Context, repo rbac.Repository, check rbac.PermissionCheck) (bool, error) {
	resourceIDVal, ok := check.ResourceContext["user_id"]
	if !ok {
		// This is a configuration error; the middleware should have provided the context.
		return false, fmt.Errorf("policy requires 'user_id' in resource context for permission '%s'", check.PermissionName)
	}

	resourceID, err := uuid.Parse(fmt.Sprintf("%v", resourceIDVal))
	if err != nil {
		return false, fmt.Errorf("invalid 'user_id' format in resource context")
	}

	// The check: is the actor trying to access their own resource?
	return check.ActorID == resourceID, nil
}
