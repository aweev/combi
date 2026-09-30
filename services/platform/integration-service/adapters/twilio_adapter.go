package adapters

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Statically verify implementation.
var _ notification.SMSProvider = (*TwilioAdapter)(nil)
var _ integration.WebhookParser = (*TwilioAdapter)(nil)

const twilioAPI = "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json"

type TwilioAdapter struct {
	client     *http.Client
	logger     log.Logger
	accountSid string
	authToken  string
	fromNumber string
}

func NewTwilioAdapter(credentials map[string]string, logger log.Logger) (*TwilioAdapter, error) {
	accountSid, ok := credentials["account_sid"]
	if !ok || accountSid == "" {
		return nil, fmt.Errorf("%w: Twilio adapter requires 'account_sid'", integration.ErrMissingCredentials)
	}
	authToken, ok := credentials["auth_token"]
	if !ok || authToken == "" {
		return nil, fmt.Errorf("%w: Twilio adapter requires 'auth_token'", integration.ErrMissingCredentials)
	}
	fromNumber, ok := credentials["from_number"]
	if !ok || fromNumber == "" {
		return nil, fmt.Errorf("%w: Twilio adapter requires 'from_number'", integration.ErrMissingCredentials)
	}

	return &TwilioAdapter{
		client:     &http.Client{Timeout: 10 * time.Second},
		logger:     logger.With(log.String("adapter", "twilio")),
		accountSid: accountSid,
		authToken:  authToken,
		fromNumber: fromNumber,
	}, nil
}

// --- SMSProvider Implementation ---

func (a *TwilioAdapter) Send(ctx context.Context, req notification.SendSMSRequest) (*notification.SendSMSResult, error) {
	apiURL := fmt.Sprintf(twilioAPI, a.accountSid)

	data := url.Values{}
	data.Set("To", req.To)
	data.Set("From", a.fromNumber)
	data.Set("Body", req.Body)
	// Note: ReferenceID (req.ReferenceID) could be stored in StatusCallback args if needed,
	// but Twilio relies on the returned SID for tracking.

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create twilio request: %w", err)
	}

	httpReq.SetBasicAuth(a.accountSid, a.authToken)
	httpReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", integration.ErrConnectionFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d, body: %s", integration.ErrProviderError, resp.StatusCode, string(bodyBytes))
	}

	var twilioResp struct {
		Sid string `json:"sid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&twilioResp); err != nil {
		return nil, fmt.Errorf("failed to decode twilio response: %w", err)
	}

	return &notification.SendSMSResult{
		ProviderID: twilioResp.Sid,
	}, nil
}

// --- WebhookParser Implementation ---

func (a *TwilioAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	// Twilio webhooks are form-encoded, not JSON.
	// Signature validation is critical here.
	// We use the auth token from the adapter (initialized with decrypted creds) for verification.
	if err := a.verifyTwilioSignature(r, a.authToken); err != nil {
		a.logger.Warn("Twilio signature verification failed", log.ErrorAttr(err))
		return "", "", nil, integration.ErrInvalidSignature
	}

	if err := r.ParseForm(); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse twilio webhook form: %w", err)
	}

	eventType := r.Form.Get("MessageStatus") // e.g. "sent", "delivered", "undelivered", "failed"
	providerEventID := r.Form.Get("MessageSid")

	// We convert the form data to JSON for consistent storage in our Webhook table.
	formData := make(map[string]string)
	for key, values := range r.Form {
		formData[key] = values[0]
	}
	payload, _ := json.Marshal(formData)

	a.logger.Info("Twilio webhook verified", log.String("event_id", providerEventID), log.String("event_type", eventType))
	return eventType, providerEventID, payload, nil
}

func (a *TwilioAdapter) verifyTwilioSignature(r *http.Request, authToken string) error {
	signature := r.Header.Get("X-Twilio-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Twilio-Signature header")
	}

	// Reconstruct the full URL (protocol + host + path + query)
	// Note: r.URL.Scheme is often empty in server handlers, assume https or check headers.
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "http" {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.String())

	// Read body to use in signature calculation if strictly needed,
	// but Twilio signs the sorted form parameters.
	// We need to parse the form to sort keys.
	// NOTE: r.ParseForm() consumes the body. We must restore it if we want downstream to read it,
	// though in this architecture, Parse() returns the payload bytes anyway.

	// Copy body for safety
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Parse form values
	formParams, err := url.ParseQuery(string(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to parse body for signature verification: %w", err)
	}

	// Sort keys
	var keys []string
	for k := range formParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build signature string
	var builder strings.Builder
	builder.WriteString(fullURL)
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString(formParams.Get(k))
	}

	// HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(builder.String()))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if signature != expectedSignature {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func (a *TwilioAdapter) SetClient(client *http.Client) {
	a.client = client
}

func (a *TwilioAdapter) TestConnection(ctx context.Context) error {
	// Fetch the account details to verify credentials
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s.json", a.accountSid)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.SetBasicAuth(a.accountSid, a.authToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return integration.ErrConnectionFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("twilio returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *TwilioAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	var payload struct {
		MessageSid    string `json:"MessageSid"`
		MessageStatus string `json:"MessageStatus"`
		ErrorCode     string `json:"ErrorCode"`
		ErrorMessage  string `json:"ErrorMessage"`
		To            string `json:"To"`
	}
	if err := json.Unmarshal(wh.Payload, &payload); err != nil {
		return nil, integration.NewNonRetryableError(err)
	}

	if payload.MessageStatus == "failed" || payload.MessageStatus == "undelivered" {
		evtPayload := notification.SMSFailedPayload{
			Provider:          "twilio",
			ProviderMessageID: payload.MessageSid,
			RecipientNumber:   payload.To,
			ErrorCode:         payload.ErrorCode,
			ErrorMessage:      payload.ErrorMessage,
			FailedAt:          time.Now().UTC(),
		}
		evt, _ := events.NewEvent(core.EventSMSFailed, system.SystemActorID, "system", evtPayload)
		return &evt, nil
	}
	return nil, nil
}
