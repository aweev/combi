package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ financial.PaymentGateway = (*PayPalAdapter)(nil)
var _ integration.WebhookParser = (*PayPalAdapter)(nil)

const (
	payPalSandboxAPI = "https://api-m.sandbox.paypal.com"
	payPalLiveAPI    = "https://api-m.paypal.com"
)

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type payPalErrorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

type PayPalAdapter struct {
	client       *http.Client
	logger       log.Logger
	clientID     string
	clientSecret string
	apiBaseURL   string
	tokenMutex   sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
}

func NewPayPalAdapter(credentials map[string]string, logger log.Logger) (*PayPalAdapter, error) {
	clientID, ok := credentials["client_id"]
	if !ok || clientID == "" {
		return nil, fmt.Errorf("%w: PayPal adapter requires 'client_id'", integration.ErrMissingCredentials)
	}
	clientSecret, ok := credentials["client_secret"]
	if !ok || clientSecret == "" {
		return nil, fmt.Errorf("%w: PayPal adapter requires 'client_secret'", integration.ErrMissingCredentials)
	}
	mode, ok := credentials["mode"]
	if !ok {
		mode = "sandbox" // Default to sandbox for safety
	}

	adapter := &PayPalAdapter{
		client:       &http.Client{Timeout: 20 * time.Second},
		logger:       logger.With(log.String("adapter", "paypal")),
		clientID:     clientID,
		clientSecret: clientSecret,
		apiBaseURL:   map[string]string{"sandbox": payPalSandboxAPI, "live": payPalLiveAPI}[mode],
	}

	if err := adapter.refreshToken(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to get initial PayPal auth token: %w", err)
	}
	return adapter, nil
}

func (a *PayPalAdapter) ProviderName() string {
	return "paypal"
}

func (a *PayPalAdapter) CreateCustomer(ctx context.Context, userID uuid.UUID, email, name string) (string, error) {
	a.logger.Warn("CreateCustomer called on PayPal adapter; this is a no-op as PayPal does not use a persistent customer object in the same way as Stripe.", log.String("user_id", userID.String()))
	// Return the UserID as a stand-in, as it won't be used by other PayPal methods.
	return userID.String(), nil
}

