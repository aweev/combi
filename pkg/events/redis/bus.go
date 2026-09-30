package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// RedisStreamBus implements events.Bus using Redis Streams.
type RedisStreamBus struct {
	client    *redis.Client
	idemStore cache.IdempotencyStore
	config    config.EventBusConfig
	logger    log.Logger
	monitor   monitoring.Recorder
	tracer    trace.Tracer

	serviceName string // Used as Consumer Group
	consumerID  string // Unique ID for this instance

	handlers map[string][]events.EventHandler
	mu       sync.RWMutex

	wg         sync.WaitGroup
	ctx        context.Context    // Lifecycle context
	cancelFunc context.CancelFunc // Function to stop consumers
	running    bool
}

func NewRedisStreamBus(
	client *redis.Client,
	idemStore cache.IdempotencyStore,
	logger log.Logger,
	monitor monitoring.Recorder,
	serviceName string,
	cfg config.EventBusConfig,
) *RedisStreamBus {
	consumerID := fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano())

	// Create a cancellable context for the bus lifecycle
	ctx, cancel := context.WithCancel(context.Background())

	return &RedisStreamBus{
		client:      client,
		idemStore:   idemStore,
		monitor:     monitor,
		logger:      logger.With(log.String("component", "eventbus"), log.String("group", serviceName)),
		config:      cfg,
		tracer:      otel.Tracer("ngo.eventbus"),
		serviceName: serviceName,
		consumerID:  consumerID,
		handlers:    make(map[string][]events.EventHandler),
		ctx:         ctx,
		cancelFunc:  cancel,
	}
}

func (b *RedisStreamBus) Health(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

func (b *RedisStreamBus) Subscribe(topic string, handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 1. Add handler to list
	b.handlers[topic] = append(b.handlers[topic], handler)

	// 2. Dynamic Subscription
	if b.running {
		if len(b.handlers[topic]) == 1 {
			b.logger.Info("Initializing dynamic subscription", log.String("topic", topic))

			if err := b.ensureGroup(context.Background(), topic); err != nil {
				b.logger.Error("Failed to create dynamic group", err, log.String("topic", topic))
			}

			b.wg.Add(1)
			go b.consumeLoop(b.ctx, topic)
		}
	}
}

func (b *RedisStreamBus) Publish(ctx context.Context, event events.DomainEvent) error {
	topic := string(event.Action)
	fullTopic := b.config.ChannelPrefix + topic

	// 1. Trace
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}
	ctx, span := b.tracer.Start(ctx, "publish "+topic, trace.WithAttributes(
		attribute.String("messaging.system", "redis"),
		attribute.String("messaging.destination.name", fullTopic),
		attribute.String("messaging.operation", "publish"),
		attribute.String("event.id", event.ID.String()),
	))
	defer span.End()

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(event.Metadata))

	// 2. Serialize
	eventBytes, err := json.Marshal(event)
	if err != nil {
		b.recordPublishMetric(topic, "error_serialize")
		return fmt.Errorf("marshal failed: %w", err)
	}

	// 3. XAdd
	err = b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fullTopic,
		Values: map[string]interface{}{"data": eventBytes},
		MaxLen: 100000,
		Approx: true,
	}).Err()

	if err != nil {
		span.RecordError(err)
		b.recordPublishMetric(topic, "error_redis")
		return err
	}

	b.recordPublishMetric(topic, "success")
	return nil
}

func (b *RedisStreamBus) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}

	if len(topics) == 0 {
		b.logger.Info("No topics subscribed; event bus consumer will not start")
		b.running = true
		return nil
	}

	for _, topic := range topics {
		if err := b.ensureGroup(ctx, topic); err != nil {
			return err
		}
	}

	for _, topic := range topics {
		b.wg.Add(1)
		go b.consumeLoop(b.ctx, topic)
	}

	// Start Reaper to handle stalled messages and retries
	b.wg.Add(1)
	go b.reaperLoop(b.ctx)

	b.running = true
	b.logger.Info("EventBus started", log.Int("topics", len(topics)), log.String("consumer_id", b.consumerID))
	return nil
}

