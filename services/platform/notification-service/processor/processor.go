package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Processor handles the send lifecycle.
type Processor interface {
	Process(ctx context.Context, notificationID string) error
	SetUserLocaleFetcher(fetcher notification.UserLocaleFetcher)
}

var _ Processor = (*processorImpl)(nil)

type processorImpl struct {
	repo             notification.Repository
	renderer         notification.TemplateRenderer
	prefs            notification.PreferenceChecker
	factory          integration.ProviderFactory
	crypto           security.CryptoService
	eventBus         events.Bus
	dispatcher       queue.Dispatcher
	logger           log.Logger
	defaultFromEmail core.Email
	userFetcher      notification.UserLocaleFetcher
}

type Config struct {
	Repo             notification.Repository
	Renderer         notification.TemplateRenderer
	Prefs            notification.PreferenceChecker
	Factory          integration.ProviderFactory
	Crypto           security.CryptoService
	EventBus         events.Bus
	Dispatcher       queue.Dispatcher
	Logger           log.Logger
	DefaultFromEmail string
}

func NewProcessor(cfg Config) Processor {
	fromEmail, err := core.NewEmail(cfg.DefaultFromEmail)
	if err != nil {
		if cfg.DefaultFromEmail == "" {
			fromEmail, _ = core.NewEmail("dev@ngo-platform.org")
		} else {
			// In production startup, we want to know immediately if config is bad
			panic(fmt.Sprintf("invalid default from email: %s", cfg.DefaultFromEmail))
		}
	}

	return &processorImpl{
		repo:             cfg.Repo,
		renderer:         cfg.Renderer,
		prefs:            cfg.Prefs,
		factory:          cfg.Factory,
		crypto:           cfg.Crypto,
		eventBus:         cfg.EventBus,
		dispatcher:       cfg.Dispatcher,
		logger:           cfg.Logger.With(log.String("component", "notification_processor")),
		defaultFromEmail: fromEmail,
	}
}

func (p *processorImpl) SetUserLocaleFetcher(fetcher notification.UserLocaleFetcher) {
	p.userFetcher = fetcher
}

func (p *processorImpl) Process(ctx context.Context, notificationIDStr string) error {
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		return queue.NewNonRetryableError(fmt.Errorf("invalid notification ID: %w", err))
	}

	logger := p.logger.With(log.String("notification_id", notificationID.String()))

	return p.repo.WithTx(ctx, func(txRepo notification.Repository) error {
		// 1. Lock the notification for update
		notif, err := txRepo.GetNotificationForUpdate(ctx, notificationID)
		if err != nil {
			if err == notification.ErrNotificationNotFound {
				return queue.NewNonRetryableError(err)
			}
			return err
		}

		// 2. Idempotency Check
		if notif.Status == notification.StatusSent || notif.Status == notification.StatusDelivered {
			logger.Info("Notification already sent, skipping.")
			return nil
		}
		if notif.Status == notification.StatusCancelled {
			logger.Info("Notification was cancelled, skipping.")
			return nil
		}

		// 3. Preference Check
		canSend, err := p.prefs.CanSend(ctx, notif.RecipientID, notif.Channel, notif.Category, notif.TemplateID)
		if err != nil {
			return err
		}
		if !canSend {
			return p.updateState(ctx, txRepo, notif, notification.StatusCancelled, nil, &notification.SendResult{Error: "User opted out"})
		}

		// 4. Template Rendering (if needed)
		if notif.TemplateID != nil && (notif.Body == "" || (notif.Subject == nil && notif.Channel == notification.ChannelEmail)) {
			// --- LOCALE DETERMINATION LOGIC ---
			locale := "en-US" // Default fallback

			// Priority 1: Locale explicitly set on the notification (requires DB column update)
			// if notif.Locale != nil { locale = *notif.Locale }

			// Priority 2: Fetch user profile preference
			if p.userFetcher != nil && notif.RecipientID != uuid.Nil {
				userLocale, err := p.userFetcher.GetUserLocale(ctx, notif.RecipientID)
				if err == nil && userLocale != "" {
					locale = userLocale
				} else if err != nil {
					// Log but don't fail; fall back to default
					logger.Warn("Failed to fetch user locale, using default", log.ErrorAttr(err))
				}
			}

			// Render
			rendered, err := p.renderer.Render(ctx, *notif.TemplateID, notif.Data, locale)
			if err != nil {
				// Rendering errors are usually permanent (bad template syntax or missing key)
				return p.updateState(ctx, txRepo, notif, notification.StatusFailed, nil, &notification.SendResult{Error: "Template render error: " + err.Error()})
			}
			notif.Subject = rendered.Subject
			notif.Body = rendered.Body
		}

		// 5. Update status to SENDING
		if err := p.updateState(ctx, txRepo, notif, notification.StatusSending, nil, nil); err != nil {
			return err
		}

		// 6. Dispatch to Specific Provider
		var sendResult *notification.SendResult
		var sendErr error

		switch notif.Channel {
		case notification.ChannelEmail:
			sendResult, sendErr = p.processEmail(ctx, notif)
		case notification.ChannelSMS:
			sendResult, sendErr = p.processSMS(ctx, notif)
		case notification.ChannelInApp:
			sendResult, sendErr = p.processInApp(ctx, notif)
		case notification.ChannelWebhook:
			sendResult, sendErr = p.processWebhook(ctx, notif)
		case notification.ChannelPush:
			sendResult, sendErr = p.processPush(ctx, notif)
		default:
			sendErr = queue.NewNonRetryableError(fmt.Errorf("unsupported channel: %s", notif.Channel))
		}

		// 7. Handle Outcome
		if sendErr != nil {
			logger.Error("Provider send failed", sendErr)
			if queue.IsNonRetryable(sendErr) {
				return p.updateState(ctx, txRepo, notif, notification.StatusFailed, nil, &notification.SendResult{Error: sendErr.Error()})
			}
			return sendErr
		}

		return p.updateState(ctx, txRepo, notif, notification.StatusSent, sendResult, nil)
	})
}

