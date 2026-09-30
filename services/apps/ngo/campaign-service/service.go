package campaign

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ campaign.Service = (*service)(nil)
var _ campaign.AdminService = (*service)(nil)

type service struct {
	repo     campaign.Repository
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewService(repo campaign.Repository, audit audit.EventRecorder, eventBus events.Bus, logger log.Logger) *service {
	return &service{
		repo:     repo,
		audit:    audit,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "campaign")),
	}
}

// --- Public Read ---

func (s *service) GetCampaign(ctx context.Context, slugOrID string) (*campaign.Campaign, error) {
	if id, err := uuid.Parse(slugOrID); err == nil {
		return s.repo.GetCampaignByID(ctx, id)
	}
	return s.repo.GetCampaignBySlug(ctx, slugOrID)
}

func (s *service) ListActiveCampaigns(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*campaign.Campaign, error) {
	status := campaign.StatusActive
	list, _, err := s.repo.ListCampaigns(ctx, campaign.CampaignFilter{
		OrganizationID: orgID,
		Status:         &status,
		Limit:          limit,
		Offset:         offset,
	})
	return list, err
}

func (s *service) GetFundraiser(ctx context.Context, slugOrID string) (*campaign.Fundraiser, error) {
	if id, err := uuid.Parse(slugOrID); err == nil {
		return s.repo.GetFundraiserByID(ctx, id)
	}
	return s.repo.GetFundraiserBySlug(ctx, slugOrID)
}

func (s *service) ListCampaignFundraisers(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]*campaign.Fundraiser, error) {
	list, _, err := s.repo.ListFundraisers(ctx, campaignID, limit, offset)
	return list, err
}

// --- Volunteer Actions (Peer-to-Peer) ---

