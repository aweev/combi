package adapters

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1" // nolint:gosec // Legacy AWS requirement for SNS signature verification
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

var _ notification.EmailProvider = (*SESAdapter)(nil)
var _ integration.WebhookParser = (*SESAdapter)(nil)

// Cache for SNS signing certificates to avoid fetching on every webhook
var certCache = sync.Map{}

type SESAdapter struct {
	client     *sesv2.Client
	httpClient *http.Client // for SNS cert fetching
	logger     log.Logger
}

func NewSESAdapter(credentialsMap map[string]string, logger log.Logger) (*SESAdapter, error) {
	accessKeyID := credentialsMap["access_key_id"]
	secretAccessKey := credentialsMap["secret_access_key"]
	region := credentialsMap["region"]

	if accessKeyID == "" || secretAccessKey == "" || region == "" {
		return nil, fmt.Errorf("%w: SES requires access_key_id, secret_access_key, and region", integration.ErrMissingCredentials)
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &SESAdapter{
		client:     sesv2.NewFromConfig(cfg),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger.With(log.String("adapter", "ses")),
	}, nil
}

// --- EmailProvider Implementation ---

func (a *SESAdapter) Send(ctx context.Context, req notification.SendRequest) (*notification.SendResult, error) {
	destinations := make([]string, len(req.To))
	for i, to := range req.To {
		destinations[i] = to.Email.String()
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(req.From.Email.String()),
		Destination: &types.Destination{
			ToAddresses: destinations,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(req.Subject)},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(req.HTMLBody)},
				},
			},
		},
	}

	if req.TextBody != "" {
		input.Content.Simple.Body.Text = &types.Content{Data: aws.String(req.TextBody)}
	}

	output, err := a.client.SendEmail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", integration.ErrProviderError, err)
	}

	now := time.Now().UTC()
	return &notification.SendResult{
		ProviderID:  *output.MessageId,
		DeliveredAt: &now, // Sent to SES, not user delivery
	}, nil
}

// --- WebhookParser Implementation (SNS) ---

type snsMessage struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Subject          string `json:"Subject"`
	Message          string `json:"Message"` // The actual SES event is here
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL"`
}

func (a *SESAdapter) Parse(r *http.Request, config *integration.Integration) (string, string, []byte, error) {
	// SES notifications come via SNS.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", nil, err
	}

	var msg snsMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return "", "", nil, fmt.Errorf("failed to unmarshal SNS message: %w", err)
	}

	// Handle Subscription Confirmation automatically
	if msg.Type == "SubscriptionConfirmation" {
		a.logger.Info("Auto-confirming SNS subscription", log.String("url", msg.SubscribeURL))

		// FIX: Use context and check response
		ctx := r.Context()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, msg.SubscribeURL, nil)
		if err != nil {
			return "", "", nil, err
		}

		resp, err := a.httpClient.Do(req)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to confirm subscription: %w", err)
		}
		// FIX: Body must be closed
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", "", nil, fmt.Errorf("subscription confirmation failed with status: %d", resp.StatusCode)
		}

		return "SNS_SUBSCRIPTION", msg.MessageId, body, nil
	}

	// Verify SNS Signature
	if err := a.verifySNSSignature(&msg); err != nil {
		return "", "", nil, integration.ErrInvalidSignature
	}

	// Extract SES event
	var sesEvent struct {
		EventType string `json:"eventType"` // e.g. Bounce, Complaint
		Mail      struct {
			MessageID string `json:"messageId"`
		} `json:"mail"`
	}

	if err := json.Unmarshal([]byte(msg.Message), &sesEvent); err != nil {
		// Might be a raw SNS message not from SES?
		return "SNS_NOTIFICATION", msg.MessageId, body, nil
	}

	// We return the SES event type (Bounce/Complaint) and the SES Message ID
	return sesEvent.EventType, sesEvent.Mail.MessageID, []byte(msg.Message), nil
}

