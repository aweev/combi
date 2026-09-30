package campaign

import (
	"context"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	service campaign.Service
	logger  log.Logger
}

func NewSubscriber(service campaign.Service, logger log.Logger) *Subscriber {
	return &Subscriber{service: service, logger: logger.With(log.String("component", "campaign_subscriber"))}
}

func (s *Subscriber) Handle(ctx context.Context, evt events.DomainEvent) error {
	// Filter: We only care about successful payments
	if evt.Action != core.EventDonationReceived {
		return nil
	}

	// 1. Unmarshal Strong Payload
	var p financial.PaymentSucceededPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		s.logger.Warn("campaign: skipping malformed donation payload", log.ErrorAttr(err))
		return nil
	}

	// 2. Extract Linkage
	// Campaign ID is mandatory for attribution
	campIDStr, ok := p.Metadata["campaign_id"]
	if !ok || campIDStr == "" {
		return nil // Donation not linked to a campaign
	}
	campaignID, err := uuid.Parse(campIDStr)
	if err != nil {
		s.logger.Warn("campaign: invalid campaign_id in metadata", log.String("val", campIDStr))
		return nil
	}

	// Fundraiser ID is optional (Peer-to-Peer)
	var fundraiserID *uuid.UUID
	if fIDStr, ok := p.Metadata["fundraiser_id"]; ok && fIDStr != "" {
		if uid, err := uuid.Parse(fIDStr); err == nil {
			fundraiserID = &uid
		}
	}

	// 3. Inject Tenant Context
	// This is critical because downstream service/repo calls use WithTenant helpers
	orgCtx := &organization.OrgContext{OrgID: p.OrganizationID}
	ctx = organization.NewContextWithOrg(ctx, orgCtx)

	// 4. Delegate to Service
	return s.service.RecordDonation(ctx, p.TransactionID, campaignID, fundraiserID, p.Amount)
}
