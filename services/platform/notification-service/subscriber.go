package notification

import (
	"context"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	listener *EventListener
	logger   log.Logger
}

func NewSubscriber(listener *EventListener, logger log.Logger) *Subscriber {
	return &Subscriber{listener: listener, logger: logger}
}

func (s *Subscriber) RegisterSubscribers(bus events.Bus) {
	bus.Subscribe(core.EventEmailBounced.String(), s.handleEmailBounce)
	bus.Subscribe(core.EventSMSFailed.String(), s.handleSMSFailure)
}

func (s *Subscriber) handleEmailBounce(ctx context.Context, evt events.DomainEvent) error {
	var p notification.EmailBouncedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}
	return s.listener.HandleEmailBounced(ctx, evt) // Listener expects DomainEvent or Params, adjust accordingly
}

func (s *Subscriber) handleSMSFailure(ctx context.Context, evt events.DomainEvent) error {
	var p notification.SMSFailedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}
	return s.listener.HandleSMSFailed(ctx, evt)
}
