// internal/domain/financial/gateway.go
package financial

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- DTOs for the Gateway Contract ---

// ChargeRequest now includes the optional ProviderCustomerID
type ChargeRequest struct {
	UserID             uuid.UUID
	Amount             int64
	Currency           string
	Description        string
	IdempotencyKey     string
	PaymentToken       string
	ProviderCustomerID *string // Optional: The customer ID from the provider (e.g., Stripe's "cus_...")
}

// RecurringPaymentRequest now requires the ProviderCustomerID
type RecurringPaymentRequest struct {
	UserID             uuid.UUID
	ProviderPriceID    string
	PaymentToken       string
	ProviderCustomerID string // Required: The customer ID from the provider.
}

// TransactionResult is the generic, standardized outcome of a charge or refund attempt.
// Renamed from ChargeResult for better clarity.
type TransactionResult struct {
	Success               bool
	ProviderTransactionID string            // The ID for the charge or refund transaction.
	Status                TransactionStatus // SUCCEEDED, PENDING, FAILED
	Amount                int64             // The amount that was processed.
	Currency              string            // The currency of the transaction.
	Provider              string            // The name of the provider, e.g. "stripe".
	FailureCode           string            // A standardized code, e.g., "card_declined", "insufficient_funds"
	FailureMessage        string
}

// StoredPaymentMethod contains a generic, safe handle to a stored payment method.
type StoredPaymentMethod struct {
	ProviderMethodID string // The unique ID from the provider.
	Type             string // "card"
	Brand            string // "visa"
	Last4            string // "4242"
	ExpiryMonth      int
	ExpiryYear       int
}

// RecurringPaymentResult is the generic outcome of a subscription creation.
type RecurringPaymentResult struct {
	Success                bool
	ProviderSubscriptionID string
	Status                 SubscriptionStatus
	InitialPaymentResult   *TransactionResult // Result of the first payment, if applicable.
}

// PriceCreateRequest contains the data needed to create a price object with a provider.
type PriceCreateRequest struct {
	ProductProviderID string // The provider's ID for the parent product
	Amount            int64
	Currency          string
	Interval          *Interval
}

// SubscriptionDetails contains detailed information about a subscription from the provider.
type SubscriptionDetails struct {
	ProviderSubscriptionID string
	Status                 SubscriptionStatus
	DefaultPaymentMethodID string // The provider's ID for the payment method
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       time.Time
	CancelAtPeriodEnd      bool
	*Subscription
	*Price
}

// PaymentGatewayFactory creates instances of PaymentGateway.
type PaymentGatewayFactory interface {
	GetPaymentGateway(ctx context.Context) (PaymentGateway, error)
}

// PaymentGateway abstracts the external provider (Stripe, PayPal).
type PaymentGateway interface {
	ProviderName() string
	CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (providerCustomerID string, err error)
	Charge(ctx context.Context, req ChargeRequest) (*TransactionResult, error)
	Refund(ctx context.Context, providerTransactionID string, amount int64, reason string) (*TransactionResult, error)
	StorePaymentMethod(ctx context.Context, providerCustomerID string, paymentToken string) (*StoredPaymentMethod, error)
	DetachPaymentMethod(ctx context.Context, providerMethodID string) error
	StartRecurringPayment(ctx context.Context, req RecurringPaymentRequest) (*RecurringPaymentResult, error)
	CancelRecurringPayment(ctx context.Context, providerSubscriptionID string, atPeriodEnd bool) error
	GetSubscriptionDetails(ctx context.Context, providerSubscriptionID string) (*SubscriptionDetails, error)
	CreateProduct(ctx context.Context, name, description string) (providerProductID string, err error)
	CreatePrice(ctx context.Context, req PriceCreateRequest) (providerPriceID string, err error)
	ArchiveProduct(ctx context.Context, providerProductID string) error
	ArchivePrice(ctx context.Context, providerMethodID string) error
}
