package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/client"
	"github.com/stripe/stripe-go/v76/webhook"
)

type StripeAdapter struct {
	client *client.API
	logger log.Logger
}

// NewStripeAdapter initializes the client.
func NewStripeAdapter(credentials map[string]string, logger log.Logger) (*StripeAdapter, error) {
	key, ok := credentials["secret_key"]
	if !ok || key == "" {
		return nil, fmt.Errorf("missing 'secret_key' for stripe adapter")
	}

	sc := &client.API{}
	sc.Init(key, nil)

	return &StripeAdapter{
		client: sc,
		logger: logger.With(log.String("adapter", "stripe")),
	}, nil
}

func (a *StripeAdapter) ProviderName() string {
	return "stripe"
}

// --- Customer Management ---

func (a *StripeAdapter) CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	}
	c, err := a.client.Customers.New(params)
	if err != nil {
		return "", a.mapError(err)
	}
	return c.ID, nil
}

// --- Transaction Management ---

func (a *StripeAdapter) Charge(ctx context.Context, req financial.ChargeRequest) (*financial.TransactionResult, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(req.Amount),
		Currency: stripe.String(strings.ToLower(req.Currency)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Description: stripe.String(req.Description),
		Confirm:     stripe.Bool(true),
		ReturnURL:   stripe.String("https://ngo-platform.org/payment/return"), // In a real app, this comes from config
	}

	if req.ProviderCustomerID != nil {
		params.Customer = stripe.String(*req.ProviderCustomerID)
	}
	if req.PaymentToken != "" {
		// Assuming PaymentToken is a PaymentMethod ID (pm_...)
		params.PaymentMethod = stripe.String(req.PaymentToken)
	}

	// Idempotency
	if req.IdempotencyKey != "" {
		params.SetIdempotencyKey(req.IdempotencyKey)
	}

	pi, err := a.client.PaymentIntents.New(params)
	if err != nil {
		return nil, a.mapError(err)
	}

	return a.mapPaymentIntentToResult(pi), nil
}

func (a *StripeAdapter) Refund(ctx context.Context, providerTransactionID string, amount int64, reason string) (*financial.TransactionResult, error) {
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(providerTransactionID),
		Amount:        stripe.Int64(amount),
	}
	if reason != "" {
		params.Metadata = map[string]string{"reason": reason}
	}

	re, err := a.client.Refunds.New(params)
	if err != nil {
		return nil, a.mapError(err)
	}

	status := financial.StatusPending
	switch re.Status {
	case stripe.RefundStatusSucceeded:
		status = financial.StatusSucceeded
	case stripe.RefundStatusFailed:
		status = financial.StatusFailed
	}

	return &financial.TransactionResult{
		Success:               re.Status == stripe.RefundStatusSucceeded,
		ProviderTransactionID: re.ID,
		Status:                status,
		Amount:                re.Amount,
		Currency:              string(re.Currency),
		Provider:              "stripe",
	}, nil
}

// --- Subscription Management ---

func (a *StripeAdapter) StartRecurringPayment(ctx context.Context, req financial.RecurringPaymentRequest) (*financial.RecurringPaymentResult, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(req.ProviderCustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(req.ProviderPriceID)},
		},
		PaymentBehavior:      stripe.String("default_incomplete"),
		DefaultPaymentMethod: stripe.String(req.PaymentToken),
		Expand:               []*string{stripe.String("latest_invoice.payment_intent")},
	}

	sub, err := a.client.Subscriptions.New(params)
	if err != nil {
		return nil, a.mapError(err)
	}

	res := &financial.RecurringPaymentResult{
		Success:                sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing,
		ProviderSubscriptionID: sub.ID,
		Status:                 a.mapSubscriptionStatus(sub.Status),
	}

	// Extract initial payment result
	if sub.LatestInvoice != nil && sub.LatestInvoice.PaymentIntent != nil {
		res.InitialPaymentResult = a.mapPaymentIntentToResult(sub.LatestInvoice.PaymentIntent)
	}

	return res, nil
}

func (a *StripeAdapter) CancelRecurringPayment(ctx context.Context, providerSubscriptionID string, atPeriodEnd bool) error {
	if atPeriodEnd {
		params := &stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(true)}
		_, err := a.client.Subscriptions.Update(providerSubscriptionID, params)
		return a.mapError(err)
	}
	_, err := a.client.Subscriptions.Cancel(providerSubscriptionID, nil)
	return a.mapError(err)
}

