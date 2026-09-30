package payments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/combi/pkg/domain/financial"
	fin_mocks "github.com/combi/pkg/domain/financial/mocks"
	"github.com/combi/pkg/log"
	fin_service "github.com/combi/payments"
)

func TestLedgerService_Post(t *testing.T) {
	// Setup
	mockRepo := fin_mocks.NewMockRepository(t)
	ledger := fin_service.NewLedgerService(&log.NoOpLogger{})
	ctx := context.Background()

	// Helper to create IDs
	txID := uuid.New()
	accCash := uuid.New()
	accRevenue := uuid.New()

	t.Run("Success: Balanced Entry", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txID,
			Description:   "Payment Received",
			PostedAt:      time.Now(),
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accCash, Direction: financial.Debit, Amount: 5000},     // +$50.00 to Bank
				{ID: uuid.New(), AccountID: accRevenue, Direction: financial.Credit, Amount: 5000}, // +$50.00 to Revenue
			},
		}

		// Expect persistence call
		mockRepo.EXPECT().CreateJournalEntry(ctx, entry).Return(nil).Once()

		err := ledger.Post(ctx, mockRepo, entry)
		assert.NoError(t, err)
	})

	t.Run("Failure: Unbalanced Entry", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txID,
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accCash, Direction: financial.Debit, Amount: 5000},
				{ID: uuid.New(), AccountID: accRevenue, Direction: financial.Credit, Amount: 4999}, // Missing 1 cent
			},
		}

		// Repo should NOT be called
		err := ledger.Post(ctx, mockRepo, entry)
		assert.ErrorIs(t, err, financial.ErrInvalidInput)
		assert.Contains(t, err.Error(), "unbalanced")
	})

	t.Run("Failure: Negative Amount", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txID,
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accCash, Direction: financial.Debit, Amount: -100}, // Invalid
				{ID: uuid.New(), AccountID: accRevenue, Direction: financial.Credit, Amount: -100},
			},
		}

		err := ledger.Post(ctx, mockRepo, entry)
		assert.ErrorIs(t, err, financial.ErrInvalidInput)
		assert.Contains(t, err.Error(), "must be positive")
	})

	t.Run("Failure: Single Leg Entry", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txID,
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accCash, Direction: financial.Debit, Amount: 5000},
			},
		}

		err := ledger.Post(ctx, mockRepo, entry)
		assert.ErrorIs(t, err, financial.ErrInvalidInput)
		assert.Contains(t, err.Error(), "at least two legs")
	})

	t.Run("Failure: Repository Error", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txID,
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accCash, Direction: financial.Debit, Amount: 100},
				{ID: uuid.New(), AccountID: accRevenue, Direction: financial.Credit, Amount: 100},
			},
		}

		// Simulate DB error
		mockRepo.EXPECT().CreateJournalEntry(ctx, entry).Return(errors.New("db connection lost")).Once()

		err := ledger.Post(ctx, mockRepo, entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save ledger entry")
	})
}
