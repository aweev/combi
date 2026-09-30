package integration

import (
	"context"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

type Repository interface {
	// Integration CRUD
	CreateIntegration(ctx context.Context, i *Integration) error
	GetIntegration(ctx context.Context, id uuid.UUID) (*Integration, error)
	GetActiveIntegrationByProvider(ctx context.Context, provider core.ProviderType) (*Integration, error)
	UpdateIntegration(ctx context.Context, i *Integration) error
	DeleteIntegration(ctx context.Context, id uuid.UUID) error
	ListIntegrations(ctx context.Context, filter IntegrationFilter) ([]*Integration, int, error)

	// Webhook Management
	CreateWebhook(ctx context.Context, w *Webhook) error
	GetWebhook(ctx context.Context, id uuid.UUID) (*Webhook, error)
	GetWebhookByProviderID(ctx context.Context, provider core.ProviderType, eventID string) (*Webhook, error)
	UpdateWebhook(ctx context.Context, w *Webhook) error
}
