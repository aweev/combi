// emit.go — context-filling event emission.
//
// Emit is the recommended way to publish domain events: it fills the event's
// Metadata with the request correlation values from the context (request_id,
// tenant_id, trace_id) before publishing, so consumers always have them
// without each service reimplementing the plumbing.
//
// Usage:
//
//	if err := events.Emit(ctx, bus, core.EventAccountCreated, accountID, "account", payload); err != nil {
//	    // Non-fatal: log and continue, or wrap per service policy.
//	    return fmt.Errorf("emit account.created: %w", err)
//	}

package events

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/middleware"
	"github.com/google/uuid"
)

// Emit builds a DomainEvent for the given action, enriches it with request
// correlation metadata from the context, and publishes it to the bus.
//
// Metadata keys (aligned with pkg/middleware wire headers):
//
//	request_id  — from middleware.RequestIDFromContext
//	tenant_id   — from middleware.TenantIDFromContext
//	trace_id    — from middleware.TraceIDFromContext
//	actor_id    — the actorID parameter (for quick consumer-side filtering)
func Emit(ctx context.Context, bus Bus, action core.EventAction, actorID uuid.UUID, actorType string, payload interface{}) error {
	event, err := NewEvent(action, actorID, actorType, payload)
	if err != nil {
		return err
	}
	return EmitEvent(ctx, bus, event)
}

// EmitEvent enriches an already-built DomainEvent with correlation metadata
// from the context and publishes it. Use it when you need to customise the
// event before publishing (e.g. set TargetID/TargetType).
func EmitEvent(ctx context.Context, bus Bus, event DomainEvent) error {
	if bus == nil {
		return fmt.Errorf("events: cannot emit %q — bus is nil", event.Action)
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	// Correlation values — a missing value simply isn't recorded, so tests
	// and background jobs without a request context keep working.
	if v := middleware.RequestIDFromContext(ctx); v != "" {
		event.Metadata["request_id"] = v
	}
	if v := middleware.TenantIDFromContext(ctx); v != "" {
		event.Metadata["tenant_id"] = v
	}
	if v := middleware.TraceIDFromContext(ctx); v != "" {
		event.Metadata["trace_id"] = v
	}
	if event.ActorID != (uuid.UUID{}) {
		event.Metadata["actor_id"] = event.ActorID.String()
	}

	return bus.Publish(ctx, event)
}
