package video

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type AutomationSubscriber struct {
	realtime realtime.Hub
	logger   log.Logger
}

func NewAutomationSubscriber(realtime realtime.Hub, logger log.Logger) *AutomationSubscriber {
	return &AutomationSubscriber{
		realtime: realtime,
		logger:   logger.With(log.String("component", "video_automation")),
	}
}

func (s *AutomationSubscriber) RegisterSubscribers(bus events.Bus) {
	bus.Subscribe(string(core.EventDonationReceived), s.HandleDonation)
	bus.Subscribe(string(core.EventGoalReached), s.HandleGoalReached)
}

func (s *AutomationSubscriber) HandleDonation(ctx context.Context, event events.DomainEvent) error {
	var payload core.DonationReceivedPayload
	if err := event.UnmarshalPayload(&payload); err != nil {
		return err
	}

	msg := fmt.Sprintf("New Donation: %s %s received! 🎉", payload.Amount, payload.Currency)

	// Broadcast to all active video channels
	// In a real scenario, we might want to filter by ProgramID or RoomID.
	// For MVP, we broadcast to a "global-impact" channel that the frontend listens to.
	return s.broadcastImpact(ctx, "Impact Milestone", msg, "donation")
}

func (s *AutomationSubscriber) HandleGoalReached(ctx context.Context, event events.DomainEvent) error {
	// Assume payload contains goal info
	msg := "Major Milestone: Funding goal reached for our clean water initiative! 🌊"
	return s.broadcastImpact(ctx, "Goal Reached", msg, "impact")
}

func (s *AutomationSubscriber) broadcastImpact(ctx context.Context, title, message, iType string) error {
	s.logger.Info("Broadcasting automated impact toast", log.String("title", title))

	payload := map[string]any{
		"title":   title,
		"message": message,
		"type":    iType,
	}

	// We publish to the "global:impact" channel
	return s.realtime.Publish(ctx, realtime.PublishRequest{
		Channel: "global:impact",
		Event:   "impact_toast",
		Data:    payload,
	})
}
