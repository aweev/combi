package organization

import (
	"context"
	"fmt"

	// Need auth events
	"github.com/google/uuid"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type EventSubscriber struct {
	repo    organization.Repository
	service organization.Service
	logger  log.Logger
}

func NewEventSubscriber(repo organization.Repository, service organization.Service, logger log.Logger) *EventSubscriber {
	return &EventSubscriber{repo, service, logger.With(log.String("component", "org_event_subscriber"))}
}

// HandleEmailVerified listens for `auth.email.verified`.
// This ensures we only auto-add users who have proven they own the email.
func (s *EventSubscriber) HandleEmailVerified(ctx context.Context, event events.DomainEvent) error {
	// We expect the payload to contain the UserID and potentially the Email.
	// If the event payload schema for `auth.email.verified` is just UserID, we fetch the user.
	// Assuming the payload is similar to UserCreated or has the necessary info.

	// For this example, let's assume the payload is map[string]string{"user_id": "...", "email": "..."}
	// or a struct.
	var payload struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}

	if err := event.UnmarshalPayload(&payload); err != nil {
		return nil // Non-retryable
	}

	userID, _ := uuid.Parse(payload.UserID)

	s.logger.Info("Checking pending invitations for verified email",
		log.String("email", payload.Email),
		log.String("user_id", payload.UserID),
	)

	// 1. List pending invites
	invitations, err := s.repo.ListPendingInvitationsByEmail(ctx, payload.Email)
	if err != nil {
		return fmt.Errorf("failed to list pending invitations: %w", err)
	}

	if len(invitations) == 0 {
		return nil
	}

	// 2. Auto-accept
	for _, inv := range invitations {
		s.logger.Info("Auto-accepting invitation", log.String("invitation_id", inv.ID.String()))

		// Use the INTERNAL method to skip token check
		if err := s.service.AcceptInvitationInternal(ctx, inv.ID, userID); err != nil {
			s.logger.Error("Failed to auto-accept invitation", err, log.String("invitation_id", inv.ID.String()))
			// Continue to next invite
		}
	}

	return nil
}
