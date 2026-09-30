package integration

import (
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/security"
	"github.com/google/uuid"
)

type IntegrationStatus string

const (
	StatusActive   IntegrationStatus = "ACTIVE"
	StatusInactive IntegrationStatus = "INACTIVE"
	StatusError    IntegrationStatus = "ERROR"
)

type WebhookStatus string

const (
	WebhookPending   WebhookStatus = "PENDING"
	WebhookProcessed WebhookStatus = "PROCESSED"
	WebhookFailed    WebhookStatus = "FAILED"
	WebhookIgnored   WebhookStatus = "IGNORED"
)

// Integration stores configuration for external providers.
type Integration struct {
	ID                   uuid.UUID
	Provider             core.ProviderType // Uses shared core enum
	Name                 string
	Status               IntegrationStatus
	CredentialsEncrypted []byte
	EncryptionContext    *security.EncryptionContext
	Configuration        json.RawMessage // Non-sensitive config (e.g. webhook IDs)
	LastError            *string
	LastErrorAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Webhook is the immutable record of an incoming event.
type Webhook struct {
	ID              uuid.UUID
	IntegrationID   uuid.UUID
	Provider        core.ProviderType
	ProviderEventID string
	EventType       string
	Status          WebhookStatus
	Payload         json.RawMessage
	ErrorMessage    *string
	RetryCount      int
	ReceivedAt      time.Time
	ProcessedAt     *time.Time
}
