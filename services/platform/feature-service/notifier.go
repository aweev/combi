// internal/services/feature/redis_notifier.go
package feature

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/log"
)

type redisNotifier struct {
	client  *redis.Client
	logger  log.Logger
	channel string
}

// Statically verify interface implementation.
var _ feature.Notifier = (*redisNotifier)(nil)

func NewRedisNotifier(client *redis.Client, logger log.Logger) *redisNotifier {
	return &redisNotifier{
		client:  client,
		channel: "config_updates",
		logger:  logger.With(log.String("component", "config_redis_notifier")),
	}
}

// --- Publish Methods ---

func (n *redisNotifier) NotifyFlagUpdated(ctx context.Context, flagKey string) error {
	payload := fmt.Sprintf("flag:%s", flagKey)
	return n.publish(ctx, payload)
}

func (n *redisNotifier) NotifyBrandUpdated(ctx context.Context, organizationID *uuid.UUID) error {
	var orgIDStr string
	if organizationID != nil {
		orgIDStr = organizationID.String()
	} else {
		orgIDStr = "default"
	}
	payload := fmt.Sprintf("brand:%s", orgIDStr)
	return n.publish(ctx, payload)
}

func (n *redisNotifier) NotifySettingsUpdated(ctx context.Context, organizationID uuid.UUID) error {
	payload := fmt.Sprintf("settings:%s", organizationID.String())
	return n.publish(ctx, payload)
}

func (n *redisNotifier) publish(ctx context.Context, payload string) error {
	if err := n.client.Publish(ctx, n.channel, payload).Err(); err != nil {
		n.logger.Error("CRITICAL: Failed to publish config update notification", err, log.String("payload", payload))
		return err
	}
	n.logger.Info("Published config update notification", log.String("payload", payload))
	return nil
}

// --- Subscribe Method ---

// Subscribe sets up a listener for config update notifications.
// FIXED: The callback function signature now matches func(key string).
func (n *redisNotifier) Subscribe(ctx context.Context, flagRefreshFunc func(flagKey string)) {
	pubsub := n.client.Subscribe(ctx, n.channel)

	// Note: We don't defer pubsub.Close() here because this function blocks until ctx is done,
	// but if it spawns a goroutine, we need to handle cleanup.
	// In this implementation, it blocks.

	// Wait for confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		n.logger.Error("Failed to subscribe to config update channel, real-time updates disabled", err)
		return
	}
	defer pubsub.Close()

	ch := pubsub.Channel()
	n.logger.Info("Subscribed to config update channel", log.String("channel", n.channel))

	for {
		select {
		case <-ctx.Done():
			n.logger.Info("Unsubscribing from config update channel.")
			return
		case msg := <-ch:
			if strings.HasPrefix(msg.Payload, "flag:") {
				flagKey := strings.TrimPrefix(msg.Payload, "flag:")
				n.logger.Info("Received flag update notification", log.String("flag_key", flagKey))
				// Execute callback. No context passed because it's a background trigger.
				flagRefreshFunc(flagKey)
			}
		}
	}
}
