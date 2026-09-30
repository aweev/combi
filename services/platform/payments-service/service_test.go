package payments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/privacy"

	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	fin_mocks "github.com/combi/pkg/domain/financial/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	event_mocks "github.com/combi/pkg/events/mocks"

	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	fin_service "github.com/combi/payments"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Test Setup ---

type serviceTestSuite struct {
	repo    *fin_mocks.MockRepository
	factory *fin_mocks.MockPaymentGatewayFactory
	gateway *fin_mocks.MockPaymentGateway
	ledger  *fin_mocks.MockLedger
	audit   *audit_mocks.MockEventRecorder
	queue   *queue_mocks.MockDispatcher
	bus     *event_mocks.MockBus
	service *fin_service.Service
	actor   *auth.Actor
	ctx     context.Context
}

func setupTest(t *testing.T) *serviceTestSuite {
	s := &serviceTestSuite{
		repo:    fin_mocks.NewMockRepository(t),
		factory: fin_mocks.NewMockPaymentGatewayFactory(t),
		gateway: fin_mocks.NewMockPaymentGateway(t),
		ledger:  fin_mocks.NewMockLedger(t),
		audit:   audit_mocks.NewMockEventRecorder(t),
		queue:   queue_mocks.NewMockDispatcher(t),
		bus:     event_mocks.NewMockBus(t),
		actor:   &auth.Actor{UserID: uuid.New()},
	}

	s.service = fin_service.NewService(
		s.repo, s.factory, s.ledger, s.audit, s.queue, s.bus, &log.NoOpLogger{},
	)
	s.ctx = auth.NewContextWithActor(context.Background(), s.actor)

	return s
}

// Helper to enable Tx mocking only when needed
func (s *serviceTestSuite) mockTx() {
	s.repo.EXPECT().WithTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(financial.Repository) error) error {
			return fn(s.repo) // Pass the mock repo itself as the txRepo
		})
}

// --- Privacy Tests ---

func TestService_AnonymizeUser(t *testing.T) {
	s := setupTest(t)
	userID := uuid.New()

	// 1. Count
	s.repo.EXPECT().CountPaymentMethodsForUser(mock.Anything, userID).Return(int64(2), nil).Once()

	// 2. Anonymize
	s.repo.EXPECT().AnonymizeUserRecords(mock.Anything, userID).Return(nil).Once()

	report, err := s.service.AnonymizeUser(s.ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.RecordsDeleted)
	assert.Equal(t, "Financial", report.Service)
	// Check Interface Compliance
	var _ privacy.Anonymizer = s.service
}

// --- CreateOneTimePayment Tests ---

func TestService_CreateOneTimePayment(t *testing.T) {
	s := setupTest(t)
	token := "tok_visa"
	params := financial.CreatePaymentParams{
		UserID:         s.actor.UserID,
		Amount:         1000,
		Currency:       "USD",
		Description:    "Test Donation",
		ProviderToken:  &token,
		IdempotencyKey: "idem_123",
	}

	// Common Mocks setup
	s.factory.EXPECT().GetPaymentGateway(mock.Anything).Return(s.gateway, nil)
	s.gateway.EXPECT().ProviderName().Return("stripe")

	t.Run("Success", func(t *testing.T) {
		s.mockTx() // Enable Tx for this run

		// Customer Setup
		s.repo.EXPECT().GetCustomerByUserID(mock.Anything, params.UserID).Return(nil, financial.ErrCustomerNotFound).Once()
		s.gateway.EXPECT().CreateCustomer(mock.Anything, params.UserID, "", "").Return("cus_123", nil).Once()
		s.repo.EXPECT().CreateCustomer(mock.Anything, mock.Anything).Return(nil).Once()

		// 1. Gateway Charge
		s.gateway.EXPECT().Charge(mock.Anything, mock.MatchedBy(func(req financial.ChargeRequest) bool {
			return req.Amount == 1000 && req.PaymentToken == "tok_visa" && *req.ProviderCustomerID == "cus_123"
		})).Return(&financial.TransactionResult{
			Success:               true,
			ProviderTransactionID: "ch_123",
			Status:                financial.StatusSucceeded,
			Provider:              "stripe",
			Amount:                1000,
			Currency:              "USD",
		}, nil).Once()

		// 2. DB Creation
		s.repo.EXPECT().CreateTransaction(mock.Anything, mock.MatchedBy(func(tx *financial.Transaction) bool {
			return tx.Status == financial.StatusSucceeded && tx.ProviderID == "ch_123"
		})).Return(nil).Once()

		// 3. Ledger Posting
		acc1 := &financial.Account{ID: uuid.New(), Name: "Stripe Clearing"}
		acc2 := &financial.Account{ID: uuid.New(), Name: "General Revenue"}
		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "Stripe Clearing").Return(acc1, nil).Once()
		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "General Revenue").Return(acc2, nil).Once()

		s.ledger.EXPECT().Post(mock.Anything, s.repo, mock.MatchedBy(func(j *financial.JournalEntry) bool {
			return len(j.Entries) == 2 && j.Entries[0].Amount == 1000
		})).Return(nil).Once()

		// 4. Audit & Events
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventPaymentSucceeded && e.Outcome == audit.OutcomeSuccess
		})).Return(nil).Once()

		s.bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventPaymentSucceeded
		})).Return(nil).Once()

		// Execute
		tx, err := s.service.CreateOneTimePayment(s.ctx, params)
		require.NoError(t, err)
		assert.Equal(t, financial.StatusSucceeded, tx.Status)
	})

	t.Run("Gateway Failure", func(t *testing.T) {
		// NOTE: No mockTx() here because CreateOneTimePayment fails BEFORE WithTx is called if Charge fails.

		// Customer Setup (Must happen even for failure case, as it's called before Charge)
		s.repo.EXPECT().GetCustomerByUserID(mock.Anything, params.UserID).Return(&financial.Customer{ProviderID: "cus_existing"}, nil).Once()

		// Mock Gateway Error
		s.gateway.EXPECT().Charge(mock.Anything, mock.Anything).
			Return(nil, errors.New("connection error")).Once()

		// Audit Failure
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Outcome == audit.OutcomeFailed && e.Action == core.EventPaymentFailed
		})).Return(nil).Once()

		// Execute
		tx, err := s.service.CreateOneTimePayment(s.ctx, params)
		assert.Error(t, err)
		assert.Nil(t, tx)
	})
}