func (b *RedisStreamBus) Shutdown(ctx context.Context) error {
	b.logger.Info("EventBus shutting down...")
	b.cancelFunc()
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("EventBus shutdown complete")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *RedisStreamBus) ensureGroup(ctx context.Context, topic string) error {
	fullTopic := b.config.ChannelPrefix + topic
	err := b.client.XGroupCreateMkStream(ctx, fullTopic, b.serviceName, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("failed to create group for %s: %w", fullTopic, err)
	}
	return nil
}

func (b *RedisStreamBus) consumeLoop(ctx context.Context, topic string) {
	defer b.wg.Done()
	fullTopic := b.config.ChannelPrefix + topic
	b.logger.Debug("Starting consume loop", log.String("topic", topic))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    b.serviceName,
				Consumer: b.consumerID,
				Streams:  []string{fullTopic, ">"},
				Count:    10,
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if !errors.Is(err, redis.Nil) {
					b.logger.Error("XReadGroup error", err, log.String("topic", topic))
					time.Sleep(1 * time.Second)
				}
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					b.processMessage(ctx, topic, message)
				}
			}
		}
	}
}

func (b *RedisStreamBus) processMessage(ctx context.Context, topic string, redisMsg redis.XMessage) {
	start := time.Now()
	fullTopic := b.config.ChannelPrefix + topic

	// 1. Unmarshal
	dataStr, ok := redisMsg.Values["data"].(string)
	if !ok {
		b.logger.Error("Corrupt event payload", nil, log.String("id", redisMsg.ID))
		b.client.XAck(ctx, fullTopic, b.serviceName, redisMsg.ID)
		return
	}

	var event events.DomainEvent
	if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
		b.moveToDLQ(ctx, fullTopic, redisMsg, "json_unmarshal_error")
		return
	}

	// 2. Idempotency Check
	idempKey := fmt.Sprintf("evt:%s:%s", event.ID.String(), b.serviceName)
	_, _, found, err := b.idemStore.GetResult(ctx, idempKey)
	if err != nil {
		b.logger.Error("Idempotency check failed", err)
		return
	}
	if found {
		b.client.XAck(ctx, fullTopic, b.serviceName, redisMsg.ID)
		return
	}

	// 3. Trace Context
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}
	parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(event.Metadata))

	spanCtx, span := b.tracer.Start(parentCtx, "process "+topic, trace.WithAttributes(
		attribute.String("messaging.system", "redis"),
		attribute.String("messaging.destination.name", fullTopic),
		attribute.String("messaging.consumer.id", b.serviceName),
		attribute.String("messaging.message.id", redisMsg.ID),
	))
	defer span.End()

	// 4. Execution
	b.mu.RLock()
	handlers := b.handlers[topic]
	b.mu.RUnlock()

	b.recordProcessMetric(topic, "received")

	success := true
	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("panic in handler: %v", r)
					b.logger.Error("Handler panic", err, log.String("topic", topic))
					success = false
					span.RecordError(err)
				}
			}()

			if err := h(spanCtx, event); err != nil {
				b.logger.Error("Handler failed", err, log.String("topic", topic))
				success = false
				span.RecordError(err)
			}
		}()
	}

	// 5. Result
	if !success {
		b.recordProcessMetric(topic, "failed")
		// Do not ACK on failure, allow Reaper to handle retry/DLQ
	} else {
		_ = b.idemStore.StoreResult(ctx, idempKey, json.RawMessage(`"ok"`), nil, 24*time.Hour)
		_ = b.client.XAck(ctx, fullTopic, b.serviceName, redisMsg.ID)

		b.recordProcessMetric(topic, "success")
		b.recordDurationMetric(topic, start)
	}
}

func (b *RedisStreamBus) moveToDLQ(ctx context.Context, originalTopic string, msg redis.XMessage, reason string) {
	dlqTopic := originalTopic + b.config.DLQSuffix

	values := make(map[string]interface{})
	for k, v := range msg.Values {
		values[k] = v
	}
	values["error_reason"] = reason
	values["failed_group"] = b.serviceName
	values["failed_at"] = time.Now().Format(time.RFC3339)

	err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqTopic,
		Values: values,
	}).Err()

	if err != nil {
		b.logger.Error("CRITICAL: Failed to write to DLQ", err, log.String("id", msg.ID))
		return
	}
	b.client.XAck(ctx, originalTopic, b.serviceName, msg.ID)
}

