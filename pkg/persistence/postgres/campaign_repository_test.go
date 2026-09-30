package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCampaignRepository_CampaignCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewCampaignRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	camp := &campaign.Campaign{
		ID:         uuid.New(),
		Slug:       "winter-2024",
		Name:       "Winter Appeal",
		GoalAmount: 5000000, // 50k
		Currency:   "EUR",
		Status:     campaign.StatusDraft,
		AllowP2P:   true,
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}

	t.Run("CreateAndGet", func(t *testing.T) {
		err := repo.CreateCampaign(ctx, camp)
		require.NoError(t, err)

		// By ID
		got, err := repo.GetCampaignByID(ctx, camp.ID)
		require.NoError(t, err)
		assert.Equal(t, camp.Name, got.Name)

		// By Slug
		gotSlug, err := repo.GetCampaignBySlug(ctx, "winter-2024")
		require.NoError(t, err)
		assert.Equal(t, camp.ID, gotSlug.ID)
	})

	t.Run("Update", func(t *testing.T) {
		camp.Status = campaign.StatusActive
		err := repo.UpdateCampaign(ctx, camp)
		require.NoError(t, err)

		got, _ := repo.GetCampaignByID(ctx, camp.ID)
		assert.Equal(t, campaign.StatusActive, got.Status)
	})

	t.Run("DuplicateSlug", func(t *testing.T) {
		dup := *camp
		dup.ID = uuid.New()
		err := repo.CreateCampaign(ctx, &dup)
		assert.ErrorIs(t, err, campaign.ErrSlugTaken)
	})
}

func TestCampaignRepository_FundraiserCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()
	repo := postgres.NewCampaignRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Setup parent campaign
	camp := &campaign.Campaign{
		ID:        uuid.New(),
		Slug:      "main",
		Name:      "Main",
		Status:    campaign.StatusDraft,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateCampaign(ctx, camp))

	f := &campaign.Fundraiser{
		ID:         uuid.New(),
		CampaignID: camp.ID,
		OwnerID:    uuid.New(),
		Slug:       "johns-page",
		Name:       "John's Run",
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}

	t.Run("CreateAndGet", func(t *testing.T) {
		require.NoError(t, repo.CreateFundraiser(ctx, f))

		got, err := repo.GetFundraiserBySlug(ctx, "johns-page")
		require.NoError(t, err)
		assert.Equal(t, f.ID, got.ID)
	})

	t.Run("ListFundraisers", func(t *testing.T) {
		list, total, err := repo.ListFundraisers(ctx, camp.ID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, f.ID, list[0].ID)
	})
}

func TestCampaignRepository_Stats(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()
	repo := postgres.NewCampaignRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	camp := &campaign.Campaign{
		ID:        uuid.New(),
		Slug:      "stats-camp",
		Name:      "Stats",
		Status:    campaign.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateCampaign(ctx, camp))

	f := &campaign.Fundraiser{ID: uuid.New(), CampaignID: camp.ID, OwnerID: uuid.New(), Slug: "stats-fund", Name: "F", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.CreateFundraiser(ctx, f))

	t.Run("Attribution and Increment", func(t *testing.T) {
		amount := int64(5000) // 50.00
		attr := &campaign.DonationAttribution{
			TransactionID: uuid.New(),
			CampaignID:    camp.ID,
			FundraiserID:  &f.ID,
			Amount:        amount,
			CreatedAt:     time.Now(),
		}

		// 1. Create Attribution Record
		require.NoError(t, repo.CreateAttribution(ctx, attr))

		// 2. Increment Stats
		require.NoError(t, repo.IncrementStats(ctx, camp.ID, &f.ID, amount))

		// 3. Verify
		c, _ := repo.GetCampaignByID(ctx, camp.ID)
		assert.Equal(t, amount, c.TotalRaised)
		assert.Equal(t, 1, c.DonorCount)

		fund, _ := repo.GetFundraiserByID(ctx, f.ID)
		assert.Equal(t, amount, fund.TotalRaised)
		assert.Equal(t, 1, fund.DonorCount)
	})
}
