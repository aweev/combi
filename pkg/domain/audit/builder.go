package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// contextKey is a private type to prevent collisions with other packages' context keys.
type contextKey string

const (
	// ipAddressKey is the context key for the client's IP address.
	ipAddressKey contextKey = "ip_address"
	// userAgentKey is the context key for the client's User-Agent header.
	userAgentKey contextKey = "user_agent"
)

// WithIPAddress adds an IP address to a context for the audit builder to extract.
func WithIPAddress(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipAddressKey, ip)
}

// WithUserAgent adds a User-Agent string to a context for the audit builder to extract.
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, userAgentKey, ua)
}

// EventBuilder provides a fluent API for creating audit events, ensuring
// all required fields are set and improving the readability of event creation logic.
type EventBuilder struct {
	event Event
}

// NewEvent starts building a new audit event.
// actorID, actorType, and action are the minimum required fields.
func NewEvent(actorID uuid.UUID, actorType string, action core.EventAction) *EventBuilder {
	return &EventBuilder{
		event: Event{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			ActorID:   actorID,
			ActorType: actorType,
			Action:    action,
			Outcome:   OutcomePending, // Default to pending until explicitly set
		},
	}
}

// WithTarget sets the entity that the action was performed on.
func (b *EventBuilder) WithTarget(id uuid.UUID, typeName, displayName string) *EventBuilder {
	b.event.TargetID = &id
	b.event.TargetType = &typeName
	b.event.Sensitive.TargetName = displayName
	return b
}

// WithSuccess marks the event as successful.
func (b *EventBuilder) WithSuccess() *EventBuilder {
	b.event.Outcome = OutcomeSuccess
	return b
}

// WithFailure marks the event as failed and includes a reason.
func (b *EventBuilder) WithFailure(reason string) *EventBuilder {
	b.event.Outcome = OutcomeFailed
	b.event.OutcomeReason = &reason
	return b
}

// WithSensitiveData populates the entire sensitive, encrypted portion of the event.
func (b *EventBuilder) WithSensitiveData(sensitiveData SensitiveData) *EventBuilder {
	b.event.Sensitive = sensitiveData
	return b
}

// WithMetadata adds arbitrary, sensitive data to the event. This will be encrypted.
func (b *EventBuilder) WithMetadata(data map[string]interface{}) *EventBuilder {
	if raw, err := json.Marshal(data); err == nil {
		b.event.Sensitive.Metadata = raw
	}
	return b
}

// WithContext extracts common contextual information like TraceID, IP address, etc.
func (b *EventBuilder) WithContext(ctx context.Context) *EventBuilder {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		traceID := span.SpanContext().TraceID().String()
		b.event.TraceID = &traceID
	}
	if ip, ok := ctx.Value(ipAddressKey).(string); ok {
		b.event.IPAddress = &ip
	}
	if ua, ok := ctx.Value(userAgentKey).(string); ok {
		b.event.UserAgent = &ua
	}
	return b
}

func (b *EventBuilder) WithOutcome(outcome Outcome) *EventBuilder {
	b.event.Outcome = outcome
	return b
}

// Build returns the final, constructed audit event.
func (b *EventBuilder) Build() Event {
	return b.event
}
