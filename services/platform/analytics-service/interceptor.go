package analytics

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// EventInterceptor is a decorator for the EventBus.
// It intercepts every Publish call and forwards a copy to the Analytics Service.
type EventInterceptor struct {
	next    events.Bus
	service analytics.Service
	logger  log.Logger
}

// Health implements events.Bus.
func (i *EventInterceptor) Health(ctx context.Context) error {
	panic("unimplemented")
}

// NewEventInterceptor creates the wrapper.
// 'next' is the real RedisStreamBus.
func NewEventInterceptor(next events.Bus, service analytics.Service, logger log.Logger) *EventInterceptor {
	return &EventInterceptor{
		next:    next,
		service: service,
		logger:  logger.With(log.String("component", "analytics_interceptor")),
	}
}

// Subscribe delegates directly to the underlying bus.
func (i *EventInterceptor) Subscribe(topic string, handler events.EventHandler) {
	i.next.Subscribe(topic, handler)
}

// Start delegates directly to the underlying bus.
func (i *EventInterceptor) Start(ctx context.Context) error {
	return i.next.Start(ctx)
}

// Shutdown delegates directly to the underlying bus.
func (i *EventInterceptor) Shutdown(ctx context.Context) error {
	return i.next.Shutdown(ctx)
}

// Publish is where the magic happens.
func (i *EventInterceptor) Publish(ctx context.Context, evt events.DomainEvent) error {
	if err := i.next.Publish(ctx, evt); err != nil {
		return err
	}

	i.track(context.Background(), evt)

	return nil
}

func (i *EventInterceptor) track(ctx context.Context, evt events.DomainEvent) {
	// Don't track the batch event itself to avoid infinite loops or noise
	if evt.Action == "job.outcomes.batched" {
		return
	}

	// Convert DomainEvent -> AnalyticsEvent
	aEvent := i.mapToAnalytics(evt)

	if err := i.service.TrackEvent(ctx, aEvent); err != nil {
		// Log but don't crash; analytics shouldn't break the app
		i.logger.Debug("failed to intercept event for analytics", log.ErrorAttr(err))
	}
}

func (i *EventInterceptor) mapToAnalytics(evt events.DomainEvent) analytics.AnalyticsEvent {
	// 1. Flatten payload for storage
	var props map[string]any
	if len(evt.Payload) > 0 {
		if err := json.Unmarshal(evt.Payload, &props); err != nil {
			props = map[string]any{"raw_payload": string(evt.Payload)}
		}
	} else {
		props = make(map[string]any)
	}

	// 2. Merge Metadata into properties (e.g. TraceID, IP)
	for k, v := range evt.Metadata {
		props["meta_"+k] = v
	}

	// 3. Resolve Actor
	var userID *uuid.UUID
	if evt.ActorType == "user" {
		// Assuming ActorID is a valid UUID
		uid := evt.ActorID
		userID = &uid
	} else {
		props["actor_type"] = evt.ActorType
		props["actor_id"] = evt.ActorID.String()
	}

	// 4. Target info
	if evt.TargetID != nil {
		props["target_id"] = evt.TargetID.String()
	}
	if evt.TargetType != nil {
		props["target_type"] = *evt.TargetType
	}

	return analytics.AnalyticsEvent{
		ID:         uuid.New(),
		UserID:     userID,
		SessionID:  "system", // Backend events usually don't have a web session
		EventType:  string(evt.Action),
		Properties: props,
		CreatedAt:  evt.Timestamp,
		// IP/UserAgent might be missing for backend events unless in metadata
		IPAddress: "127.0.0.1",
	}
}