func (a *SESAdapter) verifySNSSignature(msg *snsMessage) error {
	// 1. Get Cert
	cert, err := a.getCert(msg.SigningCertURL)
	if err != nil {
		return err
	}

	// 2. Build String to Sign (simplified, full spec is complex)
	// For prod, use the AWS SDK's verification utility if available or strict canonicalization.
	// This block reconstructs the string based on SNS docs.
	var builder strings.Builder
	builder.WriteString("Message\n")
	builder.WriteString(msg.Message + "\n")
	builder.WriteString("MessageId\n")
	builder.WriteString(msg.MessageId + "\n")
	if msg.Subject != "" {
		builder.WriteString("Subject\n")
		builder.WriteString(msg.Subject + "\n")
	}
	builder.WriteString("Timestamp\n")
	builder.WriteString(msg.Timestamp + "\n")
	builder.WriteString("TopicArn\n")
	builder.WriteString(msg.TopicArn + "\n")
	builder.WriteString("Type\n")
	builder.WriteString(msg.Type + "\n")

	// 3. Verify
	decodedSig, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return err
	}

	// nolint:gosec // SHA1 required by AWS SNS legacy spec
	hasher := sha1.New()
	hasher.Write([]byte(builder.String()))

	return rsa.VerifyPKCS1v15(cert.PublicKey.(*rsa.PublicKey), crypto.SHA1, hasher.Sum(nil), decodedSig)
}

func (a *SESAdapter) getCert(urlStr string) (*x509.Certificate, error) {
	// Basic security check on URL
	u, err := url.Parse(urlStr)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Host, ".amazonaws.com") {
		return nil, fmt.Errorf("invalid cert url")
	}

	if val, ok := certCache.Load(urlStr); ok {
		return val.(*x509.Certificate), nil
	}

	// FIX: Use context for get
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// FIX: Body must be closed
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode cert pem")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	certCache.Store(urlStr, cert)
	return cert, nil
}

func (a *SESAdapter) ToDomainEvent(wh *integration.Webhook) (*events.DomainEvent, error) {
	// Note: Parse() already extracted the inner SES JSON message into wh.Payload.

	var sesEvent struct {
		EventType string `json:"eventType"`
		Mail      struct {
			MessageID   string   `json:"messageId"`
			Destination []string `json:"destination"`
		} `json:"mail"`
		Bounce struct {
			BounceType    string `json:"bounceType"`
			BounceSubType string `json:"bounceSubType"`
		} `json:"bounce"`
		Complaint struct {
			ComplaintFeedbackType string `json:"complaintFeedbackType"`
		} `json:"complaint"`
	}

	if err := json.Unmarshal(wh.Payload, &sesEvent); err != nil {
		return nil, integration.NewNonRetryableError(err)
	}

	recipient := ""
	if len(sesEvent.Mail.Destination) > 0 {
		recipient = sesEvent.Mail.Destination[0]
	}

	if sesEvent.EventType == "Bounce" {
		evtPayload := notification.EmailBouncedPayload{
			RecipientEmail:    recipient,
			Provider:          "ses",
			ProviderMessageID: sesEvent.Mail.MessageID,
			BounceType:        sesEvent.Bounce.BounceType,
			Reason:            sesEvent.Bounce.BounceSubType,
			BouncedAt:         time.Now().UTC(),
		}
		evt, _ := events.NewEvent(core.EventEmailBounced, system.SystemActorID, "system", evtPayload)
		return &evt, nil
	}

	if sesEvent.EventType == "Complaint" {
		// Treat complaints similar to bounces for reputation management
		evtPayload := notification.EmailBouncedPayload{
			RecipientEmail:    recipient,
			Provider:          "ses",
			ProviderMessageID: sesEvent.Mail.MessageID,
			BounceType:        "complaint",
			Reason:            sesEvent.Complaint.ComplaintFeedbackType,
			BouncedAt:         time.Now().UTC(),
		}
		evt, _ := events.NewEvent(core.EventEmailBounced, system.SystemActorID, "system", evtPayload)
		return &evt, nil
	}

	return nil, nil
}
