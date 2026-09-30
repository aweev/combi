package campaign_test

import (
	"context"
	"testing"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/campaign"
	campaign_mocks "github.com/combi/pkg/domain/campaign/mocks"

	// Platform Mocks
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	campaign_svc "github.com/combi/campaign"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testSuite struct {
	repo    *campaign_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	bus     *events_mocks.MockBus
	service campaign.Service
}

func setupTest(t *testing.T) *testSuite {
	s := &testSuite{
		repo:  campaign_mocks.NewMockRepository(t),
		audit: audit_mocks.NewMockEventRecorder(t),
		bus:   events_mocks.NewMockBus(t),
	}
	s.service = campaign_svc.NewService(s.repo, s.audit, s.bus, &log.NoOpLogger{})
	return s
}

func TestService_CreateFundraiser(t *testing.T) {
	s := setupTest(t)
	ctx := context.Background()
	ownerID := uuid.New()
	campID := uuid.New()

	params := campaign.CreateFundraiserParams{
		CampaignID: campID,
		OwnerID:    ownerID,
		Name:       "My Run",
		Slug:       "my-run",
		GoalAmount: 1000,
	}

	t.Run("Success", func(t *testing.T) {
		// Mock Campaign fetch
		camp := &campaign.Campaign{
			ID:       campID,
			Status:   campaign.StatusActive,
			AllowP2P: true,
		}
		s.repo.EXPECT().GetCampaignByID(mock.Anything, campID).Return(camp, nil).Once()

		s.repo.EXPECT().CreateFundraiser(mock.Anything, mock.MatchedBy(func(f *campaign.Fundraiser) bool {
			return f.OwnerID == ownerID && f.Name == "My Run"
		})).Return(nil).Once()

		s.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// Added expectation for event bus publish
		s.bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(evt interface{}) bool {
			// In a real test, you might cast evt to events.DomainEvent and check fields
			return true
		})).Return(nil).Once()

		actCtx := auth.NewContextWithActor(ctx, &auth.Actor{UserID: ownerID})

		f, err := s.service.CreateFundraiser(actCtx, params)
		require.NoError(t, err)
		assert.Equal(t, "My Run", f.Name)
	})

	t.Run("Campaign Closed", func(t *testing.T) {
		camp := &campaign.Campaign{ID: campID, Status: campaign.StatusCompleted}
		s.repo.EXPECT().GetCampaignByID(ctx, campID).Return(camp, nil).Once()

		_, err := s.service.CreateFundraiser(ctx, params)
		assert.ErrorIs(t, err, campaign.ErrCampaignClosed)
	})

	t.Run("P2P Disabled", func(t *testing.T) {
		camp := &campaign.Campaign{ID: campID, Status: campaign.StatusActive, AllowP2P: false}
		s.repo.EXPECT().GetCampaignByID(ctx, campID).Return(camp, nil).Once()

		_, err := s.service.CreateFundraiser(ctx, params)
		assert.ErrorContains(t, err, "disabled")
	})
}

func TestService_RecordDonation(t *testing.T) {
	s := setupTest(t)
	ctx := context.Background()
	txID := uuid.New()
	campID := uuid.New()
	fundID := uuid.New()
	amount := int64(5000)

	t.Run("Success with Goal Met", func(t *testing.T) {
		// 1. Create Attribution
		s.repo.EXPECT().CreateAttribution(ctx, mock.MatchedBy(func(a *campaign.DonationAttribution) bool {
			return a.TransactionID == txID && a.Amount == amount
		})).Return(nil).Once()

		// 2. Increment Stats
		s.repo.EXPECT().IncrementStats(ctx, campID, &fundID, amount).Return(nil).Once()

		// 3. Check Goals - Campaign (Not Met)
		s.repo.EXPECT().GetCampaignByID(ctx, campID).Return(&campaign.Campaign{
			ID: campID, GoalAmount: 100000, TotalRaised: 10000,
		}, nil).Once()

		// 4. Check Goals - Fundraiser (Met!)
		s.repo.EXPECT().GetFundraiserByID(ctx, fundID).Return(&campaign.Fundraiser{
			ID: fundID, GoalAmount: 5000, TotalRaised: 5000, OwnerID: uuid.New(),
		}, nil).Once()

		// 5. Expect Event Publish
		s.bus.EXPECT().Publish(ctx, mock.MatchedBy(func(evt interface{}) bool {
			return true
		})).Return(nil).Once()

		err := s.service.RecordDonation(ctx, txID, campID, &fundID, amount)
		require.NoError(t, err)
	})
}
