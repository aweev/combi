package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type BrevoAdapter struct {
	client *http.Client
	apiKey string
	logger log.Logger
}

func NewBrevoAdapter(credentials map[string]string, logger log.Logger) (*BrevoAdapter, error) {
	apiKey, ok := credentials["api_key"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("missing 'api_key' for brevo adapter")
	}
	return &BrevoAdapter{
		client: &http.Client{Timeout: 10 * time.Second},
		apiKey: apiKey,
		logger: logger.With(log.String("adapter", "brevo")),
	}, nil
}

func (a *BrevoAdapter) Send(ctx context.Context, req notification.SendRequest) (*notification.SendResult, error) {
	url := "https://api.brevo.com/v3/smtp/email"

	to := make([]map[string]string, len(req.To))
	for i, r := range req.To {
		to[i] = map[string]string{"email": r.Email.String(), "name": r.Name}
	}

	payload := map[string]interface{}{
		"sender":      map[string]string{"name": req.From.Name, "email": req.From.Email.String()},
		"to":          to,
		"subject":     req.Subject,
		"htmlContent": req.HTMLBody,
		"textContent": req.TextBody,
		"tags":        req.Tags,
	}

	jsonBody, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	httpReq.Header.Set("api-key", a.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, integration.ErrConnectionFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brevo error %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		MessageID string `json:"messageId"`
	}
	// FIX: errcheck
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode brevo response: %w", err)
	}

	now := time.Now().UTC()
	return &notification.SendResult{
		ProviderID:  res.MessageID,
		DeliveredAt: &now, // Accepted by API
	}, nil
}

func (a *BrevoAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	// Brevo webhooks are JSON. No signature by default, rely on IP whitelist or token in URL (not standard).
	// For security, we should check a shared secret if Brevo supports it, or assume the URL itself is secret.

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", nil, err
	}

	var event struct {
		Event     string `json:"event"`
		MessageID string `json:"message-id"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return "", "", nil, integration.ErrInvalidInput
	}

	// Strip brackets from message-id if present (Brevo sends <...>)
	msgID := strings.Trim(event.MessageID, "<>")

	return event.Event, msgID, body, nil
}

func (a *BrevoAdapter) TestConnection(ctx context.Context) error {
	// Brevo has a specific 'get account' endpoint
	url := "https://api.brevo.com/v3/account"
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("api-key", a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return integration.ErrConnectionFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("brevo returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *BrevoAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	var payload struct {
		Event     string `json:"event"`
		Email     string `json:"email"`
		MessageID string `json:"message-id"`
		Reason    string `json:"reason"`
		Date      string `json:"date"` // Brevo sends YYYY-MM-DD HH:mm:ss usually
	}

	if err := json.Unmarshal(wh.Payload, &payload); err != nil {
		return nil, integration.NewNonRetryableError(err)
	}

	if payload.Event == "hard_bounce" || payload.Event == "spam" {
		// Parse time if possible, else use current
		bouncedAt := time.Now().UTC()
		// Add parsing logic here if needed

		evtPayload := notification.EmailBouncedPayload{
			RecipientEmail:    payload.Email,
			Provider:          "brevo",
			ProviderMessageID: strings.Trim(payload.MessageID, "<>"),
			BounceType:        payload.Event,
			Reason:            payload.Reason,
			BouncedAt:         bouncedAt,
		}
		evt, _ := events.NewEvent(core.EventEmailBounced, system.SystemActorID, "system", evtPayload)
		return &evt, nil
	}
	return nil, nil
}