// --- CreateSubscription Tests ---

func TestService_CreateSubscription(t *testing.T) {
	s := setupTest(t)
	priceID := uuid.New()
	interval := financial.IntervalMonth

	params := financial.CreateSubscriptionParams{
		UserID:        s.actor.UserID,
		PriceID:       priceID,
		ProviderToken: func() *string { s := "tok_visa"; return &s }(),
	}

	// Common Mocks
	s.factory.EXPECT().GetPaymentGateway(mock.Anything).Return(s.gateway, nil)
	s.gateway.EXPECT().ProviderName().Return("stripe")

	t.Run("Success", func(t *testing.T) {
		s.mockTx() // Enable Tx

		// Customer Setup
		existingCust := &financial.Customer{ID: uuid.New(), UserID: s.actor.UserID, ProviderID: "cus_999"}
		s.repo.EXPECT().GetCustomerByUserID(mock.Anything, s.actor.UserID).Return(existingCust, nil).Once()

		// 1. Get Price
		s.repo.EXPECT().GetPrice(mock.Anything, priceID).Return(&financial.Price{
			ID: priceID, ProviderPriceID: "price_123", IsActive: true, Interval: &interval,
		}, nil).Once()

		// 2. Gateway Subscription
		s.gateway.EXPECT().StartRecurringPayment(mock.Anything, mock.MatchedBy(func(req financial.RecurringPaymentRequest) bool {
			return req.ProviderCustomerID == "cus_999" && req.ProviderPriceID == "price_123"
		})).Return(&financial.RecurringPaymentResult{
			Success:                true,
			ProviderSubscriptionID: "sub_123",
			Status:                 financial.SubStatusActive,
		}, nil).Once()

		// 3. DB Creation
		s.repo.EXPECT().CreateSubscription(mock.Anything, mock.MatchedBy(func(sub *financial.Subscription) bool {
			return sub.ProviderID == "sub_123" && sub.Status == financial.SubStatusActive
		})).Return(nil).Once()

		// 4. Audit & Events
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Outcome == audit.OutcomeSuccess
		})).Return(nil).Once()

		s.bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventSubscriptionCreated
		})).Return(nil).Once()

		// Execute
		sub, err := s.service.CreateSubscription(s.ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "sub_123", sub.ProviderID)
	})

	t.Run("Inactive Price", func(t *testing.T) {
		s.mockTx() // Enable Tx (Validation happens inside Tx)

		// Customer Setup (Happens before Tx)
		existingCust := &financial.Customer{ID: uuid.New(), UserID: s.actor.UserID, ProviderID: "cus_999"}
		s.repo.EXPECT().GetCustomerByUserID(mock.Anything, s.actor.UserID).Return(existingCust, nil).Once()

		s.repo.EXPECT().GetPrice(mock.Anything, priceID).Return(&financial.Price{
			ID: priceID, IsActive: false, // Inactive
		}, nil).Once()

		// Audit for failure
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Outcome == audit.OutcomeFailed && e.Action == core.EventSubscriptionFailed
		})).Return(nil).Once()

		_, err := s.service.CreateSubscription(s.ctx, params)
		assert.ErrorIs(t, err, financial.ErrInvalidInput)
	})
}
