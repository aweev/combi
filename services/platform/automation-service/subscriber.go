package automation

import (
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	ingest automation.EventIngestService
	logger log.Logger
}

func NewSubscriber(ingest automation.EventIngestService, logger log.Logger) *Subscriber {
	return &Subscriber{ingest: ingest, logger: logger}
}

func (s *Subscriber) RegisterSubscribers(bus events.Bus) {
	// Subscribe to "Triggerable" events defined in Core
	// This list defines what events can trigger automation rules by default
	topics := []core.EventAction{
		core.EventUserCreated,
		core.EventUserLoggedIn,
		core.EventDonationReceived,
		core.EventFormSubmission,
		core.EventOpportunityCreated,
		core.EventProjectCompleted,
		core.EventContentPublished,
	}

	for _, t := range topics {
		bus.Subscribe(t.String(), s.handleAny)
	}
}

func (s *Subscriber) handleAny(ctx context.Context, evt events.DomainEvent) error {
	var data map[string]any
	if err := json.Unmarshal(evt.Payload, &data); err != nil {
		s.logger.Warn("automation: skipping malformed event", log.String("event_id", evt.ID.String()))
		return nil
	}

	// Standardize metadata for rule engine
	data["actor_id"] = evt.ActorID.String()
	data["event_id"] = evt.ID.String()
	for k, v := range evt.Metadata {
		data["meta_"+k] = v
	}

	autoEvent := automation.Event{
		ID:        evt.ID,
		Type:      evt.Action.String(), // Use the core constant string value as the trigger type
		Source:    "system",
		Timestamp: evt.Timestamp,
		Data:      data,
		ActorID:   &evt.ActorID,
	}

	return s.ingest.Ingest(ctx, autoEvent)
}
