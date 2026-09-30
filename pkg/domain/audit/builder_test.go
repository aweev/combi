package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestEventBuilder(t *testing.T) {
	actorID := uuid.New()
	targetID := uuid.New()

	t.Run("should build a minimal successful event", func(t *testing.T) {
		event := audit.NewEvent(actorID, "user", "auth.login").WithSuccess().Build()

		assert.Equal(t, actorID, event.ActorID)
		assert.Equal(t, "user", event.ActorType)
		assert.Equal(t, core.EventAction("auth.login"), event.Action)
		assert.Equal(t, audit.OutcomeSuccess, event.Outcome)
		assert.WithinDuration(t, time.Now().UTC(), event.CreatedAt, 2*time.Second)
		assert.NotNil(t, event.ID)
	})

	t.Run("should build a failed event with a reason", func(t *testing.T) {
		event := audit.NewEvent(actorID, "system", "job.failed").
			WithFailure("dependency unavailable").
			Build()

		assert.Equal(t, audit.OutcomeFailed, event.Outcome)
		assert.NotNil(t, event.OutcomeReason)
		assert.Equal(t, "dependency unavailable", *event.OutcomeReason)
	})

	t.Run("should correctly add target information", func(t *testing.T) {
		event := audit.NewEvent(actorID, "user", "document.deleted").
			WithTarget(targetID, "document", "Annual Report").
			Build()

		assert.NotNil(t, event.TargetID)
		assert.Equal(t, targetID, *event.TargetID)
		assert.NotNil(t, event.TargetType)
		assert.Equal(t, "document", *event.TargetType)
		assert.Equal(t, "Annual Report", event.Sensitive.TargetName)
	})

	t.Run("should extract context values", func(t *testing.T) {
		// Create a mock span with a trace ID
		traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

		// Add our custom context values
		ctx = audit.WithIPAddress(ctx, "192.168.1.1")
		ctx = audit.WithUserAgent(ctx, "Go-Test")

		event := audit.NewEvent(actorID, "user", "auth.login").WithContext(ctx).Build()

		assert.NotNil(t, event.TraceID)
		assert.Equal(t, traceID.String(), *event.TraceID)
		assert.NotNil(t, event.IPAddress)
		assert.Equal(t, "192.168.1.1", *event.IPAddress)
		assert.NotNil(t, event.UserAgent)
		assert.Equal(t, "Go-Test", *event.UserAgent)
	})
}
