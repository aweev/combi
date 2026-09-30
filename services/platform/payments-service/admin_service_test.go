package payments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"

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

type adminServiceTestSuite struct {
	repo    *fin_mocks.MockRepository
	gateway *fin_mocks.MockPaymentGateway
	ledger  *fin_mocks.MockLedger
	audit   *audit_mocks.MockEventRecorder
	queue   *queue_mocks.MockDispatcher
	bus     *event_mocks.MockBus
	service *fin_service.AdminService
	admin   *auth.Actor
	ctx     context.Context
}

func setupAdminTest(t *testing.T) *adminServiceTestSuite {
	s := &adminServiceTestSuite{
		repo:    fin_mocks.NewMockRepository(t),
		gateway: fin_mocks.NewMockPaymentGateway(t),
		ledger:  fin_mocks.NewMockLedger(t),
		audit:   audit_mocks.NewMockEventRecorder(t),
		queue:   queue_mocks.NewMockDispatcher(t),
		bus:     event_mocks.NewMockBus(t),
		admin:   &auth.Actor{UserID: uuid.New()},
	}

	s.service = fin_service.NewAdminService(
		s.repo, s.gateway, s.ledger, s.audit, s.queue, s.bus, &log.NoOpLogger{},
	)
	s.ctx = auth.NewContextWithActor(context.Background(), s.admin)

	return s
}

func (s *adminServiceTestSuite) mockTx() {
	s.repo.EXPECT().WithTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(financial.Repository) error) error {
			return fn(s.repo)
		})
}

// --- Refund Tests ---

func TestAdminService_IssueRefund(t *testing.T) {
	s := setupAdminTest(t)
	txID := uuid.New()
	originalTx := &financial.Transaction{
		ID:         txID,
		UserID:     func() *uuid.UUID { u := uuid.New(); return &u }(),
		Status:     financial.StatusSucceeded,
		Amount:     1000,
		Currency:   "USD",
		Provider:   "stripe",
		ProviderID: "ch_123",
	}

	t.Run("Success", func(t *testing.T) {
		s.mockTx()

		// 1. Get Transaction
		s.repo.EXPECT().GetTransaction(mock.Anything, txID).Return(originalTx, nil).Once()

		// 2. Gateway Refund
		s.gateway.EXPECT().Refund(mock.Anything, "ch_123", int64(500), "Customer Request").
			Return(&financial.TransactionResult{
				Success:               true,
				ProviderTransactionID: "re_123",
				Status:                financial.StatusSucceeded,
				Amount:                500,
			}, nil).Once()

		// 3. Create Refund Record
		s.repo.EXPECT().CreateTransaction(mock.Anything, mock.MatchedBy(func(tx *financial.Transaction) bool {
			return tx.Type == financial.TypeRefund && tx.Amount == 500 && tx.ProviderID == "re_123"
		})).Return(nil).Once()

		// 4. Ledger Post (Reverse Logic)
		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "Stripe Clearing").
			Return(&financial.Account{ID: uuid.New()}, nil).Once()
		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "General Revenue").
			Return(&financial.Account{ID: uuid.New()}, nil).Once()

		s.ledger.EXPECT().Post(mock.Anything, s.repo, mock.MatchedBy(func(j *financial.JournalEntry) bool {
			// Check logic: Revenue Debit (Decrease), Cash Credit (Decrease)
			// Assuming Ledger service handles validation, we check structure
			return len(j.Entries) == 2 && j.Entries[0].Direction == financial.Debit // Revenue
		})).Return(nil).Once()

		// 5. Audit & Events
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventTransactionRefunded && e.Outcome == audit.OutcomeSuccess
		})).Return(nil).Once()

		s.bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventTransactionRefunded
		})).Return(nil).Once()

		// Execute
		tx, err := s.service.IssueRefund(s.ctx, txID, s.admin.UserID, 500, "Customer Request")
		require.NoError(t, err)
		assert.Equal(t, int64(500), tx.Amount)
	})

	t.Run("Gateway Failure", func(t *testing.T) {
		s.mockTx()
		s.repo.EXPECT().GetTransaction(mock.Anything, txID).Return(originalTx, nil).Once()

		s.gateway.EXPECT().Refund(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("gateway error")).Once()

		_, err := s.service.IssueRefund(s.ctx, txID, s.admin.UserID, 500, "Fail")
		assert.Error(t, err)
	})

	t.Run("Invalid State", func(t *testing.T) {
		s.mockTx()
		failedTx := *originalTx
		failedTx.Status = financial.StatusFailed

		s.repo.EXPECT().GetTransaction(mock.Anything, txID).Return(&failedTx, nil).Once()

		_, err := s.service.IssueRefund(s.ctx, txID, s.admin.UserID, 500, "Fail")
		assert.ErrorIs(t, err, financial.ErrTransactionNotRefundable)
	})
}

