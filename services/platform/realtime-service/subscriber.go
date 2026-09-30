package realtime

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type EventSubscriber struct {
	hub    realtime.Hub
	logger log.Logger
}

func NewEventSubscriber(hub realtime.Hub, logger log.Logger) *EventSubscriber {
	return &EventSubscriber{hub: hub, logger: logger}
}

func (s *EventSubscriber) RegisterSubscribers(bus events.Bus) {
	bus.Subscribe(core.EventMessageCreated.String(), s.handleChatMessage)
	bus.Subscribe(core.EventNotificationSent.String(), s.handleNotification)
	bus.Subscribe(core.EventImpactMetricUpdated.String(), s.handleImpactUpdate)
}

func (s *EventSubscriber) handleChatMessage(ctx context.Context, evt events.DomainEvent) error {
	var p messaging.MessageCreatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logger.Warn("realtime: failed to unmarshal chat message", log.ErrorAttr(err))
		return nil
	}

	return s.hub.Publish(ctx, realtime.PublishRequest{
		Channel: realtime.ChannelID(fmt.Sprintf("chat:room:%s", p.RoomID)),
		Event:   "message.new",
		Data:    p,
	})
}

func (s *EventSubscriber) handleNotification(ctx context.Context, evt events.DomainEvent) error {
	// Assumes notification domain defines this payload
	var p notification.NotificationSentPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}

	return s.hub.Publish(ctx, realtime.PublishRequest{
		Channel: realtime.ChannelID(fmt.Sprintf("user:%s", p.RecipientID)),
		Event:   "notification.new",
		Data:    p,
	})
}

func (s *EventSubscriber) handleImpactUpdate(ctx context.Context, evt events.DomainEvent) error {
	var p impact.MetricsUpdatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}

	return s.hub.Publish(ctx, realtime.PublishRequest{
		Channel: "public:impact",
		Event:   "stats.update",
		Data:    p,
	})
}
