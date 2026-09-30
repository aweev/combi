// internal/platform/secret/rotator.go
package rotator

import (
	"context"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/security"
)

// --- Concrete Rotator for API Keys ---

type apiKeyRotator struct {
	randomSvc security.RandomService
}

func (r *apiKeyRotator) Rotate(_ context.Context, _ *secret.Secret) (map[string]interface{}, error) {
	newKey, err := r.randomSvc.GenerateString(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random string for new secret: %w", err)
	}

	newValue := map[string]interface{}{
		"api_key":      fmt.Sprintf("sk_live_%s", newKey),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"rotated_by":   "ngo.secret-service",
	}
	return newValue, nil
}

// --- Rotator Factory Implementation ---

type rotatorFactoryImpl struct {
	// You would inject dependencies needed by different rotators here
	randomSvc security.RandomService
	// stripeClient *stripe.Client
}

func NewRotatorFactory(randomSvc security.RandomService) secret.RotatorFactory {
	return &rotatorFactoryImpl{
		randomSvc: randomSvc,
	}
}

func (f *rotatorFactoryImpl) GetRotator(s *secret.Secret) (secret.Rotator, error) {
	// The rotation strategy is determined by metadata on the secret itself.
	rotationType, ok := s.Metadata["rotation_type"]
	if !ok {
		return nil, fmt.Errorf("secret '%s' is not configured for rotation (missing 'rotation_type' metadata)", s.Name)
	}

	switch rotationType {
	case "api_key_random":
		return &apiKeyRotator{randomSvc: f.randomSvc}, nil
	// case "stripe_api_key":
	// 	return &stripeRotator{client: f.stripeClient}, nil
	default:
		return nil, fmt.Errorf("unknown rotation type '%s' for secret '%s'", rotationType, s.Name)
	}
}
