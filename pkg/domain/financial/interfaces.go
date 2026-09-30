package financial

import (
	"context"

	"github.com/combi/pkg/domain/privacy"
	"github.com/google/uuid"
)

// Service defines the core contract for user-facing financial operations.
type Service interface {
	privacy.Anonymizer
	CreateOneTimePayment(ctx context.Context, params CreatePaymentParams) (*Transaction, error)
	CreateDonation(ctx context.Context, params CreateDonationParams) (*Transaction, error)
	CreateSubscription(ctx context.Context, params CreateSubscriptionParams) (*Subscription, error)
	CancelSubscription(ctx context.Context, userID, subscriptionID uuid.UUID) (*Subscription, error)

	// Payment Methods
	AddPaymentMethod(ctx context.Context, userID uuid.UUID, providerToken string) (*PaymentMethod, error)
	ListUserPaymentMethods(ctx context.Context, userID uuid.UUID) ([]*PaymentMethod, error)
	SetDefaultPaymentMethod(ctx context.Context, userID, paymentMethodID uuid.UUID) error
	DeletePaymentMethod(ctx context.Context, userID, paymentMethodID uuid.UUID) error

	// Invoicing
	PayInvoice(ctx context.Context, invoiceID uuid.UUID) error
	ListUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*SubscriptionDetails, error)
}

// AdminService defines administrative and accounting functions.
type AdminService interface {
	// Product Catalog
	CreateProduct(ctx context.Context, params CreateProductParams) (*Product, error)
	UpdateProduct(ctx context.Context, productID uuid.UUID, params UpdateProductParams) (*Product, error)
	ListProducts(ctx context.Context) ([]*Product, error)

	// Transactions
	GetTransaction(ctx context.Context, transactionID uuid.UUID) (*Transaction, error) // Critical for RefundWorker
	ListTransactions(ctx context.Context, filter TransactionFilter) ([]*Transaction, int, error)

	// Operations
	IssueRefund(ctx context.Context, transactionID, actorID uuid.UUID, amount int64, reason string) (*Transaction, error)
	DisburseGrant(ctx context.Context, params DisburseGrantParams) (*Transaction, error)
}

// Ledger handles double-entry bookkeeping.
type Ledger interface {
	Post(ctx context.Context, txRepo Repository, journal *JournalEntry) error
}

// --- DTOs for Service Methods ---

type CreateDonationParams struct {
	UserID         uuid.UUID
	Amount         int64
	Currency       string
	Description    string
	PaymentToken   string
	IdempotencyKey string
}

type CreatePaymentParams struct {
	UserID          uuid.UUID              `validate:"required"`
	Amount          int64                  `validate:"required,gt=0"`
	Currency        string                 `validate:"required,iso4217"`
	Description     string                 `validate:"required"`
	PaymentMethodID *uuid.UUID             // Use an existing saved method
	ProviderToken   *string                // Or provide a new one-time token
	IdempotencyKey  string                 `validate:"required"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type CreateSubscriptionParams struct {
	UserID          uuid.UUID `validate:"required"`
	PriceID         uuid.UUID `validate:"required"`
	PaymentMethodID *uuid.UUID
	ProviderToken   *string
}

type CreateProductParams struct {
	Type        ProductType `validate:"required"`
	Name        string      `validate:"required"`
	Description string
	IsActive    bool
	Prices      []PriceParams `validate:"required,min=1,dive"`
}

type PriceParams struct {
	Amount   int64     `validate:"required,gt=0"`
	Currency string    `validate:"required,iso4217"`
	Interval *Interval // Required for subscriptions
	IsActive bool
}

type UpdateProductParams struct {
	Name        *string
	Description *string
	IsActive    *bool
}

type TransactionFilter struct {
	UserID *uuid.UUID
	Type   *TransactionType
	Status *TransactionStatus
	Limit  int
	Offset int
}

// --- System Components & Infrastructure (Outbound Ports) ---

type DisburseGrantParams struct {
	RecipientUserID uuid.UUID `validate:"required"`
	Amount          int64     `validate:"required,gt=0"`
	Currency        string    `validate:"required,iso4217"`
	Description     string    `validate:"required"`
	ReferenceID     uuid.UUID `validate:"required"` // Links to the grant application ID
	IdempotencyKey  string    `validate:"required"`
}
