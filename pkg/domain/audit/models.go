package audit

import (
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/security"
	"github.com/google/uuid"
)

// --- Enums ---

// Outcome represents the result of an audited action.
type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailed  Outcome = "FAILED"
	OutcomePending Outcome = "PENDING" // For long-running or asynchronous actions
)

// --- Core Models ---

// SystemActorID is a predefined UUID for actions performed by the system itself
// (e.g., in a background job or workflow), rather than by a specific user.
var SystemActorID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// SensitiveData contains all fields within an audit event that must be encrypted at rest.
// This struct makes the security boundary explicit.
type SensitiveData struct {
	ActorName  string          `json:"actor_name,omitempty"`  // PII: e.g., "john.doe@example.com"
	TargetName string          `json:"target_name,omitempty"` // Sensitive: e.g., "Annual Financial Report"
	OldValues  json.RawMessage `json:"old_values,omitempty"`  // The state of the resource before the change.
	NewValues  json.RawMessage `json:"new_values,omitempty"`  // The state of the resource after the change.
	Metadata   json.RawMessage `json:"metadata,omitempty"`    // Any other sensitive contextual data.
	KeyID      string          `json:"key_id,omitempty"`
}

// Event is the immutable, canonical record of a significant action in the system.
type Event struct {
	// --- Core, Unencrypted Fields (for indexing and querying) ---
	ID         uuid.UUID        `json:"id"`
	CreatedAt  time.Time        `json:"created_at"`
	ActorID    uuid.UUID        `json:"actor_id"`
	ActorType  string           `json:"actor_type"` // e.g., "user", "system", "api_key"
	Action     core.EventAction `json:"action"`
	Outcome    Outcome          `json:"outcome"`
	TargetID   *uuid.UUID       `json:"target_id,omitempty"`
	TargetType *string          `json:"target_type,omitempty"`

	// --- Encrypted Payload ---
	// These fields are handled by the repository and are not exposed in API responses.
	EncryptedData     []byte
	EncryptionContext *security.EncryptionContext

	// --- Decrypted Sensitive Fields (populated on read) ---
	// This field is populated by the service after decryption and is not stored in the database.
	Sensitive SensitiveData `json:"sensitive,omitempty" db:"-"`

	// --- Unencrypted Contextual Information ---
	OutcomeReason *string `json:"outcome_reason,omitempty"` // e.g., "Invalid password", "Allowed by policy [ID]"
	IPAddress     *string `json:"ip_address,omitempty"`
	UserAgent     *string `json:"user_agent,omitempty"`
	TraceID       *string `json:"trace_id,omitempty"`
}

// RetentionPolicy defines how long audit data for a specific type should be kept.
type RetentionPolicy struct {
	ID          uuid.UUID     `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	DataType    string        `json:"data_type"` // e.g., "auth.*", "financial.donation", "*" for all
	Duration    time.Duration `json:"duration"`  // e.g., 365 * 24 * time.Hour for one year
	IsActive    bool          `json:"is_active"`
	CreatedBy   uuid.UUID     `json:"created_by"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type LogEntry struct {
	ID         uuid.UUID        `json:"id"`
	Action     core.EventAction `json:"action"`
	ActorID    uuid.UUID        `json:"actor_id"`
	ActorType  string           `json:"actor_type"`
	TargetID   uuid.UUID        `json:"target_id"`
	TargetType string           `json:"target_type"`
	IPAddress  string           `json:"ip_address"`
	UserAgent  string           `json:"user_agent"`
	Changes    map[string]any   `json:"changes,omitempty"` // Diff of old vs new
	Metadata   map[string]any   `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	TenantID   uuid.UUID        `json:"tenant_id"` // Multi-tenancy
	Encrypted  bool             `json:"encrypted"` // Flag if changes are encrypted
}

type LegalHold struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	HoldSubject string     `json:"hold_subject"`
	Reason      string     `json:"reason"`
	Filter      LogFilter  `json:"filter"` // e.g., "ActorID = X"
	CreatedBy   uuid.UUID  `json:"created_by"`
	IssuedAt    time.Time  `json:"issued_at"`
	CreatedAt   time.Time  `json:"created_at"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
	ReleasedBy  *uuid.UUID `json:"released_by,omitempty"`
	Active      bool       `json:"active"`
}
