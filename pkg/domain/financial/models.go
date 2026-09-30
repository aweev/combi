// internal/domain/financial/models.go
package financial

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// LAYER 1: THE LEDGER (The Immutable Accounting Foundation)
// This is the lowest, most fundamental layer. Everything above eventually results
// in an immutable, balanced entry here. This is the source of truth for all financial reporting.
// ============================================================================

type AccountType string

const (
	Asset     AccountType = "ASSET"     // What we own (e.g., Cash in Bank)
	Liability AccountType = "LIABILITY" // What we owe (e.g., Funds to be Paid Out)
	Equity    AccountType = "EQUITY"    // Net worth
	Revenue   AccountType = "REVENUE"   // Income (e.g., Donations Received)
	Expense   AccountType = "EXPENSE"   // Costs (e.g., Payment Provider Fees)
)

// Account represents a ledger account (e.g., "Stripe Clearing Account", "Donations Revenue").
type Account struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `gorm:"uniqueIndex"`
	Type      AccountType
	Currency  string // ISO 4217 code (e.g., "EUR")
	IsActive  bool
	CreatedAt time.Time
}

type EntryDirection string

const (
	Debit  EntryDirection = "DEBIT"  // Increases Assets/Expenses
	Credit EntryDirection = "CREDIT" // Increases Liabilities/Equity/Revenue
)

// LedgerEntry is a single debit or credit line within a JournalEntry. It is never created alone.
type LedgerEntry struct {
	ID             uuid.UUID `gorm:"primaryKey"`
	JournalEntryID uuid.UUID
	AccountID      uuid.UUID
	Direction      EntryDirection
	Amount         int64 // In the smallest currency unit (e.g., cents)
}

// JournalEntry is an immutable, balanced set of debits and credits representing one financial event.
// The sum of all debits MUST equal the sum of all credits.
type JournalEntry struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	TransactionID  uuid.UUID // Links this accounting entry back to the operational event.
	Description    string
	Entries        []LedgerEntry `gorm:"foreignKey:JournalEntryID"`
	PostedAt      time.Time
}

// ============================================================================
// LAYER 2: THE TRANSACTIONAL LAYER (The Operational Events)
// This layer represents the real-world financial events that occur. Each Transaction
// will result in the creation of one or more JournalEntries in the ledger.
// ============================================================================

type TransactionType string

const (
	TypePayment     TransactionType = "PAYMENT"      // A charge to a customer (donation, purchase).
	TypeRefund      TransactionType = "REFUND"       // A full or partial return of funds to a customer.
	TypePayout      TransactionType = "PAYOUT"       // A transfer of funds out to a partner or vendor.
	TypeProviderFee TransactionType = "PROVIDER_FEE" // The fee charged by a payment provider.
	TypeTransfer    TransactionType = "TRANSFER"     // An internal movement of funds (e.g., from clearing to bank).
)

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "PENDING"   // Initiated, awaiting confirmation.
	StatusSucceeded TransactionStatus = "SUCCEEDED" // Funds have been successfully moved.
	StatusFailed    TransactionStatus = "FAILED"    // The transaction failed.
)

