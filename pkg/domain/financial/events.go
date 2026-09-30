package financial

import (
	"time"

	"github.com/google/uuid"
)

// PaymentSucceededPayload is the master record for money movement.
type PaymentSucceededPayload struct {
	TransactionID         uuid.UUID         `json:"transaction_id"`
	OrganizationID        uuid.UUID         `json:"organization_id"`
	UserID                uuid.UUID         `json:"user_id"` // The Payer
	Amount                int64             `json:"amount"`  // In smallest unit (cents)
	Currency              string            `json:"currency"`
	Provider              string            `json:"provider"`
	Type                  string            `json:"type"` // "DONATION", "TICKET", "SHOP"
	ProviderTransactionID string            `json:"provider_transaction_id"`
	Metadata              map[string]string `json:"metadata,omitempty"` // Contains CampaignID, FundraiserID
	OccurredAt            time.Time         `json:"occurred_at"`
}

// SubscriptionRenewedPayload is for recurring giving analysis.
type SubscriptionRenewedPayload struct {
	SubscriptionID         string    `json:"subscription_id"`          // Our internal ID (if known) or...
	ProviderSubscriptionID string    `json:"provider_subscription_id"` // ...the provider ID
	ProviderInvoiceID      string    `json:"provider_invoice_id"`
	UserID                 uuid.UUID `json:"user_id,omitempty"` // Might be nil if coming from webhook
	AmountPaid             int64     `json:"amount_paid"`
	Currency               string    `json:"currency"`
	PeriodStart            time.Time `json:"period_start"`
	PeriodEnd              time.Time `json:"period_end"`
}

type SubscriptionCancelledPayload struct {
	ProviderSubscriptionID string    `json:"provider_subscription_id"`
	CancelledAt            time.Time `json:"cancelled_at"`
}
