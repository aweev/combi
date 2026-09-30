package integration

import (
	"context"
	"net/http"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/events"
	"github.com/google/uuid"
)

// ============================================================================
// INBOUND PORTS (Application Services)
// These define what the application can do. They are called by handlers or other services.
// ============================================================================

// AdminService defines the contract for managing integration configurations.
type AdminService interface {
	Create(ctx context.Context, params CreateIntegrationParams) (*Integration, error)
	Get(ctx context.Context, id uuid.UUID) (*Integration, error)
	// GetWithCredentials returns the integration and its decrypted credentials. For admin use only.
	GetWithCredentials(ctx context.Context, id uuid.UUID) (*Integration, map[string]string, error)
	List(ctx context.Context, filter IntegrationFilter) ([]*Integration, int, error)
	Update(ctx context.Context, id uuid.UUID, params UpdateIntegrationParams) (*Integration, error)
	Delete(ctx context.Context, id uuid.UUID) error
	TestConnection(ctx context.Context, id uuid.UUID) (bool, string, error)
}

// ConnectionTester is an optional interface that adapters can implement
// to verify their credentials are valid.
type ConnectionTester interface {
	TestConnection(ctx context.Context) error
}

// WebhookService defines the contract for handling incoming webhooks.
type WebhookService interface {
	ProcessWebhook(ctx context.Context, providerType core.ProviderType, r *http.Request) (*Webhook, error)
}

type ProviderFactory interface {
	GetPaymentGateway(ctx context.Context) (financial.PaymentGateway, error)
	BuildPaymentGateway(provider core.ProviderType, creds map[string]string) (financial.PaymentGateway, error)
	BuildEmailProvider(provider core.ProviderType, creds map[string]string) (notification.EmailProvider, error)
	BuildSMSProvider(provider core.ProviderType, creds map[string]string) (notification.SMSProvider, error)
	BuildPushProvider(typ core.ProviderType, config map[string]string) (notification.PushProvider, error)
	BuildWebPushProvider(typ core.ProviderType, config map[string]string) (notification.WebPushProvider, error)
	GetWebhookParser(config *Integration, creds map[string]string) (WebhookParser, error)
}

// WebhookParser is an OUTBOUND PORT that defines the contract for parsing an incoming HTTP request
// from a specific provider into a standardized format. Each provider (Stripe, Brevo) will have its own implementation.
type WebhookParser interface {
	// Parse validates the request and extracts standard metadata.
	Parse(r *http.Request, config *Integration) (eventType string, providerEventID string, payload []byte, err error)

	// ToDomainEvent maps the validated webhook to a canonical system event.
	// It returns nil if the webhook event is not relevant to the system.
	ToDomainEvent(webhook *Webhook) (*events.DomainEvent, error)
}

// ============================================================================
// DTOs (Data Transfer Objects)
// ============================================================================

type CreateIntegrationParams struct {
	Provider      core.ProviderType `json:"provider" validate:"required,oneof=STRIPE BREVO TWILIO PAYPAL SES SMTP"`
	Name          string            `json:"name" validate:"required,min=3,max=100"`
	Credentials   map[string]string `json:"credentials" validate:"required"`
	Configuration map[string]any    `json:"configuration"`
}

type UpdateIntegrationParams struct {
	Name          *string            `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Status        *IntegrationStatus `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE"`
	Credentials   map[string]string  `json:"credentials,omitempty"`
	Configuration map[string]any     `json:"configuration,omitempty"`
}

type IntegrationFilter struct {
	Provider *core.ProviderType
	Status   *IntegrationStatus
	Limit    int
	Offset   int
}
