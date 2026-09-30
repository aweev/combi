package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/security"
	"github.com/google/uuid"
)

var (
	// 404 – Not Found
	ErrSecretNotFound = apperror.New(
		http.StatusNotFound,
		"SECRET_NOT_FOUND",
		"The requested secret could not be found.",
		nil,
	)

	// 409 – Conflict (already exists)
	ErrSecretAlreadyExists = apperror.New(
		http.StatusConflict,
		"SECRET_ALREADY_EXISTS",
		"A secret with this name already exists.",
		nil,
	)

	// 500 – Internal Server Errors
	ErrRotationFailed = apperror.New(
		http.StatusInternalServerError,
		"ROTATION_FAILED",
		"The secret rotation process failed.",
		nil,
	)

	ErrDecryptionFailed = apperror.New(
		http.StatusInternalServerError,
		"DECRYPTION_FAILED",
		"Failed to decrypt the secret value. This may indicate data corruption or a key mismatch.",
		nil,
	)
)

// BackendError wraps storage-layer failures.
type BackendError struct {
	Err     error  // original error
	Backend string // e.g., "mongo", "postgres", "vault", etc.
	Name    string // secret name
	IsFatal bool   // true = permanent failure, false = transient recoverable error
}

func (e *BackendError) Error() string {
	return fmt.Sprintf("secret backend '%s' error for secret '%s': %v",
		e.Backend, e.Name, e.Err)
}

func (e *BackendError) Unwrap() error { return e.Err }

const (
	EventSecretCreated core.EventAction = "secret.created"
	EventSecretUpdated core.EventAction = "secret.updated"
	EventSecretDeleted core.EventAction = "secret.deleted"

	EventSecretAccessed         core.EventAction = "secret.accessed"
	EventSecretRotated          core.EventAction = "secret.rotated"
	EventSecretRotationFailed   core.EventAction = "secret.rotation.failed"
	EventSecretDecryptionFailed core.EventAction = "secret.decryption.failed"
)

// SecretAccessedPayload is the data published when a secret is retrieved.
type SecretAccessedPayload struct {
	Name       string    `json:"name"`
	Version    int       `json:"version"`
	AccessedBy string    `json:"accessed_by"`
	Timestamp  time.Time `json:"timestamp"`
}

const (
	// JobSecretRotate is the job type for automated background rotation of a secret.
	JobSecretRotate core.JobType = "secret:rotate"
)

const (
	// ResourceSecret is the canonical name for secret resources.
	ResourceSecret = "secret"
)

var (
	// PermSecretRead allows reading secret metadata and values.
	PermSecretRead = core.BuildPermissionName(ResourceSecret, "read")
	// PermSecretManage allows creating, updating, and deleting secrets.
	PermSecretManage = core.BuildPermissionName(ResourceSecret, "manage")
	// PermSecretRotate allows triggering a manual rotation.
	PermSecretRotate = core.BuildPermissionName(ResourceSecret, "rotate")
)

type SecretVersion struct {
	Version        int
	EncryptedValue []byte
	CreatedAt      time.Time
	CreatedBy      uuid.UUID
}

type SecretRotatedPayload struct {
	Name       string    `json:"name"`
	OldVersion int       `json:"old_version"`
	NewVersion int       `json:"new_version"`
	RotatedBy  string    `json:"rotated_by"`
	Timestamp  time.Time `json:"timestamp"`
}

type RotationFailedPayload struct {
	Name             string    `json:"name"`
	AttemptedVersion int       `json:"attempted_version"`
	AttemptedBy      string    `json:"attempted_by"`
	Reason           string    `json:"reason"`
	Timestamp        time.Time `json:"timestamp"`
}

type SecretDecryptionFailedPayload struct {
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	AttemptedBy string    `json:"attempted_by"`
	Error       string    `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
}

type SecretStatus string

const (
	StatusActive         SecretStatus = "ACTIVE"
	StatusRotationFailed SecretStatus = "ROTATION_FAILED"
	StatusDeprecated     SecretStatus = "DEPRECATED"
)

type Secret struct {
	ID                uuid.UUID
	Name              string
	EncryptedValue    []byte
	EncryptionContext []byte // JSON serialized security.EncryptionContext
	Version           int
	Status            SecretStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	RotationPolicy    *RotationPolicy
	Metadata          map[string]string
}

type RotationPolicy struct {
	Enabled        bool          `json:"enabled"`
	Frequency      time.Duration `json:"frequency"` // e.g., 720h (30 days)
	NextRotationAt time.Time     `json:"next_rotation_at"`
}

func (p *RotationPolicy) ScheduleNextRotation() {
	if p.Enabled {
		p.NextRotationAt = time.Now().UTC().Add(p.Frequency)
	}
}

// Helper methods to handle serialization of the encryption context

func (s *Secret) SetEncryptionContext(ctx *security.EncryptionContext) error {
	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal encryption context: %w", err)
	}
	s.EncryptionContext = data
	return nil
}

func (s *Secret) GetEncryptionContext() (*security.EncryptionContext, error) {
	if len(s.EncryptionContext) == 0 {
		return nil, fmt.Errorf("encryption context is empty")
	}
	var ctx security.EncryptionContext
	if err := json.Unmarshal(s.EncryptionContext, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal encryption context: %w", err)
	}
	return &ctx, nil
}

// --- Events ---

// SecretValue is the decrypted map of key-value pairs
type SecretValue map[string]interface{}

// CreateSecretParams input DTO
type CreateSecretParams struct {
	Name           string
	Value          SecretValue
	RotationPolicy *RotationPolicy
	Metadata       map[string]string
}

// UpdateSecretParams input DTO
type UpdateSecretParams struct {
	Value          SecretValue
	RotationPolicy *RotationPolicy
	Metadata       map[string]string
}

// Service defines the core business logic for retrieving secret values.
// This is the interface most services in your application will use.
type Service interface {
	// GetSecretValue retrieves a secret and returns its decrypted value.
	// It creates an audit event for the access and handles caching.
	GetSecretValue(ctx context.Context, key string) (SecretValue, error)
}

// AdminService defines the administrative contract for managing the secret lifecycle.
type AdminService interface {
	CreateSecret(ctx context.Context, params CreateSecretParams) (*Secret, error)
	UpdateSecret(ctx context.Context, key string, params UpdateSecretParams) (*Secret, error)
	GetSecretMetadata(ctx context.Context, key string) (*Secret, error)
	ListSecrets(ctx context.Context, prefix string) ([]string, error)
	DeleteSecret(ctx context.Context, key string) error
	RotateSecret(ctx context.Context, key string) (*Secret, error)
}

// Repository defines the persistence contract for secrets, agnostic of the backend.
type Repository interface {
	Get(ctx context.Context, name string) (*Secret, error)
	GetVersion(ctx context.Context, name string, version int) (*Secret, error)
	Set(ctx context.Context, secret *Secret) error
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, name string) error
	FindSecretsDueForRotation(ctx context.Context, now time.Time) ([]*Secret, error)
}

type Rotator interface {
	Rotate(ctx context.Context, oldSecret *Secret) (newValue map[string]interface{}, err error)
}

type RotatorFactory interface {
	GetRotator(secret *Secret) (Rotator, error)
}