// reaperLoop runs the background process to reclaim stalled messages.
func (b *RedisStreamBus) reaperLoop(ctx context.Context) {
	defer b.wg.Done()

	pollInterval := b.config.ReaperPollInterval
	if pollInterval == 0 {
		pollInterval = 1 * time.Minute
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Use a shorter timeout for the reaper cycle to allow graceful shutdown
			reapCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			b.reap(reapCtx)
			cancel()
		}
	}
}

func (b *RedisStreamBus) reap(ctx context.Context) {
	b.mu.RLock()
	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	b.mu.RUnlock()

	for _, topic := range topics {
		if ctx.Err() != nil {
			return
		}

		fullTopic := b.config.ChannelPrefix + topic

		// Find Pending Messages (Delivered but not Acked)
		// that have been idle longer than the grace period.
		pending, err := b.client.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: fullTopic,
			Group:  b.serviceName,
			Start:  "-",
			End:    "+",
			Count:  50,
			Idle:   b.config.ReaperGracePeriod,
		}).Result()

		if err != nil {
			b.logger.Warn("Reaper failed to check pending", log.ErrorAttr(err))
			continue
		}

		for _, msg := range pending {
			// 1. Check Retry Count vs Max Config
			if int(msg.RetryCount) > b.config.MaxRetries {
				b.logger.Warn("Message exceeded max retries, moving to DLQ", log.String("id", msg.ID))

				// Fetch payload to move to DLQ
				rawMsgs, err := b.client.XRange(ctx, fullTopic, msg.ID, msg.ID).Result()
				if err == nil && len(rawMsgs) > 0 {
					b.moveToDLQ(ctx, fullTopic, rawMsgs[0], "max_retries_exceeded")
				} else {
					// Message lost from stream (trimmed)? Just Ack to clear pending state.
					if err := b.client.XAck(ctx, fullTopic, b.serviceName, msg.ID).Err(); err != nil {
						b.logger.Error("Reaper failed to ack missing message", err)
					}
				}
				continue
			}

			// 2. Claim Ownership
			// If message is stuck, claim it for this consumer (b.consumerID)
			claimed, err := b.client.XClaim(ctx, &redis.XClaimArgs{
				Stream:   fullTopic,
				Group:    b.serviceName,
				Consumer: b.consumerID,
				MinIdle:  b.config.ReaperGracePeriod,
				Messages: []string{msg.ID},
			}).Result()

			if err != nil {
				continue
			}

			// 3. Process
			// XClaim returns the messages it successfully claimed.
			for _, xmsg := range claimed {
				b.logger.Info("Reaper reclaimed message", log.String("id", xmsg.ID))
				b.processMessage(ctx, topic, xmsg)
			}
		}
	}
}

func (b *RedisStreamBus) recordProcessMetric(topic, status string) {
	if b.monitor != nil {
		b.monitor.IncrementCounter(monitoring.MetricEventsProcessedTotal, monitoring.MetricLabels{
			monitoring.LabelTopic:  topic,
			monitoring.LabelStatus: status,
		})
	}
}

func (b *RedisStreamBus) recordDurationMetric(topic string, start time.Time) {
	if b.monitor != nil {
		b.monitor.RecordHistogram(monitoring.MetricJobDurationSeconds, time.Since(start).Seconds(), monitoring.MetricLabels{
			monitoring.LabelQueue:   "events",
			monitoring.LabelJobType: topic,
		})
	}
}

func (b *RedisStreamBus) recordPublishMetric(topic, status string) {
	if b.monitor != nil {
		b.monitor.IncrementCounter(monitoring.MetricEventsPublishedTotal, monitoring.MetricLabels{
			monitoring.LabelTopic:  topic,
			monitoring.LabelStatus: status,
		})
	}
}
