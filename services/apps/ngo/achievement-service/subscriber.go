package achievement

import (
	"context"

	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	service achievement.Service
	logger  log.Logger
}

func NewSubscriber(service achievement.Service, logger log.Logger) *Subscriber {
	return &Subscriber{
		service: service,
		logger:  logger.With(log.String("component", "achievement_subscriber")),
	}
}

// HandleLogin processes 'auth.user.loggedin'
func (s *Subscriber) HandleLogin(ctx context.Context, evt events.DomainEvent) error {
	var p auth.LoginPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logError(evt, err)
		return nil
	}
	return s.service.ProcessEvent(ctx, p.UserID, core.EventUserLoggedIn, map[string]any{
		"method": p.Method,
	})
}

// HandleDonation processes 'financial.donation.received'
func (s *Subscriber) HandleDonation(ctx context.Context, evt events.DomainEvent) error {
	var p financial.PaymentSucceededPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logError(evt, err)
		return nil
	}
	return s.service.ProcessEvent(ctx, p.UserID, core.EventDonationReceived, map[string]any{
		"amount":   p.Amount, // In cents
		"currency": p.Currency,
	})
}

// HandleVolunteerHours processes 'project.hours.logged'
func (s *Subscriber) HandleVolunteerHours(ctx context.Context, evt events.DomainEvent) error {
	var p project.HoursLoggedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logError(evt, err)
		return nil
	}
	return s.service.ProcessEvent(ctx, p.UserID, core.EventHoursLogged, map[string]any{
		"hours":      p.Hours,
		"project_id": p.ProjectID.String(),
	})
}

// HandleProjectCompletion processes 'project.completed'
func (s *Subscriber) HandleProjectCompletion(ctx context.Context, evt events.DomainEvent) error {
	var p project.ProjectCompletedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logError(evt, err)
		return nil
	}
	return s.service.ProcessEvent(ctx, p.ManagerID, core.EventProjectCompleted, map[string]any{
		"project_id": p.ProjectID.String(),
	})
}

func (s *Subscriber) logError(evt events.DomainEvent, err error) {
	s.logger.Warn("achievement: failed to unmarshal payload",
		log.String("event", evt.Action.String()),
		log.ErrorAttr(err),
	)
}
