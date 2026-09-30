package adapters

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ notification.EmailProvider = (*SendGridAdapter)(nil)
var _ integration.WebhookParser = (*SendGridAdapter)(nil)

type SendGridAdapter struct {
	client *sendgrid.Client
	logger log.Logger
	apiKey string
}

type SendGridEvent struct {
	Email       string `json:"email"`
	Timestamp   int64  `json:"timestamp"`
	Event       string `json:"event"`
	SgEventID   string `json:"sg_event_id"`
	SgMessageID string `json:"sg_message_id"`
	Response    string `json:"response,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Status      string `json:"status,omitempty"`
}

func NewSendGridAdapter(credentials map[string]string, logger log.Logger) (*SendGridAdapter, error) {
	apiKey, ok := credentials["api_key"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("%w: SendGrid adapter requires 'api_key'", integration.ErrMissingCredentials)
	}

	return &SendGridAdapter{
		client: sendgrid.NewSendClient(apiKey),
		apiKey: apiKey,
		logger: logger.With(log.String("adapter", "sendgrid")),
	}, nil
}

// --- EmailProvider Implementation ---

func (a *SendGridAdapter) Send(ctx context.Context, req notification.SendRequest) (*notification.SendResult, error) {
	from := mail.NewEmail(req.From.Name, req.From.Email.String())
	subject := req.Subject

	// 1. Create the Personalization object first
	p := mail.NewPersonalization()

	// 2. Add Recipients
	for _, r := range req.To {
		p.AddTos(mail.NewEmail(r.Name, r.Email.String()))
	}

	// 3. Add Custom Arg to this Personalization
	// This ensures the reference_id is sent with this specific batch of emails.
	p.SetCustomArg("reference_id", req.ReferenceID)

	// 4. Create Mail object and add the configured personalization
	m := mail.NewV3Mail()
	m.SetFrom(from)
	m.Subject = subject
	m.AddPersonalizations(p)

	// 5. Add Content
	content := mail.NewContent("text/html", req.HTMLBody)
	m.AddContent(content)

	if req.TextBody != "" {
		m.AddContent(mail.NewContent("text/plain", req.TextBody))
	}

	// 6. Send
	resp, err := a.client.SendWithContext(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", integration.ErrConnectionFailed, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: status %d, body: %s", integration.ErrProviderError, resp.StatusCode, resp.Body)
	}

	// Extract Message ID
	msgID := ""
	if values, ok := resp.Headers["X-Message-Id"]; ok && len(values) > 0 {
		msgID = values[0]
	}

	return &notification.SendResult{
		ProviderID: msgID,
		// DeliveredAt is nil because SendGrid is async
	}, nil
}

// --- ConnectionTester Implementation ---

func (a *SendGridAdapter) TestConnection(ctx context.Context) error {
	request := sendgrid.GetRequest(a.apiKey, "/v3/user/profile", "")
	request.Method = "GET"

	resp, err := sendgrid.API(request)
	if err != nil {
		return fmt.Errorf("%w: %v", integration.ErrConnectionFailed, err)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("%w: invalid api key permissions", integration.ErrAuthenticationFailed)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sendgrid returned status %d", resp.StatusCode)
	}

	return nil
}

// --- WebhookParser Implementation ---

func (a *SendGridAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	var cfgMap map[string]any
	if err := json.Unmarshal(config.Configuration, &cfgMap); err != nil {
		return "", "", nil, integration.ErrInvalidConfiguration
	}

	publicKeyStr, _ := cfgMap["webhook_verification_key"].(string)
	if publicKeyStr == "" {
		return "", "", nil, fmt.Errorf("%w: missing 'webhook_verification_key'", integration.ErrInvalidConfiguration)
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read body: %w", err)
	}

	signature := r.Header.Get("X-Twilio-Email-Event-Webhook-Signature")
	timestampStr := r.Header.Get("X-Twilio-Email-Event-Webhook-Timestamp")

	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid timestamp header")
	}
	eventTime := time.Unix(ts, 0)
	// Check if timestamp is within 5 minutes of now (past or future)
	if time.Since(eventTime) > 5*time.Minute || time.Until(eventTime) > 5*time.Minute {
		a.logger.Warn("SendGrid webhook timestamp stale", log.String("timestamp", timestampStr))
		return "", "", nil, integration.ErrInvalidSignature // Reuse error to obscure details
	}

	if err := verifySignature(publicKeyStr, payload, signature, timestampStr); err != nil {
		a.logger.Warn("SendGrid signature verification failed", log.ErrorAttr(err))
		return "", "", nil, integration.ErrInvalidSignature
	}

	var events []SendGridEvent
	if err := json.Unmarshal(payload, &events); err != nil {
		return "", "", nil, fmt.Errorf("%w: %v", integration.ErrInvalidInput, err)
	}

	if len(events) == 0 {
		return "ping", "ping-" + fmt.Sprint(time.Now().UnixNano()), payload, nil
	}

	firstEvent := events[0]

	return firstEvent.Event, firstEvent.SgEventID, payload, nil
}

func verifySignature(publicKeyStr string, payload []byte, signature string, timestamp string) error {
	block, _ := pem.Decode([]byte(publicKeyStr))
	if block == nil {
		return errors.New("invalid public key format")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}
	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("public key is not ECDSA")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errors.New("invalid signature base64")
	}

	type ecdsaSignature struct {
		R, S *big.Int
	}
	var esig ecdsaSignature
	if _, err := asn1.Unmarshal(sigBytes, &esig); err != nil {
		return fmt.Errorf("failed to unmarshal asn1 signature: %w", err)
	}

	signedData := append([]byte(timestamp), payload...)
	hash := sha256.Sum256(signedData)

	if !ecdsa.Verify(ecdsaPubKey, hash[:], esig.R, esig.S) {
		return errors.New("signature verification failed")
	}

	return nil
}

func (a *SendGridAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	var eventsList []SendGridEvent
	if err := json.Unmarshal(wh.Payload, &eventsList); err != nil {
		return nil, err
	}
	if len(eventsList) == 0 {
		return nil, nil
	}
	eventData := eventsList[0]

	if eventData.Event == "bounce" || eventData.Event == "dropped" {
		evtPayload := notification.EmailBouncedPayload{
			RecipientEmail:    eventData.Email,
			Provider:          "sendgrid",
			ProviderMessageID: eventData.SgMessageID,
			BounceType:        eventData.Event,
			Reason:            eventData.Reason,
			BouncedAt:         time.Now().UTC(),
		}
		evt, _ := events.NewEvent(core.EventEmailBounced, system.SystemActorID, "system", evtPayload)
		return &evt, nil
	}
	return nil, nil
}
