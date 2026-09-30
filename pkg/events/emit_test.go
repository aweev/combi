package events

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/middleware"
	"github.com/google/uuid"
)

// captureBus records published events.
type captureBus struct {
	published []DomainEvent
}

func (b *captureBus) Publish(ctx context.Context, event DomainEvent) error {
	b.published = append(b.published, event)
	return nil
}
func (b *captureBus) Subscribe(topic string, handler EventHandler) {}
func (b *captureBus) Start(ctx context.Context) error              { return nil }
func (b *captureBus) Shutdown(ctx context.Context) error           { return nil }
func (b *captureBus) Health(ctx context.Context) error             { return nil }

func TestEmitFillsCorrelationMetadata(t *testing.T) {
	ctx := middleware.WithRequestID(
		middleware.WithTenantID(context.Background(), "ten_emit"), "req_emit",
	)
	actor := uuid.New()
	bus := &captureBus{}

	if err := Emit(ctx, bus, core.EventAccountCreated, actor, "account", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if len(bus.published) != 1 {
		t.Fatalf("published %d events, want 1", len(bus.published))
	}
	evt := bus.published[0]
	if evt.Metadata["request_id"] != "req_emit" {
		t.Errorf("request_id = %q, want req_emit", evt.Metadata["request_id"])
	}
	if evt.Metadata["tenant_id"] != "ten_emit" {
		t.Errorf("tenant_id = %q, want ten_emit", evt.Metadata["tenant_id"])
	}
	if evt.Metadata["actor_id"] != actor.String() {
		t.Errorf("actor_id = %q, want %s", evt.Metadata["actor_id"], actor)
	}
	if evt.Action != core.EventAccountCreated {
		t.Errorf("action = %q", evt.Action)
	}
}

func TestEmitWithoutContextValuesStillPublishes(t *testing.T) {
	bus := &captureBus{}
	if err := Emit(context.Background(), bus, core.EventAccountCreated, uuid.New(), "account", nil); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(bus.published) != 1 {
		t.Fatalf("published %d events, want 1", len(bus.published))
	}
}

func TestEmitNilBusErrors(t *testing.T) {
	err := EmitEvent(context.Background(), nil, DomainEvent{Action: "x.y"})
	if err == nil {
		t.Fatal("nil bus must be rejected")
	}
}
