package workers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	int_mocks "github.com/combi/pkg/domain/integration/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	event_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	security_mocks "github.com/combi/pkg/security/mocks"
	"github.com/combi/integration/workers"
)

func TestWebhookWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	webhookID := uuid.New()
	integID := uuid.New()

	payloadBytes, _ := json.Marshal(map[string]string{"webhook_id": webhookID.String()})
	job := &queue.Job{ID: uuid.New(), Payload: payloadBytes}

	t.Run("should process valid webhook and publish event", func(t *testing.T) {
		repo := int_mocks.NewMockRepository(t)
		factory := int_mocks.NewMockProviderFactory(t)
		bus := event_mocks.NewMockBus(t)
		crypto := security_mocks.NewMockCryptoService(t)
		parser := int_mocks.NewMockWebhookParser(t)
		worker := workers.NewWebhookWorker(repo, factory, bus, crypto, &log.NoOpLogger{})

		webhook := &integration.Webhook{ID: webhookID, IntegrationID: integID, Status: integration.WebhookPending}
		integ := &integration.Integration{ID: integID, CredentialsEncrypted: []byte("enc")}

		repo.EXPECT().GetWebhook(ctx, webhookID).Return(webhook, nil).Once()
		repo.EXPECT().GetIntegration(ctx, integID).Return(integ, nil).Once()

		crypto.EXPECT().Decrypt(ctx, integ.CredentialsEncrypted, mock.Anything).Return([]byte(`{}`), nil).Once()
		factory.EXPECT().GetWebhookParser(integ, mock.Anything).Return(parser, nil).Once()

		// Parser returns a valid event
		domainEvt := &events.DomainEvent{Action: core.EventAction("test.event")}
		parser.EXPECT().ToDomainEvent(webhook).Return(domainEvt, nil).Once()

		bus.EXPECT().Publish(ctx, *domainEvt).Return(nil).Once()

		repo.EXPECT().UpdateWebhook(ctx, mock.MatchedBy(func(w *integration.Webhook) bool {
			return w.Status == integration.WebhookProcessed && w.ProcessedAt != nil
		})).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("should mark as ignored if no domain mapping", func(t *testing.T) {
		// ... setup mocks ...
		repo := int_mocks.NewMockRepository(t)
		factory := int_mocks.NewMockProviderFactory(t)
		crypto := security_mocks.NewMockCryptoService(t)
		parser := int_mocks.NewMockWebhookParser(t)
		// Bus not needed
		worker := workers.NewWebhookWorker(repo, factory, nil, crypto, &log.NoOpLogger{})

		webhook := &integration.Webhook{ID: webhookID, IntegrationID: integID}
		integ := &integration.Integration{ID: integID}

		repo.EXPECT().GetWebhook(ctx, webhookID).Return(webhook, nil).Once()
		repo.EXPECT().GetIntegration(ctx, integID).Return(integ, nil).Once()
		crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return([]byte(`{}`), nil).Once()
		factory.EXPECT().GetWebhookParser(integ, mock.Anything).Return(parser, nil).Once()

		// Parser returns nil (no mapping)
		parser.EXPECT().ToDomainEvent(webhook).Return(nil, nil).Once()

		repo.EXPECT().UpdateWebhook(ctx, mock.MatchedBy(func(w *integration.Webhook) bool {
			return w.Status == integration.WebhookIgnored
		})).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("should retry on transient errors", func(t *testing.T) {
		repo := int_mocks.NewMockRepository(t)
		worker := workers.NewWebhookWorker(repo, nil, nil, nil, &log.NoOpLogger{})

		repo.EXPECT().GetWebhook(ctx, webhookID).Return(nil, errors.New("db timeout")).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.Error(t, err) // Queue will retry
	})

	t.Run("should fail non-retryably on parser fatal error", func(t *testing.T) {
		repo := int_mocks.NewMockRepository(t)
		factory := int_mocks.NewMockProviderFactory(t)
		crypto := security_mocks.NewMockCryptoService(t)
		parser := int_mocks.NewMockWebhookParser(t)
		worker := workers.NewWebhookWorker(repo, factory, nil, crypto, &log.NoOpLogger{})

		webhook := &integration.Webhook{ID: webhookID, IntegrationID: integID}
		integ := &integration.Integration{ID: integID}

		repo.EXPECT().GetWebhook(ctx, webhookID).Return(webhook, nil).Once()
		repo.EXPECT().GetIntegration(ctx, integID).Return(integ, nil).Once()
		crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return([]byte(`{}`), nil).Once()
		factory.EXPECT().GetWebhookParser(integ, mock.Anything).Return(parser, nil).Once()

		// Parser returns NonRetryable
		fatalErr := integration.NewNonRetryableError(errors.New("bad payload"))
		parser.EXPECT().ToDomainEvent(webhook).Return(nil, fatalErr).Once()

		repo.EXPECT().UpdateWebhook(ctx, mock.MatchedBy(func(w *integration.Webhook) bool {
			return w.Status == integration.WebhookFailed && *w.ErrorMessage == "non-retryable integration error: bad payload"
		})).Return(nil).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err) // Job succeeds (marked as failed in logic)
	})
}
