package notification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
)

// SendRecipient defines a single recipient for an email.
type SendRecipient struct {
	Name  string
	Email core.Email
}

// SendRequest is a generic, provider-agnostic DTO for sending a single transactional email.
type SendRequest struct {
	From        SendRecipient
	To          []SendRecipient
	Subject     string
	HTMLBody    string
	TextBody    string // Optional plain text version
	Tags        []string
	ReferenceID string // An internal ID to correlate with this send request
}

// SendResult contains the provider-specific information after a successful send.

type SendResult struct {
	Error       string
	ProviderID  string
	DeliveredAt *time.Time
}

// EmailProvider defines the contract that all email integration adapters must fulfill.
// This allows the notification service to send emails without knowing the specific provider.
type EmailProvider interface {
	Send(ctx context.Context, req SendRequest) (*SendResult, error)
}

// --- DTOs for SMS ---
type SendSMSRequest struct {
	To          string // E.164 format phone number
	Body        string
	ReferenceID string // Internal ID for correlation
}

type SendSMSResult struct {
	ProviderID string
}

// SMSProvider defines the contract for all SMS integration adapters.
type SMSProvider interface {
	Send(ctx context.Context, req SendSMSRequest) (*SendSMSResult, error)
}

type SendPushRequest struct {
	DeviceToken string
	Title       string
	Body        string
	Data        map[string]any
	ReferenceID string
}

type SendPushResult struct {
	ProviderID string
}

type PushProvider interface {
	Send(ctx context.Context, req SendPushRequest) (*SendPushResult, error)
}

// --- DTOs for Web Push ---
type JSONB json.RawMessage
type SendWebPushRequest struct {
	Subscription JSONB // The browser subscription object (endpoint, keys)
	Title        string
	Body         string
	Icon         string
	Link         string
}

type WebPushProvider interface {
	Send(ctx context.Context, req SendWebPushRequest) (*SendResult, error)
}
