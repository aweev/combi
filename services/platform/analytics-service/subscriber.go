package analytics

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	service analytics.Service
	logger  log.Logger
}

func NewSubscriber(service analytics.Service, logger log.Logger) *Subscriber {
	return &Subscriber{
		service: service,
		logger:  logger.With(log.String("component", "analytics_subscriber")),
	}
}

// Handle adapts DomainEvent -> AnalyticsEvent.
// Since analytics tracks "everything", we use a generic handler but optimize for key events.
func (s *Subscriber) Handle(ctx context.Context, evt events.DomainEvent) error {
	// Filter noise
	if evt.Action == "job.outcomes.batched" {
		return nil
	}

	properties := make(map[string]any)

	// 1. Try Specific Unmarshal for rich data extraction
	switch evt.Action {
	case core.EventUserCreated:
		var p user.UserCreatedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			properties["email_domain"] = extractDomain(p.Email)
			properties["role"] = p.Role
		}
	case core.EventDonationReceived:
		var p financial.PaymentSucceededPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			properties["amount"] = p.Amount // Cents
			properties["currency"] = p.Currency
			properties["provider"] = p.Provider
		}
	case core.EventUserLoggedIn:
		var p auth.LoginPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			properties["method"] = p.Method
			properties["ip"] = p.IPAddress // Important for geo-analytics
		}
	default:
		// 2. Generic Fallback: Flatten JSON payload
		_ = json.Unmarshal(evt.Payload, &properties)
	}

	// 3. Inject Metadata (TraceID, Context)
	for k, v := range evt.Metadata {
		properties["meta_"+k] = v
	}

	// 4. Resolve Actor
	var userID *uuid.UUID
	// If the event was triggered by a user, link it.
	if evt.ActorType == "user" && evt.ActorID != uuid.Nil {
		uid := evt.ActorID
		userID = &uid
	}

	// 5. Track Async
	return s.service.TrackEvent(ctx, analytics.AnalyticsEvent{
		ID:         uuid.New(),
		UserID:     userID,
		SessionID:  "system", // Backend events often lack a user session
		EventType:  evt.Action.String(),
		Properties: properties,
		CreatedAt:  evt.Timestamp,
		IPAddress:  "127.0.0.1", // Default if not in payload
	})
}

// Simple helper to get "gmail.com" from "foo@gmail.com"
func extractDomain(email string) string {
	// Find the last index of "@" to handle cases where the local part
	// might contain a quoted "@" (though rare, it is valid RFC).
	i := strings.LastIndex(email, "@")

	// Check if "@" was found and ensures it is not the last character
	if i >= 0 && i < len(email)-1 {
		return email[i+1:]
	}

	return "unknown"
}
