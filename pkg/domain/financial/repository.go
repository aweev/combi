package financial

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository aggregates all data access for this domain.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	// Transaction
	CreateTransaction(ctx context.Context, tx *Transaction) error
	GetTransaction(ctx context.Context, id uuid.UUID) (*Transaction, error)
	UpdateTransaction(ctx context.Context, tx *Transaction) error
	ListTransactions(ctx context.Context, filter TransactionFilter) ([]*Transaction, int, error)

	// Subscription
	CreateSubscription(ctx context.Context, sub *Subscription) error
	GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error)
	UpdateSubscription(ctx context.Context, sub *Subscription) error
	ListSubscriptionsForUser(ctx context.Context, userID uuid.UUID) ([]*Subscription, error)
	ScanDueSubscriptions(ctx context.Context, cutoff time.Time, limit int) ([]*Subscription, error)

	// Products (New Section - Resolves 4 errors)
	CreateProduct(ctx context.Context, p *Product) error
	GetProduct(ctx context.Context, id uuid.UUID) (*Product, error)
	UpdateProduct(ctx context.Context, p *Product) error
	ListProducts(ctx context.Context) ([]*Product, error)

	// Pricing
	GetPrice(ctx context.Context, id uuid.UUID) (*Price, error)
	GetPricesByIDs(ctx context.Context, ids []uuid.UUID) ([]*Price, error)

	// Payment Methods
	CreatePaymentMethod(ctx context.Context, pm *PaymentMethod) error
	GetPaymentMethod(ctx context.Context, id uuid.UUID) (*PaymentMethod, error)
	ListPaymentMethodsForUser(ctx context.Context, userID uuid.UUID) ([]*PaymentMethod, error)
	GetDefaultPaymentMethodForUser(ctx context.Context, userID uuid.UUID) (*PaymentMethod, error)
	CountPaymentMethodsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	UpdatePaymentMethod(ctx context.Context, pm *PaymentMethod) error
	ClearDefaultPaymentMethod(ctx context.Context, userID uuid.UUID) error
	DeletePaymentMethod(ctx context.Context, id uuid.UUID) error

	// Customer
	CreateCustomer(ctx context.Context, c *Customer) error
	GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (*Customer, error)

	// Ledger
	CreateJournalEntry(ctx context.Context, entry *JournalEntry) error
	GetLedgerAccountByName(ctx context.Context, name string) (*Account, error)

	// Invoices
	CreateInvoice(ctx context.Context, inv *Invoice) error // Explicitly needed for worker
	GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error)
	UpdateInvoice(ctx context.Context, inv *Invoice) error
	ListInvoicesForUser(ctx context.Context, userID uuid.UUID) ([]*Invoice, error) // For worker checks

	// Reporting (Aggregates)
	GetAnnualFinancialSummary(ctx context.Context, year int) (*AnnualSummaryDTO, error)

	// Privacy
	AnonymizeUserRecords(ctx context.Context, userID uuid.UUID) error
}

// AnnualSummaryDTO represents the aggregated financial data for a specific year.
// This is used by the Reporting Service.
type AnnualSummaryDTO struct {
	Year             int            `json:"year"`
	TotalRevenue     int64          `json:"total_revenue"` // Sum of succeeded payments
	TotalRefunds     int64          `json:"total_refunds"` // Sum of refunds
	NetRevenue       int64          `json:"net_revenue"`   // Revenue - Refunds
	TransactionCount int            `json:"transaction_count"`
	Currency         string         `json:"currency"`
	MonthlyBreakdown []MonthSummary `json:"monthly_breakdown"`
}

type MonthSummary struct {
	Month  int   `json:"month"` // 1-12
	Amount int64 `json:"amount"`
}