// Transaction is the record of a single financial event, like a donation or a refund.
type Transaction struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID  `db:"organization_id"`
	UserID         *uuid.UUID // The user associated with this transaction.
	Type           TransactionType
	Status         TransactionStatus
	Amount               int64 // The gross amount of the transaction.
	Currency             string
	Description          string
	Provider             string     // The generic name of the provider (e.g., "stripe"). Set by the service.
	ProviderID           string     // The unique ID from the external provider (e.g., "ch_123...").
	RelatedTransactionID *uuid.UUID // For refunds, this links to the original payment.
	ProcessedAt          time.Time

	ReferenceType *string    `json:"reference_type,omitempty"` // e.g., "campaign", "event_registration"
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`   // e.g., CampaignID

	Metadata map[string]interface{} `gorm:"type:jsonb"` // For provider-specific metadata
}

// PaymentMethod stores a reusable, generic handle to a payment source for a user.
// It contains no sensitive information, only metadata and the provider's ID.
type PaymentMethod struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID
	Provider       string // The generic name of the provider.
	ProviderID string            // The unique ID from the provider (e.g., "pm_123...").
	Type       string            // e.g., "card", "bank_account"
	Details    map[string]string `gorm:"type:jsonb"` // Safe details like {"brand": "visa", "last4": "4242"}
	IsDefault  bool
	CreatedAt  time.Time
}

// ============================================================================
// LAYER 3: THE PRODUCT & SUBSCRIPTION LAYER (The Commercial Offerings)
// This layer defines what users can pay for. These models drive the creation
// of Transactions.
// ============================================================================

type ProductType string

const (
	ProductTypeOneTime      ProductType = "ONE_TIME"     // e.g., a one-off donation option.
	ProductTypeSubscription ProductType = "SUBSCRIPTION" // e.g., a monthly giving plan.
)

type Interval string

const (
	IntervalDay   Interval = "DAY"
	IntervalWeek  Interval = "WEEK"
	IntervalMonth Interval = "MONTH"
	IntervalYear  Interval = "YEAR"
)

type SubscriptionStatus string

const (
	SubStatusActive     SubscriptionStatus = "ACTIVE"
	SubStatusPastDue    SubscriptionStatus = "PAST_DUE"   // Payment failed, in a retry period.
	SubStatusCancelled  SubscriptionStatus = "CANCELLED"  // Will end at the end of the current period.
	SubStatusEnded      SubscriptionStatus = "ENDED"      // Has fully expired.
	SubStatusIncomplete SubscriptionStatus = "INCOMPLETE" // Initial payment failed.
)

// Product defines a purchasable item, like a "Silver Monthly Donor" plan.
type Product struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	Type           ProductType
	Name           string
	Description    string
	IsActive    bool
	Prices      []Price `gorm:"foreignKey:ProductID"`
	CreatedAt   time.Time
}

// Price defines a specific price point for a Product (e.g., €10/month).
type Price struct {
	ID              uuid.UUID `gorm:"primaryKey"`
	OrganizationID  uuid.UUID `db:"organization_id"`
	ProductID       uuid.UUID
	Amount          int64
	Currency        string
	Interval        *Interval // Null for one-time products.
	ProviderPriceID string    // Provider-specific ID (e.g., Stripe Price ID) for this price point
	IsActive        bool
}

// Subscription represents a user's recurring payment for a Product.
type Subscription struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID `db:"organization_id"`
	UserID             uuid.UUID
	PriceID            uuid.UUID
	Status             SubscriptionStatus
	Provider           string
	ProviderID         string // The unique ID from the external provider (e.g., "sub_123...").
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	CancelledAt        *time.Time
	EndedAt            *time.Time
}

type InvoiceStatus string

const (
	StatusInvoiceDraft         InvoiceStatus = "DRAFT"         // Not yet finalized, can be modified.
	StatusInvoiceOpen          InvoiceStatus = "OPEN"          // Finalized and awaiting payment.
	StatusInvoicePaid          InvoiceStatus = "PAID"          // Payment has been successfully collected.
	StatusInvoiceVoid          InvoiceStatus = "VOID"          // The invoice has been cancelled and is no longer payable.
	StatusInvoiceUncollectible InvoiceStatus = "UNCOLLECTIBLE" // Payment has failed after retries.
)

// Invoice represents a bill for a subscription renewal or one-time purchase.
// It is the precursor to a Transaction for recurring payments.
type Invoice struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID
	SubscriptionID *uuid.UUID // Links to the subscription that generated this invoice.
	TransactionID  *uuid.UUID // Links to the Transaction that paid this invoice.
	Status         InvoiceStatus
	AmountDue      int64
	Currency       string
	DueDate        time.Time // The date payment is expected.
	PeriodStart    time.Time // The start of the service period this invoice covers.
	PeriodEnd      time.Time // The end of the service period this invoice covers.
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Customer struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID `gorm:"uniqueIndex"` // Each user should have only one customer record per provider.
	Provider   string    // The generic name of the provider (e.g., "stripe").
	ProviderID string    // The unique ID from the external provider (e.g., "cus_123...").
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