func (a *StripeAdapter) GetSubscriptionDetails(ctx context.Context, providerSubscriptionID string) (*financial.SubscriptionDetails, error) {
	sub, err := a.client.Subscriptions.Get(providerSubscriptionID, nil)
	if err != nil {
		return nil, a.mapError(err)
	}

	details := &financial.SubscriptionDetails{
		ProviderSubscriptionID: sub.ID,
		Status:                 a.mapSubscriptionStatus(sub.Status),
		CurrentPeriodStart:     time.Unix(sub.CurrentPeriodStart, 0),
		CurrentPeriodEnd:       time.Unix(sub.CurrentPeriodEnd, 0),
		CancelAtPeriodEnd:      sub.CancelAtPeriodEnd,
	}
	return details, nil
}

// --- Product Management ---

func (a *StripeAdapter) CreateProduct(ctx context.Context, name, description string) (string, error) {
	params := &stripe.ProductParams{
		Name:        stripe.String(name),
		Description: stripe.String(description),
	}
	prod, err := a.client.Products.New(params)
	if err != nil {
		return "", a.mapError(err)
	}
	return prod.ID, nil
}

func (a *StripeAdapter) CreatePrice(ctx context.Context, req financial.PriceCreateRequest) (string, error) {
	params := &stripe.PriceParams{
		Product:    stripe.String(req.ProductProviderID),
		UnitAmount: stripe.Int64(req.Amount),
		Currency:   stripe.String(strings.ToLower(req.Currency)),
	}
	if req.Interval != nil {
		params.Recurring = &stripe.PriceRecurringParams{
			Interval: stripe.String(strings.ToLower(string(*req.Interval))),
		}
	}
	price, err := a.client.Prices.New(params)
	if err != nil {
		return "", a.mapError(err)
	}
	return price.ID, nil
}

func (a *StripeAdapter) ArchiveProduct(ctx context.Context, providerProductID string) error {
	params := &stripe.ProductParams{Active: stripe.Bool(false)}
	_, err := a.client.Products.Update(providerProductID, params)
	return a.mapError(err)
}

func (a *StripeAdapter) ArchivePrice(ctx context.Context, providerPriceID string) error {
	params := &stripe.PriceParams{Active: stripe.Bool(false)}
	_, err := a.client.Prices.Update(providerPriceID, params)
	return a.mapError(err)
}

// --- Payment Methods ---

func (a *StripeAdapter) StorePaymentMethod(ctx context.Context, providerCustomerID string, paymentToken string) (*financial.StoredPaymentMethod, error) {
	// Attach the payment method to the customer
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(providerCustomerID),
	}
	pm, err := a.client.PaymentMethods.Attach(paymentToken, params)
	if err != nil {
		return nil, a.mapError(err)
	}

	return &financial.StoredPaymentMethod{
		ProviderMethodID: pm.ID,
		Type:             string(pm.Type),
		Brand:            string(pm.Card.Brand),
		Last4:            pm.Card.Last4,
		ExpiryMonth:      int(pm.Card.ExpMonth),
		ExpiryYear:       int(pm.Card.ExpYear),
	}, nil
}

func (a *StripeAdapter) DetachPaymentMethod(ctx context.Context, providerMethodID string) error {
	_, err := a.client.PaymentMethods.Detach(providerMethodID, nil)
	return a.mapError(err)
}

// --- WebhookParser Implementation ---

func (a *StripeAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	var cfgMap map[string]any
	if err := json.Unmarshal(config.Configuration, &cfgMap); err != nil {
		return "", "", nil, integration.ErrInvalidConfiguration
	}
	signingSecret, _ := cfgMap["webhook_signing_secret"].(string)

	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", nil, err
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), signingSecret)
	if err != nil {
		return "", "", nil, integration.ErrInvalidSignature
	}

	return string(event.Type), event.ID, payload, nil
}

// --- Helpers ---

