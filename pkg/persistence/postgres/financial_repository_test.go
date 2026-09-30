package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancialRepository_TransactionCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewFinancialRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	tx := &financial.Transaction{
		ID:          uuid.New(),
		UserID:      func() *uuid.UUID { u := uuid.New(); return &u }(),
		Type:        financial.TypePayment,
		Status:      financial.StatusSucceeded,
		Amount:      5000,
		Currency:    "USD",
		Description: "Donation",
		Provider:    "stripe",
		ProviderID:  "ch_123",
		ProcessedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	t.Run("CreateAndGet", func(t *testing.T) {
		err := repo.CreateTransaction(ctx, tx)
		require.NoError(t, err)

		got, err := repo.GetTransaction(ctx, tx.ID)
		require.NoError(t, err)
		assert.Equal(t, tx.Amount, got.Amount)
		assert.Equal(t, tx.UserID, got.UserID)
	})

	t.Run("ListTransactions", func(t *testing.T) {
		// Add another transaction
		tx2 := *tx
		tx2.ID = uuid.New()
		tx2.Type = financial.TypeRefund
		require.NoError(t, repo.CreateTransaction(ctx, &tx2))

		// Filter by Type
		typ := financial.TypeRefund
		list, total, err := repo.ListTransactions(ctx, financial.TransactionFilter{Type: &typ})
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, tx2.ID, list[0].ID)
	})
}

func TestFinancialRepository_Ledger(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewFinancialRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// 1. Setup Accounts (Requires direct DB insert or seeding as no API for this usually)
	accBank := financial.Account{ID: uuid.New(), Name: "Bank", Type: financial.Asset, Currency: "USD", IsActive: true}
	accRev := financial.Account{ID: uuid.New(), Name: "Revenue", Type: financial.Revenue, Currency: "USD", IsActive: true}

	// Helper to insert accounts
	insertAccount := func(a financial.Account) {
		_, err := db.ExecContext(ctx, "INSERT INTO ledger_accounts (id, name, type, currency) VALUES ($1, $2, $3, $4)", a.ID, a.Name, a.Type, a.Currency)
		require.NoError(t, err)
	}
	insertAccount(accBank)
	insertAccount(accRev)

	// 2. Setup Transaction
	tx := &financial.Transaction{
		ID: uuid.New(), Type: financial.TypePayment, Status: financial.StatusSucceeded, Amount: 100, Currency: "USD", Provider: "stripe", ProviderID: "1", ProcessedAt: time.Now(),
	}
	require.NoError(t, repo.CreateTransaction(ctx, tx))

	t.Run("CreateJournalEntry", func(t *testing.T) {
		entry := &financial.JournalEntry{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			Description:   "Donation",
			PostedAt:      time.Now().UTC(),
			Entries: []financial.LedgerEntry{
				{ID: uuid.New(), AccountID: accBank.ID, Direction: financial.Debit, Amount: 100},
				{ID: uuid.New(), AccountID: accRev.ID, Direction: financial.Credit, Amount: 100},
			},
		}

		err := repo.CreateJournalEntry(ctx, entry)
		require.NoError(t, err)

		// Basic verification via SQL
		var count int
		err = db.GetContext(ctx, &count, "SELECT COUNT(*) FROM ledger_entries WHERE journal_entry_id=$1", entry.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestFinancialRepository_Product(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()
	repo := postgres.NewFinancialRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	prod := &financial.Product{
		ID:          uuid.New(),
		Type:        financial.ProductTypeOneTime,
		Name:        "Gold Donation",
		Description: "Support us",
		IsActive:    true,
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		Prices: []financial.Price{
			{ID: uuid.New(), Amount: 5000, Currency: "USD", IsActive: true, ProviderPriceID: "price_1"},
		},
	}
	// Link IDs
	prod.Prices[0].ProductID = prod.ID

	t.Run("CreateProduct", func(t *testing.T) {
		err := repo.CreateProduct(ctx, prod)
		require.NoError(t, err)

		got, err := repo.GetProduct(ctx, prod.ID)
		require.NoError(t, err)
		assert.Equal(t, prod.Name, got.Name)
		assert.Len(t, got.Prices, 1)
		assert.Equal(t, int64(5000), got.Prices[0].Amount)
	})
}
