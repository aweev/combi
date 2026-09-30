// internal/platform/events/types.go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// EventHandler is the function signature for downstream services to consume events.
type EventHandler func(ctx context.Context, event DomainEvent) error

// Bus defines the infrastructure-agnostic contract.
type Bus interface {
	// Publish sends an event to the stream.
	Publish(ctx context.Context, event DomainEvent) error

	// Subscribe registers a handler for a specific topic (Action).
	Subscribe(topic string, handler EventHandler)

	// Start begins consumption in background goroutines.
	Start(ctx context.Context) error

	// Shutdown gracefully stops consumers and waits for processing to finish.
	Shutdown(ctx context.Context) error

	// Health checks connectivity to the underlying broker.
	Health(ctx context.Context) error
}

type EventSubscriber interface {
	RegisterSubscribers(bus Bus)
}

// DomainEvent is the canonical envelope for all system events.
type DomainEvent struct {
	ID         uuid.UUID         `json:"id"`
	Action     core.EventAction  `json:"action"`  // e.g., "user.created"
	Version    int               `json:"version"` // Schema versioning
	Timestamp  time.Time         `json:"timestamp"`
	ActorID    uuid.UUID         `json:"actor_id"`
	ActorType  string            `json:"actor_type"`
	TargetID   *uuid.UUID        `json:"target_id,omitempty"`
	TargetType *string           `json:"target_type,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"` // TraceContext, IP, etc.
	Payload    json.RawMessage   `json:"payload,omitempty"`
}

// NewEvent creates a standardized event.
func NewEvent(action core.EventAction, actorID uuid.UUID, actorType string, payload interface{}) (DomainEvent, error) {
	var payloadBytes []byte
	var err error

	if payload != nil {
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return DomainEvent{}, fmt.Errorf("failed to marshal event payload: %w", err)
		}
	}

	return DomainEvent{
		ID:        uuid.New(),
		Action:    action,
		Version:   1, // Default to version 1
		ActorID:   actorID,
		ActorType: actorType,
		Timestamp: time.Now().UTC(),
		Payload:   payloadBytes,
		Metadata:  make(map[string]string),
	}, nil
}

// UnmarshalPayload decodes the payload into a target struct.
func (e *DomainEvent) UnmarshalPayload(v interface{}) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}