func (s *service) CreateFundraiser(ctx context.Context, params campaign.CreateFundraiserParams) (*campaign.Fundraiser, error) {
	// 1. Validate Campaign
	camp, err := s.repo.GetCampaignByID(ctx, params.CampaignID)
	if err != nil {
		return nil, err
	}
	if camp.Status != campaign.StatusActive {
		return nil, campaign.ErrCampaignClosed
	}
	if !camp.AllowP2P {
		return nil, fmt.Errorf("peer-to-peer fundraising is disabled for this campaign")
	}

	// 2. Validate Ownership
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != params.OwnerID {
		return nil, auth.ErrUnauthorized
	}

	// 3. Create
	f := &campaign.Fundraiser{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		CampaignID:     params.CampaignID,
		OwnerID:        params.OwnerID,
		Name:           params.Name,
		Slug:           params.Slug,
		Description:    params.Description,
		GoalAmount:     params.GoalAmount,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateFundraiser(ctx, f); err != nil {
		return nil, err
	}

	s.auditEvent(ctx, params.OwnerID, core.EventFundraiserCreated, f.ID, f.Name)

	payload := campaign.FundraiserCreatedPayload{
		FundraiserID: f.ID,
		CampaignID:   f.CampaignID,
		OwnerID:      f.OwnerID,
		Name:         f.Name,
		GoalAmount:   f.GoalAmount,
	}
	s.publishEvent(ctx, core.EventFundraiserCreated, f.OwnerID, "user", payload)

	return f, nil
}

func (s *service) UpdateFundraiser(ctx context.Context, fundraiserID uuid.UUID, params campaign.UpdateFundraiserParams) (*campaign.Fundraiser, error) {
	f, err := s.repo.GetFundraiserByID(ctx, fundraiserID)
	if err != nil {
		return nil, err
	}

	// Authz: Only owner can update
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != f.OwnerID {
		return nil, auth.ErrUnauthorized
	}

	if params.Name != nil {
		f.Name = *params.Name
	}
	if params.Description != nil {
		f.Description = *params.Description
	}
	if params.GoalAmount != nil {
		f.GoalAmount = *params.GoalAmount
	}

	if err := s.repo.UpdateFundraiser(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// --- Admin ---

func (s *service) CreateCampaign(ctx context.Context, params campaign.CreateCampaignParams) (*campaign.Campaign, error) {
	c := &campaign.Campaign{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		Name:           params.Name,
		Slug:           params.Slug,
		Description:    params.Description,
		GoalAmount:     params.GoalAmount,
		Currency:       params.Currency,
		Status:         campaign.StatusDraft,
		AllowP2P:       params.AllowP2P,
		StartDate:      params.StartDate,
		EndDate:        params.EndDate,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateCampaign(ctx, c); err != nil {
		return nil, err
	}

	// Assume admin context for actor ID, or system ID fallback
	var actorID uuid.UUID
	if actor := auth.ActorFromContext(ctx); actor != nil {
		actorID = actor.UserID
	}

	s.auditEvent(ctx, actorID, core.EventCampaignCreated, c.ID, c.Name)
	return c, nil
}

func (s *service) UpdateCampaign(ctx context.Context, id uuid.UUID, params campaign.UpdateCampaignParams) (*campaign.Campaign, error) {
	c, err := s.repo.GetCampaignByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		c.Name = *params.Name
	}
	if params.Description != nil {
		c.Description = *params.Description
	}
	if params.GoalAmount != nil {
		c.GoalAmount = *params.GoalAmount
	}
	if params.Status != nil {
		c.Status = *params.Status
	}
	if params.IsFeatured != nil {
		c.IsFeatured = *params.IsFeatured
	}

	if err := s.repo.UpdateCampaign(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) DeleteCampaign(ctx context.Context, id uuid.UUID) error {
	status := campaign.StatusArchived
	_, err := s.UpdateCampaign(ctx, id, campaign.UpdateCampaignParams{Status: &status})
	return err
}

// --- Internal / Async Handlers ---

func (s *service) RecordDonation(ctx context.Context, transactionID, campaignID uuid.UUID, fundraiserID *uuid.UUID, amount int64) error {
	s.logger.Info("Recording donation for campaign",
		log.String("campaign_id", campaignID.String()),
		log.Int("amount", int(amount)))

	// 1. Create Attribution Record
	orgCtx := organization.OrgFromContext(ctx)
	orgID := uuid.Nil
	if orgCtx != nil {
		orgID = orgCtx.OrgID
	}

	attr := &campaign.DonationAttribution{
		TransactionID:  transactionID,
		OrganizationID: orgID,
		CampaignID:     campaignID,
		FundraiserID:   fundraiserID,
		Amount:         amount,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateAttribution(ctx, attr); err != nil {
		// Log warning on duplicate but don't fail, allowing idempotency
		s.logger.Warn("Failed to create attribution (possibly duplicate)", log.ErrorAttr(err))
		return nil
	}

	// 2. Increment Stats
	if err := s.repo.IncrementStats(ctx, campaignID, fundraiserID, amount); err != nil {
		return err
	}

	// 3. Check Goals & Emit Events
	camp, err := s.repo.GetCampaignByID(ctx, campaignID)
	if err == nil && camp.TotalRaised >= camp.GoalAmount {
		s.publishEvent(ctx, core.EventCampaignCompleted, uuid.Nil, "system", map[string]string{
			"campaign_id": campaignID.String(),
		})
	}

	if fundraiserID != nil {
		fund, err := s.repo.GetFundraiserByID(ctx, *fundraiserID)
		if err == nil && fund.TotalRaised >= fund.GoalAmount {
			s.publishEvent(ctx, core.EventFundraiserGoalMet, fund.OwnerID, "user", map[string]string{
				"fundraiser_id": fund.ID.String(),
			})
		}
	}

	return nil
}

// --- Helpers ---

func (s *service) auditEvent(ctx context.Context, actorID uuid.UUID, action core.EventAction, targetID uuid.UUID, name string) {
	evt := audit.NewEvent(actorID, "user", action).
		WithTarget(targetID, "campaign", name).
		WithSuccess().
		WithContext(ctx).Build()
	if err := s.audit.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *service) publishEvent(ctx context.Context, action core.EventAction, actorID uuid.UUID, actorType string, payload any) {
	evt, err := events.NewEvent(action, actorID, actorType, payload)
	if err != nil {
		s.logger.Error("Failed to create event", err)
		return
	}
	if err := s.eventBus.Publish(ctx, evt); err != nil {
		s.logger.Error("Failed to publish event", err)
	}
}
