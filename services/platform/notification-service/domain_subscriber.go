package notification

import (
	"context"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type DomainSubscriber struct {
	service notification.Service
	logger  log.Logger
}

func NewDomainSubscriber(service notification.Service, logger log.Logger) *DomainSubscriber {
	return &DomainSubscriber{
		service: service,
		logger:  logger.With(log.String("component", "notification_domain_subscriber")),
	}
}

// accountCreatedPayload mirrors the payload structure from Auth service
type accountCreatedPayload struct {
	UserID           uuid.UUID `json:"user_id"`
	Email            string    `json:"email"`
	VerificationLink string    `json:"verification_link"`
	Name             string    `json:"name"` // Optional, if available
}

func (s *DomainSubscriber) OnAccountCreated(ctx context.Context, evt events.DomainEvent) error {
	var p accountCreatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logger.Error("failed to unmarshal account created payload", err)
		return err
	}

	email, err := core.NewEmail(p.Email)
	if err != nil {
		s.logger.Error("invalid email in event payload", err)
		return err
	}

	// 1. Prepare Data for the Template
	// Your seeds/translations.json expects: {{.Link}} and {{.Name}}
	templateData := map[string]interface{}{
		"Link": p.VerificationLink,
		"Name": p.Name,
	}
	if templateData["Name"] == "" {
		templateData["Name"] = "Volunteer" // Fallback if name is empty
	}

	// 2. Dispatch using the TEMPLATE SLUG (defined in seeds/templates.json)
	_, err = s.service.Dispatch(ctx, notification.DispatchRequest{
		RecipientID: p.UserID,
		ToAddress:   email,
		Template: &notification.TemplatePayload{
			Slug: "auth-verify-email", // This MUST match seeds/templates.json
			Data: templateData,
		},
	})

	if err != nil {
		s.logger.Error("failed to dispatch verification email", err, log.String("user_id", p.UserID.String()))
		return err
	}

	s.logger.Info("verification email dispatched via template",
		log.String("user_id", p.UserID.String()),
		log.String("template", "auth-verify-email"),
	)
	return nil
}

// --- Magic Link Requested ---
type magicLinkPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	LoginLink string    `json:"login_link"`
}

func (s *DomainSubscriber) OnMagicLinkRequested(ctx context.Context, evt events.DomainEvent) error {
	var p magicLinkPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return err
	}

	email, _ := core.NewEmail(p.Email)

	_, err := s.service.Dispatch(ctx, notification.DispatchRequest{
		RecipientID: p.UserID,
		ToAddress:   email,
		Template: &notification.TemplatePayload{
			Slug: "auth-magic-link", // Matches seeded template
			Data: map[string]interface{}{
				"Link": p.LoginLink,
			},
		},
	})
	return err
}

// --- Password Reset Requested ---
type passwordResetPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	ResetLink string    `json:"reset_link"`
}

func (s *DomainSubscriber) OnPasswordResetRequested(ctx context.Context, evt events.DomainEvent) error {
	var p passwordResetPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return err
	}

	email, _ := core.NewEmail(p.Email)

	// Note: You need to seed a template for "auth-password-reset" first!
	// For now, we assume you will add it to seeds/templates.json
	_, err := s.service.Dispatch(ctx, notification.DispatchRequest{
		RecipientID: p.UserID,
		ToAddress:   email,
		Template: &notification.TemplatePayload{
			Slug: "auth-password-reset",
			Data: map[string]interface{}{
				"Link": p.ResetLink,
			},
		},
	})
	return err
}

// --- Organization Invitation ---
type invitePayload struct {
	InviteeEmail string `json:"invitee_email"`
	OrgName      string `json:"org_name"`
	InviteURL    string `json:"invite_url"`
	// UserID might be nil if they aren't registered yet, handled by service logic
}

func (s *DomainSubscriber) OnOrganizationInvite(ctx context.Context, evt events.DomainEvent) error {
	var p invitePayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return err
	}

	email, _ := core.NewEmail(p.InviteeEmail)

	// For invites, we might not have a RecipientID yet if the user is new.
	// The DispatchRequest might need to allow Nil UUID or handle "guest" recipients.
	// For now, assume we handle it or pass uuid.Nil.

	_, err := s.service.Dispatch(ctx, notification.DispatchRequest{
		RecipientID: uuid.Nil, // Or find/create placeholder
		ToAddress:   email,
		Template: &notification.TemplatePayload{
			Slug: "org-invite", // Matches seeded template
			Data: map[string]interface{}{
				"OrgName": p.OrgName,
				"Link":    p.InviteURL,
			},
		},
	})
	return err
}
