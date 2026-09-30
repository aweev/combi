package env

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/google/uuid"
)

// EnvRepository implements the secret.Repository interface using a JSON object
// from the ngo_SECRETS environment variable. It is intended for local development.
type EnvRepository struct {
	mu      sync.RWMutex
	secrets map[string]*secret.Secret // Store the full Secret model for simulation
}

// NewEnvRepository creates a repository from the ngo_SECRETS environment variable.
// The env var is expected to be a JSON object where keys are secret names and values are the secret data.
func NewEnvRepository() (*EnvRepository, error) {
	secretsJSON := os.Getenv("ngo_SECRETS")
	if secretsJSON == "" {
		// It's not an error to have no secrets; the map will just be empty.
		return &EnvRepository{
			secrets: make(map[string]*secret.Secret),
		}, nil
	}

	var rawSecrets map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(secretsJSON), &rawSecrets); err != nil {
		return nil, fmt.Errorf("failed to parse ngo_SECRETS JSON: %w", err)
	}

	// Pre-process the raw secrets into our domain model for easier handling.
	// For simulation, we'll just store the raw JSON value and a dummy encryption context.
	parsedSecrets := make(map[string]*secret.Secret)
	for key, value := range rawSecrets {
		valueBytes, _ := json.Marshal(value)
		encContextBytes, _ := json.Marshal(map[string]string{"provider": "env"})

		parsedSecrets[key] = &secret.Secret{
			ID:                uuid.NewSHA1(uuid.Nil, []byte(key)),
			Name:              key,
			EncryptedValue:    valueBytes, // In this mock, the value is not actually encrypted.
			EncryptionContext: encContextBytes,
			Version:           1,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		}
	}

	return &EnvRepository{secrets: parsedSecrets}, nil
}

func (r *EnvRepository) Get(ctx context.Context, name string) (*secret.Secret, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.secrets[name]
	if !ok {
		return nil, secret.ErrSecretNotFound
	}
	return s, nil
}

func (r *EnvRepository) Set(ctx context.Context, s *secret.Secret) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.secrets[s.Name] = s
	return nil
}

func (r *EnvRepository) List(ctx context.Context, prefix string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var keys []string
	for key := range r.secrets {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (r *EnvRepository) Delete(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.secrets, name)
	return nil
}

func (r *EnvRepository) FindSecretsDueForRotation(ctx context.Context, now time.Time) ([]*secret.Secret, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var dueSecrets []*secret.Secret
	for _, s := range r.secrets {
		if s.RotationPolicy != nil && s.RotationPolicy.Enabled && !s.RotationPolicy.NextRotationAt.IsZero() && s.RotationPolicy.NextRotationAt.Before(now) {
			dueSecrets = append(dueSecrets, s)
		}
	}

	return dueSecrets, nil
}

func (r *EnvRepository) GetVersion(ctx context.Context, name string, version int) (*secret.Secret, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.secrets[name]
	if !ok {
		return nil, secret.ErrSecretNotFound
	}

	// In the simple Env mock, we only store the current version.
	if s.Version != version {
		return nil, secret.ErrSecretNotFound
	}

	return s, nil
}
