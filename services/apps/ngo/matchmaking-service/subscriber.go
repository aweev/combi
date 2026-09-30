package matchmaking

import (
	"context"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	service matchmaking.Service
	logger  log.Logger
}

func NewSubscriber(service matchmaking.Service, logger log.Logger) *Subscriber {
	return &Subscriber{service: service, logger: logger}
}

func (s *Subscriber) Handle(ctx context.Context, evt events.DomainEvent) error {
	switch evt.Action {
	case core.EventUserProfileUpdated:
		var p user.ProfileUpdatedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			return s.service.UpsertCandidate(ctx, matchmaking.MatchCandidate{
				UserID:      p.UserID,
				Type:        matchmaking.TypeMentorship,
				IsAvailable: true,
				Attributes: map[string]any{
					"skills":    p.Skills,
					"interests": p.Interests,
				},
			})
		}

	case core.EventOpportunityCreated:
		var p catalog.OpportunityCreatedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			return s.service.UpsertCandidate(ctx, matchmaking.MatchCandidate{
				UserID:      p.ID,
				Type:        matchmaking.TypeVolunteer,
				IsAvailable: true,
				Attributes: map[string]any{
					"skills":   p.RequiredSkills,
					"location": p.Location,
				},
			})
		}
	}
	return nil
}