// --- Product Tests ---

func TestAdminService_CreateProduct(t *testing.T) {
	s := setupAdminTest(t)
	interval := financial.IntervalMonth
	params := financial.CreateProductParams{
		Type:        financial.ProductTypeSubscription,
		Name:        "Gold Plan",
		Description: "VIP",
		IsActive:    true,
		Prices: []financial.PriceParams{
			{Amount: 1000, Currency: "USD", Interval: &interval, IsActive: true},
		},
	}

	t.Run("Success", func(t *testing.T) {
		// 1. Gateway Product
		s.gateway.EXPECT().CreateProduct(mock.Anything, params.Name, params.Description).
			Return("prod_123", nil).Once()

		// 2. Gateway Price
		s.gateway.EXPECT().CreatePrice(mock.Anything, mock.MatchedBy(func(req financial.PriceCreateRequest) bool {
			return req.ProductProviderID == "prod_123" && req.Amount == 1000
		})).Return("price_123", nil).Once()

		// 3. DB Create
		s.repo.EXPECT().CreateProduct(mock.Anything, mock.MatchedBy(func(p *financial.Product) bool {
			return p.Name == "Gold Plan" && len(p.Prices) == 1 && p.Prices[0].ProviderPriceID == "price_123"
		})).Return(nil).Once()

		prod, err := s.service.CreateProduct(s.ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "Gold Plan", prod.Name)
	})

	t.Run("DB Failure (Rollback)", func(t *testing.T) {
		// 1. Gateway Success
		s.gateway.EXPECT().CreateProduct(mock.Anything, mock.Anything, mock.Anything).Return("prod_123", nil).Once()
		s.gateway.EXPECT().CreatePrice(mock.Anything, mock.Anything).Return("price_123", nil).Once()

		// 2. DB Failure
		s.repo.EXPECT().CreateProduct(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

		// 3. Rollback Expectations (Archive Price first, then Product)
		s.gateway.EXPECT().ArchivePrice(mock.Anything, "price_123").Return(nil).Once()
		s.gateway.EXPECT().ArchiveProduct(mock.Anything, "prod_123").Return(nil).Once()

		_, err := s.service.CreateProduct(s.ctx, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create product in repository")
	})
}

// --- Grant Disbursement Tests ---

func TestAdminService_DisburseGrant(t *testing.T) {
	s := setupAdminTest(t)
	recipientID := uuid.New()
	refID := uuid.New()

	params := financial.DisburseGrantParams{
		RecipientUserID: recipientID,
		Amount:          5000,
		Currency:        "EUR",
		Description:     "Research Grant",
		ReferenceID:     refID,
	}

	t.Run("Success", func(t *testing.T) {
		s.mockTx()

		// 1. Create Tx
		s.repo.EXPECT().CreateTransaction(mock.Anything, mock.MatchedBy(func(tx *financial.Transaction) bool {
			return tx.Type == financial.TypePayout && tx.Status == financial.StatusSucceeded && tx.Amount == 5000
		})).Return(nil).Once()

		// 2. Ledger Post (Expense Debit, Asset Credit)
		accBank := &financial.Account{ID: uuid.New()}
		accExpense := &financial.Account{ID: uuid.New()}

		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "Bank Account EUR").Return(accBank, nil).Once()
		s.repo.EXPECT().GetLedgerAccountByName(mock.Anything, "Grant Expense").Return(accExpense, nil).Once()

		s.ledger.EXPECT().Post(mock.Anything, s.repo, mock.MatchedBy(func(j *financial.JournalEntry) bool {
			return len(j.Entries) == 2 && j.Entries[0].AccountID == accExpense.ID // Expense Debit
		})).Return(nil).Once()

		// 3. Audit
		s.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventAction("financial.grant.disbursed") && e.Outcome == audit.OutcomeSuccess
		})).Return(nil).Once()

		tx, err := s.service.DisburseGrant(s.ctx, params)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), tx.Amount)
	})
}