func (a *StripeAdapter) mapPaymentIntentToResult(pi *stripe.PaymentIntent) *financial.TransactionResult {
	res := &financial.TransactionResult{
		ProviderTransactionID: pi.ID,
		Amount:                pi.Amount,
		Currency:              string(pi.Currency),
		Provider:              "stripe",
	}

	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		res.Status = financial.StatusSucceeded
		res.Success = true
	case stripe.PaymentIntentStatusRequiresAction, stripe.PaymentIntentStatusRequiresConfirmation:
		res.Status = financial.StatusPending
		res.Success = false
	default:
		res.Status = financial.StatusFailed
		res.Success = false
		if pi.LastPaymentError != nil {
			res.FailureMessage = pi.LastPaymentError.Msg
			res.FailureCode = string(pi.LastPaymentError.Code)
		}
	}
	return res
}

func (a *StripeAdapter) mapSubscriptionStatus(status stripe.SubscriptionStatus) financial.SubscriptionStatus {
	switch status {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
		return financial.SubStatusActive
	case stripe.SubscriptionStatusPastDue:
		return financial.SubStatusPastDue
	case stripe.SubscriptionStatusCanceled:
		return financial.SubStatusCancelled
	case stripe.SubscriptionStatusIncomplete:
		return financial.SubStatusIncomplete
	default:
		return financial.SubStatusEnded
	}
}

func (a *StripeAdapter) mapError(err error) error {
	if stripeErr, ok := err.(*stripe.Error); ok {
		switch stripeErr.Code {
		case stripe.ErrorCodeCardDeclined:
			return financial.ErrPaymentDeclined
		case stripe.ErrorCodeExpiredCard:
			return financial.ErrCardExpired
		case stripe.ErrorCodeBalanceInsufficient:
			return financial.ErrInsufficientFunds
		case stripe.ErrorCodeIdempotencyKeyInUse:
			return financial.ErrIdempotencyConflict
		}
	}
	return financial.ErrPaymentProviderError
}

func (a *StripeAdapter) TestConnection(ctx context.Context) error {
	// List 1 customer to verify API key works
	params := &stripe.CustomerListParams{}
	params.Limit = stripe.Int64(1)
	iter := a.client.Customers.List(params)
	if iter.Next() {
		return nil
	}
	if err := iter.Err(); err != nil {
		return a.mapError(err)
	}
	return nil
}

func (a *StripeAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	switch wh.EventType {
	case "invoice.paid":
		var payload struct {
			Object struct {
				ID           string `json:"id"`
				Subscription string `json:"subscription"`
				AmountPaid   int64  `json:"amount_paid"` // Stripe sends integer cents
				Currency     string `json:"currency"`
				PeriodStart  int64  `json:"period_start"`
				PeriodEnd    int64  `json:"period_end"`
			} `json:"object"`
		}
		if err := json.Unmarshal(wh.Payload, &payload); err != nil {
			return nil, integration.NewNonRetryableError(err)
		}

		// Logic Fix: Map to financial.SubscriptionRenewedPayload
		eventPayload := financial.SubscriptionRenewedPayload{
			ProviderSubscriptionID: payload.Object.Subscription,
			ProviderInvoiceID:      payload.Object.ID,
			AmountPaid:             payload.Object.AmountPaid,
			Currency:               payload.Object.Currency,
			PeriodStart:            time.Unix(payload.Object.PeriodStart, 0).UTC(),
			PeriodEnd:              time.Unix(payload.Object.PeriodEnd, 0).UTC(),
		}

		evt, _ := events.NewEvent(core.EventSubscriptionRenewed, system.SystemActorID, "system", eventPayload)
		return &evt, nil

	case "customer.subscription.deleted":
		var payload struct {
			Object struct {
				ID         string `json:"id"`
				CanceledAt int64  `json:"canceled_at"`
			} `json:"object"`
		}
		if err := json.Unmarshal(wh.Payload, &payload); err != nil {
			return nil, integration.NewNonRetryableError(err)
		}

		eventPayload := financial.SubscriptionCancelledPayload{
			ProviderSubscriptionID: payload.Object.ID,
			CancelledAt:            time.Unix(payload.Object.CanceledAt, 0).UTC(),
		}
		evt, _ := events.NewEvent(core.EventSubscriptionCancelled, system.SystemActorID, "system", eventPayload)
		return &evt, nil

	default:
		return nil, nil
	}
}
