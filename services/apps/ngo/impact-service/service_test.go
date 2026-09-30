package impact_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	impact_service "github.com/combi/impact"
	"github.com/combi/pkg/testutil"
)

func TestImpactContract_DonationFlow(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	logger := &log.NoOpLogger{}
	repo := postgres.NewImpactRepository(db, logger)

	// Use actual bus or mock that returns nil on Publish
	bus := events_mocks.NewMockBus(t)
	bus.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := impact_service.NewService(repo, bus, logger)
	worker := impact_service.NewEventWorker(svc, repo, logger)

	// 1. Setup Configuration
	metricID := uuid.New()
	_ = repo.CreateMetric(ctx, &impact.MetricDefinition{
		ID: metricID, Key: "funds_raised", Name: "Funds", Unit: "USD", Type: "CUMULATIVE",
	})

	// This mapping tells the worker: when "donation.succeeded" happens,
	// look into "data.amount" and record it.
	_ = repo.CreateMapping(ctx, impact.EventMapping{
		EventType:  "donation.succeeded",
		MetricID:   metricID,
		ValuePath:  "data.amount",
		Multiplier: 1.0,
	})

	// 2. Simulate External Event
	donationPayload := map[string]any{
		"data": map[string]any{
			"amount": 100.50,
		},
	}

	evt, err := events.NewEvent("donation.succeeded", uuid.New(), "payment-service", donationPayload)
	require.NoError(t, err)

	// 3. Execute Worker
	err = worker.HandleEvent(ctx, evt)
	require.NoError(t, err)

	// 4. Verify
	// Sum for the metric around the current time
	val, err := repo.SumMetric(ctx, metricID, time.Now().Add(-1*time.Minute), time.Now().Add(1*time.Minute))
	require.NoError(t, err)

	assert.Equal(t, 100.50, val, "The worker failed to dynamically extract and record the impact value")
}