// --- Channel Processors ---

func (p *processorImpl) processEmail(ctx context.Context, notif *notification.Notification) (*notification.SendResult, error) {
	providerConfig, err := p.repo.FindProviderForDispatch(ctx, notification.ChannelEmail)
	if err != nil {
		if err == notification.ErrNoProviderConfigured {
			return nil, queue.NewNonRetryableError(err)
		}
		return nil, err
	}

	creds, err := p.decryptCreds(ctx, providerConfig)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	provider, err := p.factory.BuildEmailProvider(core.ProviderType(providerConfig.Type), creds)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	subject := "Notification"
	if notif.Subject != nil {
		subject = *notif.Subject
	}

	sendReq := notification.SendRequest{
		To:          []notification.SendRecipient{{Email: notif.ToAddress}},
		From:        notification.SendRecipient{Email: p.defaultFromEmail, Name: "NGO Platform"},
		Subject:     subject,
		HTMLBody:    notif.Body,
		ReferenceID: notif.ID.String(),
		Tags:        []string{notif.Category},
	}

	return provider.Send(ctx, sendReq)
}

func (p *processorImpl) processSMS(ctx context.Context, notif *notification.Notification) (*notification.SendResult, error) {
	providerConfig, err := p.repo.FindProviderForDispatch(ctx, notification.ChannelSMS)
	if err != nil {
		if err == notification.ErrNoProviderConfigured {
			return nil, queue.NewNonRetryableError(err)
		}
		return nil, err
	}

	creds, err := p.decryptCreds(ctx, providerConfig)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	provider, err := p.factory.BuildSMSProvider(core.ProviderType(providerConfig.Type), creds)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	sendReq := notification.SendSMSRequest{
		To:          notif.ToAddress.String(),
		Body:        notif.Body,
		ReferenceID: notif.ID.String(),
	}

	smsRes, err := provider.Send(ctx, sendReq)
	if err != nil {
		return nil, err
	}

	return &notification.SendResult{
		ProviderID: smsRes.ProviderID,
	}, nil
}

func (p *processorImpl) processPush(ctx context.Context, notif *notification.Notification) (*notification.SendResult, error) {
	providerConfig, err := p.repo.FindProviderForDispatch(ctx, notification.ChannelPush)
	if err != nil {
		if err == notification.ErrNoProviderConfigured {
			return nil, queue.NewNonRetryableError(err)
		}
		return nil, err
	}

	creds, err := p.decryptCreds(ctx, providerConfig)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	provider, err := p.factory.BuildPushProvider(core.ProviderType(providerConfig.Type), creds)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	// For push, the ToAddress is likely the DeviceToken
	sendReq := notification.SendPushRequest{
		DeviceToken: notif.ToAddress.String(),
		Title:       *notif.Subject,
		Body:        notif.Body,
		Data:        notif.Data,
		ReferenceID: notif.ID.String(),
	}

	res, err := provider.Send(ctx, sendReq)
	if err != nil {
		return nil, err
	}

	return &notification.SendResult{
		ProviderID: res.ProviderID,
	}, nil
}

