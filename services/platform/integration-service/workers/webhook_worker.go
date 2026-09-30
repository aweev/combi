package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

type WebhookWorker struct {
	repo      integration.Repository
	factory   integration.ProviderFactory
	eventBus  events.Bus
	cryptoSvc security.CryptoService
	logger    log.Logger
}

func NewWebhookWorker(
	repo integration.Repository,
	factory integration.ProviderFactory,
	eventBus events.Bus,
	cryptoSvc security.CryptoService,
	logger log.Logger,
) *WebhookWorker {
	return &WebhookWorker{
		repo:      repo,
		factory:   factory,
		eventBus:  eventBus,
		cryptoSvc: cryptoSvc,
		logger:    logger.With(log.String("worker", "webhook_processor")),
	}
}

func (w *WebhookWorker) JobType() core.JobType {
	return integration.JobIntegrationProcessWebhook
}

func (w *WebhookWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p struct {
		WebhookID string `json:"webhook_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	webhookID, err := uuid.Parse(p.WebhookID)
	if err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid webhook_id format: %w", err))
	}

	webhook, err := w.repo.GetWebhook(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found: %w", err) // Retryable
	}

	if webhook.Status == integration.WebhookProcessed {
		return nil, nil // Already done
	}

	// 1. Get Integration Config to initialize Parser
	integ, err := w.repo.GetIntegration(ctx, webhook.IntegrationID)
	if err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("integration config missing for webhook: %w", err))
	}

	// Decrypt
	credsBytes, err := w.cryptoSvc.Decrypt(ctx, integ.CredentialsEncrypted, integ.EncryptionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err) // Retryable? Maybe transient key issue
	}

	var creds map[string]string
	if err := json.Unmarshal(credsBytes, &creds); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("failed to unmarshal decrypted credentials: %w", err))
	}

	// 2. Get Parser
	parser, err := w.factory.GetWebhookParser(integ, creds)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	// 3. Convert to Domain Event
	domainEvent, err := parser.ToDomainEvent(webhook)
	if err != nil {
		if integration.IsNonRetryable(err) {
			w.markFailed(ctx, webhook, err.Error())
			return nil, nil
		}
		return nil, err // Retryable parsing error
	}

	// 4. Publish Domain Event
	if domainEvent != nil {
		if err := w.eventBus.Publish(ctx, *domainEvent); err != nil {
			return nil, fmt.Errorf("failed to publish domain event: %w", err)
		}
		w.logger.Info("Converted webhook to domain event",
			log.String("event_action", string(domainEvent.Action)),
			log.String("webhook_id", webhook.ID.String()))
	} else {
		w.logger.Info("Webhook ignored (no domain mapping)", log.String("event_type", webhook.EventType))
		webhook.Status = integration.WebhookIgnored
	}

	// 5. Mark Processed
	if webhook.Status != integration.WebhookIgnored {
		webhook.Status = integration.WebhookProcessed
	}
	now := time.Now().UTC()
	webhook.ProcessedAt = &now

	if err := w.repo.UpdateWebhook(ctx, webhook); err != nil {
		return nil, err
	}

	return nil, nil
}

func (w *WebhookWorker) markFailed(ctx context.Context, webhook *integration.Webhook, msg string) {
	webhook.Status = integration.WebhookFailed
	webhook.ErrorMessage = &msg
	now := time.Now().UTC()
	webhook.ProcessedAt = &now

	if err := w.repo.UpdateWebhook(ctx, webhook); err != nil {
		w.logger.Error("failed to update webhook status to FAILED", err, log.String("webhook_id", webhook.ID.String()))
	}
}
