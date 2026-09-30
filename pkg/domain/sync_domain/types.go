package sync_domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type SyncAction string

const (
	ActionCreate SyncAction = "CREATE"
	ActionUpdate SyncAction = "UPDATE"
	ActionDelete SyncAction = "DELETE"
)

type ConflictStrategy string

const (
	StrategyServerWins ConflictStrategy = "SERVER_WINS" // Reject if version mismatch
	StrategyClientWins ConflictStrategy = "CLIENT_WINS" // Force overwrite
)

type SyncStatus string

const (
	StatusSuccess  SyncStatus = "SUCCESS"
	StatusConflict SyncStatus = "CONFLICT" // Client needs to re-fetch
	StatusError    SyncStatus = "ERROR"    // Validation or System error
)

// --- Models ---

type SyncPacket struct {
	DeviceID      string          `json:"device_id"`
	LastSyncToken string          `json:"last_sync_token"` // RFC3339 Timestamp
	Operations    []SyncOperation `json:"operations"`
}

type SyncOperation struct {
	TempID     string           `json:"temp_id"`             // Client-generated UUID (for CREATE)
	EntityID   *uuid.UUID       `json:"entity_id,omitempty"` // Server UUID (for UPDATE/DELETE)
	EntityType string           `json:"entity_type"`         // e.g. "form_submission"
	Action     SyncAction       `json:"action"`
	Payload    json.RawMessage  `json:"payload"`
	Version    int              `json:"version"` // Optimistic Locking
	Strategy   ConflictStrategy `json:"strategy"`
	OccurredAt time.Time        `json:"occurred_at"`
}

type SyncResponse struct {
	Results   []SyncResult `json:"results"`
	SyncToken string       `json:"sync_token"` // New timestamp
}

type SyncResult struct {
	TempID      string          `json:"temp_id"`
	EntityID    *uuid.UUID      `json:"entity_id,omitempty"`
	Status      SyncStatus      `json:"status"`
	NewVersion  int             `json:"new_version,omitempty"`
	ServerState json.RawMessage `json:"server_state,omitempty"` // For conflict resolution
	Error       *string         `json:"error,omitempty"`
}

// DataDelta represents changes on the server since the client's last pull.
type DataDelta struct {
	SyncToken string                   `json:"sync_token"`
	Changes   map[string][]EntityState `json:"changes"` // map[entity_type] -> list of items
}

type EntityState struct {
	ID        uuid.UUID       `json:"id"`
	Version   int             `json:"version"`
	Deleted   bool            `json:"deleted"`
	Data      json.RawMessage `json:"data,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// --- Interfaces ---

// SyncableService must be implemented by domains (e.g., FormService, UserService)
// that wish to support offline sync.
type SyncableService interface {
	// ApplySyncChange processes a single operation.
	// It must handle Idempotency if the same TempID is retried.
	ApplySyncChange(ctx context.Context, actorID uuid.UUID, op SyncOperation) (uuid.UUID, int, error)

	// FetchDelta returns all records visible to the user that changed after `since`.
	FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]EntityState, error)
}

type Service interface {
	// Registry
	RegisterDomain(entityType string, svc SyncableService)

	// Client Operations
	Push(ctx context.Context, actorID uuid.UUID, packet SyncPacket) (*SyncResponse, error)
	Pull(ctx context.Context, actorID uuid.UUID, deviceID string, since *time.Time) (*DataDelta, error)
}

// Add Repository Interface locally or in domain
type Repository interface {
	EnsureDevice(ctx context.Context, userID uuid.UUID, deviceID string) error
	LogSession(ctx context.Context, userID uuid.UUID, deviceID, sType, status string, opsCount int, errMsg *string) error
}