func (p *processorImpl) processInApp(ctx context.Context, notif *notification.Notification) (*notification.SendResult, error) {
	// 1. The record is already in the DB.
	// 2. Publish event to Realtime Service.

	payload := map[string]interface{}{
		"id":         notif.ID,
		"subject":    notif.Subject,
		"body":       notif.Body,
		"category":   notif.Category,
		"created_at": notif.CreatedAt,
	}

	evt, err := events.NewEvent("realtime.notification", notif.RecipientID, "system", payload)
	if err == nil {
		if err := p.eventBus.Publish(ctx, evt); err != nil {
			p.logger.Warn("Failed to publish realtime event for in-app notification", log.ErrorAttr(err))
		}
	} else {
		p.logger.Warn("Failed to create realtime event for in-app notification", log.ErrorAttr(err))
	}

	now := time.Now().UTC()
	return &notification.SendResult{
		ProviderID:  "db-inbox",
		DeliveredAt: &now,
	}, nil
}

func (p *processorImpl) processWebhook(ctx context.Context, notif *notification.Notification) (*notification.SendResult, error) {
	webhookPayload := map[string]interface{}{
		"id":        notif.ID,
		"category":  notif.Category,
		"timestamp": time.Now().UTC(),
		"data":      notif.Data,
		"message":   notif.Body,
	}

	// Construct the job payload expected by WebhookWorker (queue.JobWebhookSend)
	jobPayload := map[string]interface{}{
		"url":     notif.ToAddress.String(),
		"job_id":  notif.ID.String(),
		"type":    "notification.webhook",
		"payload": webhookPayload,
	}

	jobBytes, _ := json.Marshal(jobPayload)

	// Use the dispatcher to enqueue a job for the WebhookWorker
	_, err := p.dispatcher.Dispatch(ctx, queue.JobWebhookSend, jobBytes,
		queue.WithQueue("webhooks"),
		queue.WithPriority(5),
	)

	if err != nil {
		return nil, err
	}

	return &notification.SendResult{
		ProviderID: "internal-queue",
	}, nil
}

// --- Helpers ---

func (p *processorImpl) decryptCreds(ctx context.Context, provider *notification.NotificationProvider) (map[string]string, error) {
	decrypted, err := p.crypto.Decrypt(ctx, provider.Config.EncryptedData, provider.Config.EncryptionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt provider credentials: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal(decrypted, &creds); err != nil {
		return nil, fmt.Errorf("invalid provider credentials format: %w", err)
	}
	return creds, nil
}

func (p *processorImpl) updateState(
	ctx context.Context,
	repo notification.Repository,
	notif *notification.Notification,
	newStatus notification.NotificationStatus,
	result *notification.SendResult,
	failResult *notification.SendResult,
) error {
	now := time.Now().UTC()
	notif.Status = newStatus
	notif.UpdatedAt = now
	notif.Attempts++

	if result != nil {
		if result.ProviderID != "" {
			notif.ProviderID = &result.ProviderID
		}
		notif.SentAt = &now
		if result.DeliveredAt != nil {
			notif.Status = notification.StatusDelivered
			notif.DeliveredAt = result.DeliveredAt
		}
	}

	if failResult != nil {
		notif.LastError = &failResult.Error
	}

	if err := repo.UpdateNotification(ctx, notif); err != nil {
		return err
	}

	// Audit Log (Internal Event Table)
	eventMsg := fmt.Sprintf("Status changed to %s", newStatus)
	if failResult != nil {
		eventMsg = fmt.Sprintf("Failed: %s", failResult.Error)
	}

	logEvt := &notification.NotificationEvent{
		ID:             uuid.New(),
		NotificationID: notif.ID,
		Source:         notification.SourceInternal,
		EventType:      string(newStatus),
		Message:        eventMsg,
		CreatedAt:      now,
	}
	if err := repo.CreateEvent(ctx, logEvt); err != nil {
		return err
	}

	// --- NEW: Publish Domain Event for downstream consumers (Realtime/Analytics) ---
	if newStatus == notification.StatusSent || newStatus == notification.StatusDelivered {
		// Use subject if available, otherwise safe fallback
		subject := "Notification"
		if notif.Subject != nil {
			subject = *notif.Subject
		}

		payload := notification.NotificationSentPayload{
			NotificationID: notif.ID,
			RecipientID:    notif.RecipientID,
			Channel:        string(notif.Channel),
			Category:       notif.Category,
			SentAt:         now,
			Subject:        subject,
		}

		// We use 'system' as actor because this is an automated background process result
		evt, err := events.NewEvent(core.EventNotificationSent, uuid.Nil, "system", payload)
		if err == nil {
			// Fire and forget
			_ = p.eventBus.Publish(ctx, evt)
		} else {
			p.logger.Error("failed to create domain event for sent notification", err)
		}
	}

	return nil
}