func (a *PayPalAdapter) Charge(ctx context.Context, req financial.ChargeRequest) (*financial.TransactionResult, error) {
	// For PayPal, the PaymentToken is the `orderID` from the client-side SDK.
	captureURL := fmt.Sprintf("%s/v2/checkout/orders/%s/capture", a.apiBaseURL, req.PaymentToken)
	respBody, err := a.doRequest(ctx, http.MethodPost, captureURL, nil)
	if err != nil {
		return nil, err
	}

	var captureResp struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					ID     string `json:"id"`
					Amount struct {
						CurrencyCode string `json:"currency_code"`
						Value        string `json:"value"`
					} `json:"amount"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}
	if err := json.Unmarshal(respBody, &captureResp); err != nil {
		return nil, fmt.Errorf("failed to decode paypal capture response: %w", err)
	}

	if captureResp.Status != "COMPLETED" {
		return &financial.TransactionResult{
			Success:        false,
			Provider:       a.ProviderName(),
			Status:         financial.StatusFailed,
			FailureMessage: "PayPal order not completed.",
		}, nil
	}

	// A successful capture should always have a capture ID
	if len(captureResp.PurchaseUnits) == 0 || len(captureResp.PurchaseUnits[0].Payments.Captures) == 0 {
		return nil, fmt.Errorf("paypal capture response missing capture details for order %s", captureResp.ID)
	}

	capture := captureResp.PurchaseUnits[0].Payments.Captures[0]

	return &financial.TransactionResult{
		Success:               true,
		ProviderTransactionID: capture.ID,
		Status:                financial.StatusSucceeded,
		Provider:              a.ProviderName(),
		Amount:                req.Amount, // PayPal doesn't return the amount in a simple way, so we trust the request.
		Currency:              req.Currency,
	}, nil
}

func (a *PayPalAdapter) Refund(ctx context.Context, providerTransactionID string, amount int64, reason string) (*financial.TransactionResult, error) {
	refundURL := fmt.Sprintf("%s/v2/payments/captures/%s/refund", a.apiBaseURL, providerTransactionID)

	// TODO: Fetch original transaction to determine currency dynamically.
	// Currently defaulting to EUR as per organization base currency.
	currency := "EUR"

	payload := map[string]interface{}{
		"amount": map[string]string{
			"value":         fmt.Sprintf("%.2f", float64(amount)/100.0),
			"currency_code": currency,
		},
		"note_to_payer": reason,
	}

	respBody, err := a.doRequest(ctx, http.MethodPost, refundURL, payload)
	if err != nil {
		return nil, err
	}

	var refundResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &refundResp); err != nil {
		return nil, fmt.Errorf("failed to decode paypal refund response: %w", err)
	}

	return &financial.TransactionResult{
		Success:               strings.ToUpper(refundResp.Status) == "COMPLETED",
		ProviderTransactionID: refundResp.ID,
		Status:                financial.StatusSucceeded,
		Provider:              a.ProviderName(),
		Amount:                amount,
		Currency:              currency,
	}, nil
}

func (a *PayPalAdapter) CreateProduct(ctx context.Context, name, description string) (string, error) {
	productURL := fmt.Sprintf("%s/v1/catalogs/products", a.apiBaseURL)
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"type":        "SERVICE",
		"category":    "SOFTWARE",
	}
	respBody, err := a.doRequest(ctx, http.MethodPost, productURL, payload)
	if err != nil {
		return "", err
	}
	var productResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &productResp); err != nil {
		return "", fmt.Errorf("failed to decode paypal create product response: %w", err)
	}
	return productResp.ID, nil
}

func (a *PayPalAdapter) CreatePrice(ctx context.Context, req financial.PriceCreateRequest) (string, error) {
	planURL := fmt.Sprintf("%s/v1/billing/plans", a.apiBaseURL)
	if req.Interval == nil {
		return "", fmt.Errorf("paypal adapter requires an interval to create a billing plan (price)")
	}
	payload := map[string]interface{}{
		"product_id": req.ProductProviderID,
		"name":       fmt.Sprintf("Plan for %s", req.ProductProviderID),
		"status":     "ACTIVE",
		"billing_cycles": []map[string]interface{}{{
			"frequency": map[string]interface{}{
				"interval_unit":  strings.ToUpper(string(*req.Interval)),
				"interval_count": 1,
			},
			"tenure_type":  "REGULAR",
			"sequence":     1,
			"total_cycles": 0, // 0 for infinite recurrence
			"pricing_scheme": map[string]interface{}{
				"fixed_price": map[string]string{
					"value":         fmt.Sprintf("%.2f", float64(req.Amount)/100.0),
					"currency_code": req.Currency,
				},
			},
		}},
		"payment_preferences": map[string]interface{}{
			"auto_bill_outstanding": true,
		},
	}
	respBody, err := a.doRequest(ctx, http.MethodPost, planURL, payload)
	if err != nil {
		return "", err
	}
	var planResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &planResp); err != nil {
		return "", fmt.Errorf("failed to decode paypal create plan response: %w", err)
	}
	return planResp.ID, nil
}

func (a *PayPalAdapter) StartRecurringPayment(ctx context.Context, req financial.RecurringPaymentRequest) (*financial.RecurringPaymentResult, error) {
	subURL := fmt.Sprintf("%s/v1/billing/subscriptions", a.apiBaseURL)
	payload := map[string]interface{}{
		"plan_id": req.ProviderPriceID,
	}
	respBody, err := a.doRequest(ctx, http.MethodPost, subURL, payload)
	if err != nil {
		return nil, err
	}
	var subResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &subResp); err != nil {
		return nil, fmt.Errorf("failed to decode paypal create subscription response: %w", err)
	}

	return &financial.RecurringPaymentResult{
		ProviderSubscriptionID: subResp.ID,
		Status:                 a.mapSubscriptionStatus(subResp.Status),
	}, nil
}

func (a *PayPalAdapter) CancelRecurringPayment(ctx context.Context, providerSubscriptionID string, atPeriodEnd bool) error {
	if atPeriodEnd {
		a.logger.Warn("PayPal API does not support 'cancel at period end'. Subscription will be cancelled immediately.",
			log.String("provider_subscription_id", providerSubscriptionID))
	}

	cancelURL := fmt.Sprintf("%s/v1/billing/subscriptions/%s/cancel", a.apiBaseURL, providerSubscriptionID)
	payload := map[string]string{"reason": "Cancellation requested by service"}

	_, err := a.doRequest(ctx, http.MethodPost, cancelURL, payload)
	if err != nil {
		return fmt.Errorf("failed to cancel paypal subscription: %w", err)
	}
	return nil
}

func (a *PayPalAdapter) GetSubscriptionDetails(ctx context.Context, providerSubscriptionID string) (*financial.SubscriptionDetails, error) {
	subURL := fmt.Sprintf("%s/v1/billing/subscriptions/%s", a.apiBaseURL, providerSubscriptionID)
	respBody, err := a.doRequest(ctx, http.MethodGet, subURL, nil)
	if err != nil {
		return nil, err
	}
	var subResp struct {
		ID          string    `json:"id"`
		Status      string    `json:"status"`
		StatusTime  time.Time `json:"status_update_time"`
		BillingInfo struct {
			LastPayment struct {
				Time time.Time `json:"time"`
			} `json:"last_payment"`
			NextBillingTime time.Time `json:"next_billing_time"`
		} `json:"billing_info"`
	}
	if err := json.Unmarshal(respBody, &subResp); err != nil {
		return nil, fmt.Errorf("failed to decode paypal get subscription response: %w", err)
	}

	return &financial.SubscriptionDetails{
		ProviderSubscriptionID: subResp.ID,
		Status:                 a.mapSubscriptionStatus(subResp.Status),
		CurrentPeriodStart:     subResp.BillingInfo.LastPayment.Time,
		CurrentPeriodEnd:       subResp.BillingInfo.NextBillingTime,
		CancelAtPeriodEnd:      false,
	}, nil
}

func (a *PayPalAdapter) ArchiveProduct(ctx context.Context, providerProductID string) error {
	return fmt.Errorf("%w: PayPal products cannot be archived or deactivated via the API", integration.ErrOperationNotSupported)
}

func (a *PayPalAdapter) ArchivePrice(ctx context.Context, providerPriceID string) error {
	planURL := fmt.Sprintf("%s/v1/billing/plans/%s/deactivate", a.apiBaseURL, providerPriceID)
	_, err := a.doRequest(ctx, http.MethodPost, planURL, nil)
	return err
}

func (a *PayPalAdapter) DetachPaymentMethod(ctx context.Context, providerMethodID string) error {
	return fmt.Errorf("%w: payment methods are managed within the user's PayPal account, not via API", integration.ErrOperationNotSupported)
}

func (a *PayPalAdapter) StorePaymentMethod(ctx context.Context, providerCustomerID string, paymentToken string) (*financial.StoredPaymentMethod, error) {
	return nil, fmt.Errorf("%w: PayPal payment methods are tokenized via Billing Agreements or Vault tokens, which is a different flow", integration.ErrOperationNotSupported)
}

func (a *PayPalAdapter) doRequest(ctx context.Context, method, url string, payload interface{}) ([]byte, error) {
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal paypal request payload: %w", err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create paypal request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", integration.ErrConnectionFailed, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paypal response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp payPalErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil {
			return nil, a.mapPayPalError(resp.StatusCode, &errResp)
		}
		return nil, fmt.Errorf("%w: status %d, body: %s", integration.ErrProviderError, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (a *PayPalAdapter) mapPayPalError(statusCode int, errResp *payPalErrorResponse) error {
	if errResp != nil {
		switch errResp.Name {
		case "INVALID_RESOURCE_ID":
			return fmt.Errorf("%w: %s", integration.ErrBadRequest, errResp.Message)
		case "DUPLICATE_INVOICE_ID", "DUPLICATE_REQUEST_ID":
			return financial.ErrDuplicateTransaction
		case "PERMISSION_DENIED":
			return integration.ErrPermissionDenied
		}
	}
	if statusCode >= 500 {
		return integration.ErrProviderError
	}
	if statusCode >= 400 {
		return integration.ErrBadRequest
	}
	return integration.ErrUnexpectedResponse
}

func (a *PayPalAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	var cfgData map[string]interface{}
	if err := json.Unmarshal(config.Configuration, &cfgData); err != nil {
		return "", "", nil, fmt.Errorf("%w: cannot parse integration config", integration.ErrInvalidConfiguration)
	}
	webhookID, _ := cfgData["webhook_id"].(string)
	if webhookID == "" {
		return "", "", nil, fmt.Errorf("%w: webhook_id not found in config", integration.ErrInvalidConfiguration)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read webhook body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	verifyURL := fmt.Sprintf("%s/v1/notifications/verify-webhook-signature", a.apiBaseURL)
	verifyPayload := map[string]interface{}{
		"auth_algo":         r.Header.Get("PAYPAL-AUTH-ALGO"),
		"cert_url":          r.Header.Get("PAYPAL-CERT-URL"),
		"transmission_id":   r.Header.Get("PAYPAL-TRANSMISSION-ID"),
		"transmission_sig":  r.Header.Get("PAYPAL-TRANSMISSION-SIG"),
		"transmission_time": r.Header.Get("PAYPAL-TRANSMISSION-TIME"),
		"webhook_id":        webhookID,
		"webhook_event":     json.RawMessage(body),
	}

	respBody, err := a.doRequest(r.Context(), http.MethodPost, verifyURL, verifyPayload)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %w", integration.ErrInvalidSignature, err)
	}

	var verification struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.Unmarshal(respBody, &verification); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode paypal verification response: %w", err)
	}

	if verification.VerificationStatus != "SUCCESS" {
		return "", "", nil, integration.ErrInvalidSignature
	}

	var event struct {
		ID        string `json:"id"`
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode verified paypal event body: %w", err)
	}

	a.logger.Info("PayPal webhook verified", log.String("event_id", event.ID), log.String("event_type", event.EventType))
	return event.EventType, event.ID, body, nil
}

func (a *PayPalAdapter) mapSubscriptionStatus(status string) financial.SubscriptionStatus {
	switch status {
	case "ACTIVE":
		return financial.SubStatusActive
	case "INACTIVE", "SUSPENDED", "CANCELLED", "EXPIRED":
		return financial.SubStatusEnded
	case "APPROVAL_PENDING":
		return financial.SubStatusIncomplete
	default:
		return financial.SubStatusIncomplete
	}
}

func (a *PayPalAdapter) getAccessToken(ctx context.Context) (string, error) {
	a.tokenMutex.RLock()
	if time.Now().Before(a.tokenExpiry.Add(-1 * time.Minute)) {
		token := a.accessToken
		a.tokenMutex.RUnlock()
		return token, nil
	}
	a.tokenMutex.RUnlock()

	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()
	if time.Now().Before(a.tokenExpiry.Add(-1 * time.Minute)) {
		return a.accessToken, nil
	}

	if err := a.refreshToken(ctx); err != nil {
		return "", err
	}
	return a.accessToken, nil
}

func (a *PayPalAdapter) refreshToken(ctx context.Context) error {
	a.logger.Info("Refreshing PayPal access token...")

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v1/oauth2/token", a.apiBaseURL), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(a.clientID, a.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("paypal auth failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	a.accessToken = authResp.AccessToken
	a.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	a.logger.Info("Successfully refreshed PayPal access token.")

	return nil
}

func (a *PayPalAdapter) TestConnection(ctx context.Context) error {
	// Attempt to refresh token. If credentials are bad, this will fail.
	return a.refreshToken(ctx)
}

func (a *PayPalAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	// PayPal payloads are often wrapped.
	switch wh.EventType {
	case "PAYMENT.CAPTURE.COMPLETED":
		var resource struct {
			ID     string `json:"id"`
			Amount struct {
				Value        string `json:"value"` // PayPal sends "10.00" string
				CurrencyCode string `json:"currency_code"`
			} `json:"amount"`
			CreateTime time.Time `json:"create_time"`
		}

		// Try unmarshal directly
		if err := json.Unmarshal(wh.Payload, &resource); err != nil {
			// If failed, try unwrapping "resource" key
			var wrapper struct {
				Resource json.RawMessage `json:"resource"`
			}
			if errWrap := json.Unmarshal(wh.Payload, &wrapper); errWrap == nil && len(wrapper.Resource) > 0 {
				if errRes := json.Unmarshal(wrapper.Resource, &resource); errRes != nil {
					return nil, integration.NewNonRetryableError(fmt.Errorf("failed to unmarshal resource: %w", errRes))
				}
			} else {
				// Original error was likely the correct one to report
				return nil, integration.NewNonRetryableError(err)
			}
		}

		// Logic Fix: Convert "10.00" string to int64 cents
		amountFloat, err := strconv.ParseFloat(resource.Amount.Value, 64)
		if err != nil {
			return nil, integration.NewNonRetryableError(fmt.Errorf("invalid amount format: %w", err))
		}
		amountCents := int64(amountFloat * 100)

		eventPayload := financial.PaymentSucceededPayload{
			Provider:              "paypal",
			ProviderTransactionID: resource.ID,
			Amount:                amountCents,
			Currency:              resource.Amount.CurrencyCode,
			OccurredAt:            resource.CreateTime, // Maps to 'TransactionTime' or 'OccurredAt'
			Type:                  "PAYMENT",
		}

		evt, _ := events.NewEvent(core.EventPaymentSucceeded, system.SystemActorID, "system", eventPayload)
		return &evt, nil

	default:
		return nil, nil
	}
}
