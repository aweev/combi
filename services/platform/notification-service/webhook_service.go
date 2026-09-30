package notification

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
)

// ParserRegistry holds instances of webhook parsers for different providers.
type ParserRegistry struct {
	parsers map[string]notification.WebhookParser
}

func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{parsers: make(map[string]notification.WebhookParser)}
}

func (r *ParserRegistry) Register(providerName string, parser notification.WebhookParser) {
	r.parsers[providerName] = parser
}

func (r *ParserRegistry) Get(providerName string) (notification.WebhookParser, error) {
	parser, ok := r.parsers[providerName]
	if !ok {
		return nil, fmt.Errorf("no webhook parser registered for provider: %s", providerName)
	}
	return parser, nil
}

// WebhookService handles the ingestion and processing of status update webhooks.
type WebhookService struct {
	repo    notification.Repository
	parsers *ParserRegistry
	logger  log.Logger
}

func NewWebhookService(repo notification.Repository, parsers *ParserRegistry, logger log.Logger) *WebhookService {
	return &WebhookService{
		repo:    repo,
		parsers: parsers,
		logger:  logger.With(log.String("service", "notification_webhook")),
	}
}

// Process finds the correct parser, validates/parses the webhook, and updates notification statuses.
func (s *WebhookService) Process(ctx context.Context, providerName string, r *http.Request) error {
	parser, err := s.parsers.Get(providerName)
	if err != nil {
		s.logger.Warn("Received webhook for unregistered provider", log.String("provider", providerName))
		return nil // Return 200 OK to stop retries from the provider.
	}

	events, err := parser.Parse(r)
	if err != nil {
		s.logger.Error("Failed to parse incoming webhook", err, log.String("provider", providerName))
		return fmt.Errorf("%w: failed to parse webhook: %v", notification.ErrInvalidInput, err)
	}

	s.logger.Info("Successfully parsed webhook events", log.String("provider", providerName), log.Int("event_count", len(events)))

	for _, event := range events {
		if err := s.processSingleEvent(ctx, providerName, event); err != nil {
			s.logger.Error("Failed to process provider event", err,
				log.String("provider", providerName),
				log.String("provider_id", event.ProviderMessageID),
			)
		}
	}
	return nil
}

func (s *WebhookService) processSingleEvent(ctx context.Context, providerName string, event notification.ProviderEvent) error {
	return s.repo.WithTx(ctx, func(txRepo notification.Repository) error {
		n, err := txRepo.GetNotificationByProviderID(ctx, providerName, event.ProviderMessageID)
		if err != nil {
			if err == notification.ErrNotificationNotFound {
				s.logger.Warn("Webhook event received for unknown notification",
					log.String("provider", providerName),
					log.String("provider_id", event.ProviderMessageID),
				)
				return nil // Gracefully handle events for notifications from other environments.
			}
			return fmt.Errorf("failed to get notification by provider ID: %w", err)
		}

		logger := s.logger.With(log.String("notification_id", n.ID.String()))
		newStatus := mapProviderEventToStatus(event.EventType)

		newEvent := &notification.NotificationEvent{
			ID:             uuid.New(),
			NotificationID: n.ID,
			Source:         notification.SourceProvider,
			EventType:      event.EventType,
			Message:        fmt.Sprintf("Webhook event '%s' received from provider.", event.EventType),
			Details:        event.Details,
			CreatedAt:      event.Timestamp,
		}
		if err := txRepo.CreateEvent(ctx, newEvent); err != nil {
			return fmt.Errorf("failed to create provider event log: %w", err)
		}

		if shouldUpdateStatus(n.Status, newStatus) {
			logger.Info("Updating notification status from webhook",
				log.String("from_status", string(n.Status)),
				log.String("to_status", string(newStatus)),
			)
			n.Status = newStatus
			if newStatus == notification.StatusDelivered {
				n.DeliveredAt = &event.Timestamp
			}
			if newStatus == notification.StatusRead {
				n.ReadAt = &event.Timestamp
			}
			if err := txRepo.UpdateNotification(ctx, n); err != nil {
				return fmt.Errorf("failed to update notification from webhook: %w", err)
			}
		}
		return nil
	})
}

func mapProviderEventToStatus(eventType string) notification.NotificationStatus {
	switch eventType {
	case "delivered", "delivered_to_inbox":
		return notification.StatusDelivered
	case "bounced", "hard_bounce":
		return notification.StatusBounced
	case "failed", "rejected", "error":
		return notification.StatusFailed
	case "read", "opened", "clicked":
		return notification.StatusRead
	default:
		return ""
	}
}

func shouldUpdateStatus(current, new notification.NotificationStatus) bool {
	if new == "" || current == new {
		return false
	}
	statusOrder := map[notification.NotificationStatus]int{
		notification.StatusPending:   1,
		notification.StatusQueued:    2,
		notification.StatusSending:   3,
		notification.StatusSent:      4,
		notification.StatusDelivered: 5,
		notification.StatusRead:      6,
		notification.StatusFailed:    98,
		notification.StatusBounced:   99,
	}
	currentOrder, ok1 := statusOrder[current]
	newOrder, ok2 := statusOrder[new]
	if !ok1 || !ok2 {
		return true
	}
	return newOrder > currentOrder
}
