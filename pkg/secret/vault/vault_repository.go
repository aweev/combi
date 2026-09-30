package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/google/uuid"
	"github.com/hashicorp/vault/api"
)

// VaultRepository implements the secret.Repository interface using HashiCorp Vault's KVv2 engine.
type VaultRepository struct {
	client *api.Client
	prefix string
}

// FindSecretsDueForRotation implements secret.Repository.
func (r *VaultRepository) FindSecretsDueForRotation(ctx context.Context, now time.Time) ([]*secret.Secret, error) {
	panic("unimplemented")
}

// NewVaultRepository creates a new Vault repository. The client should already be authenticated.
func NewVaultRepository(client *api.Client, prefix string) *VaultRepository {
	return &VaultRepository{
		client: client,
		prefix: prefix,
	}
}

// Get reads a secret from Vault, parsing the KVv2 response structure.
func (r *VaultRepository) Get(ctx context.Context, name string) (*secret.Secret, error) {
	// For KVv2, the API path includes `/data/` between the mount path and the secret path.
	secretPath := path.Join(r.prefix, "data", name)

	vaultSecret, err := r.client.Logical().ReadWithContext(ctx, secretPath)
	if err != nil {
		return nil, &secret.BackendError{Err: err, Backend: "vault", Name: name}
	}
	if vaultSecret == nil || vaultSecret.Data == nil {
		return nil, secret.ErrSecretNotFound
	}

	data, ok := vaultSecret.Data["data"].(map[string]interface{})
	if !ok {
		// If `data` is missing, the secret might be malformed or from a different engine version.
		return nil, &secret.BackendError{
			Err:     fmt.Errorf("secret data is not in the expected KVv2 format"),
			Backend: "vault",
			Name:    name,
			IsFatal: true, // This is a structural problem, not a transient one.
		}
	}

	var rawValue, encContext []byte
	if val, ok := data["encrypted_value"]; ok {
		rawValue, _ = json.Marshal(val)
	}
	if ctx, ok := data["encryption_context"]; ok {
		encContext, _ = json.Marshal(ctx)
	}

	// Extract metadata from Vault's response
	metadata, _ := vaultSecret.Data["metadata"].(map[string]interface{})
	version := 0
	if v, ok := metadata["version"].(json.Number); ok {
		v64, _ := v.Int64()
		version = int(v64)
	}
	createdAt := time.Now().UTC()
	if c, ok := metadata["created_time"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, c)
	}
	updatedAt := createdAt
	if u, ok := metadata["updated_time"].(string); ok {
		updatedAt, _ = time.Parse(time.RFC3339, u)
	}

	return &secret.Secret{
		ID:                uuid.NewSHA1(uuid.Nil, []byte(name)), // Deterministic ID for consistency
		Name:              name,
		EncryptedValue:    rawValue,
		EncryptionContext: encContext,
		Version:           version,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func (r *VaultRepository) Set(ctx context.Context, s *secret.Secret) error {
	secretPath := path.Join(r.prefix, "data", s.Name)

	var valueData, encContextData, rotationPolicyData interface{}
	// Use json.Unmarshal to convert byte slices back to generic interfaces
	_ = json.Unmarshal(s.EncryptedValue, &valueData)
	_ = json.Unmarshal(s.EncryptionContext, &encContextData)
	if s.RotationPolicy != nil {
		// To store the rotation policy as a JSON object in Vault
		rotationPolicyBytes, _ := json.Marshal(s.RotationPolicy)
		_ = json.Unmarshal(rotationPolicyBytes, &rotationPolicyData)
	}

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"encrypted_value":    valueData,
			"encryption_context": encContextData,
			"version_id":         s.ID.String(),
			"status":             s.Status,
			"metadata":           s.Metadata,
			"rotation_policy":    rotationPolicyData,
		},
	}

	_, err := r.client.Logical().WriteWithContext(ctx, secretPath, payload)
	if err != nil {
		return &secret.BackendError{Err: err, Backend: "vault", Name: s.Name}
	}
	return nil
}

// List secrets by reading the metadata path for a KVv2 engine.
func (r *VaultRepository) List(ctx context.Context, prefix string) ([]string, error) {
	// For KVv2, listing is done on the `/metadata/` path.
	listPath := path.Join(r.prefix, "metadata", prefix)

	secretList, err := r.client.Logical().ListWithContext(ctx, listPath)
	if err != nil {
		return nil, &secret.BackendError{Err: err, Backend: "vault", Name: "list operation"}
	}
	if secretList == nil || secretList.Data["keys"] == nil {
		return []string{}, nil // No keys found
	}

	keysData, ok := secretList.Data["keys"].([]interface{})
	if !ok {
		return nil, &secret.BackendError{Err: err, Backend: "vault", Name: "list format error"}
	}

	keys := make([]string, len(keysData))
	for i, k := range keysData {
		keys[i] = k.(string)
	}
	return keys, nil
}

// Delete a secret by calling the metadata delete endpoint for KVv2.
func (r *VaultRepository) Delete(ctx context.Context, name string) error {
	// In KVv2, you must delete all versions by writing to the metadata path.
	deletePath := path.Join(r.prefix, "metadata", name)
	_, err := r.client.Logical().DeleteWithContext(ctx, deletePath)
	if err != nil {
		return &secret.BackendError{Err: err, Backend: "vault", Name: name}
	}
	return nil
}

func (r *VaultRepository) GetVersion(ctx context.Context, name string, version int) (*secret.Secret, error) {
	// KVv2 supports ?version=N
	_= path.Join(r.prefix, "data", name)

	// Vault API expects version as a query param in the data map for Read
	// However, the Go client helper ReadWithContext doesn't easily allow query params.
	// We manually construct the path with query params or use the client's underlying request builder.
	// For simplicity in Phase 0, we'll use the generic map approach if possible,
	// or we strictly check the version after read if the client lib is restrictive.

	// NOTE: The hashicorp/vault/api client handles versioning via options usually.
	// Since this is a complex implementation detail of the Vault client,
	// for Phase 0, we will implement a "Check after Read" strategy similar to Postgres
	// unless we want to write a custom request.

	current, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if current.Version != version {
		// TODO: Implement historical version fetching via Vault specific API params
		return nil, secret.ErrSecretNotFound
	}
	return current, nil
}
