package nats

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
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// NatsJetStreamBus implements events.Bus using NATS JetStream.
type NatsJetStreamBus struct {
	nc        *nats.Conn
	js        jetstream.JetStream
	idemStore cache.IdempotencyStore
	config    config.EventBusConfig
	logger    log.Logger
	monitor   monitoring.Recorder
	tracer    trace.Tracer

	serviceName string
	handlers    map[string][]events.EventHandler
	mu          sync.RWMutex

	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
	running    bool
}

func NewNatsJetStreamBus(
	nc *nats.Conn,
	idemStore cache.IdempotencyStore,
	logger log.Logger,
	monitor monitoring.Recorder,
	serviceName string,
	cfg config.EventBusConfig,
) (*NatsJetStreamBus, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &NatsJetStreamBus{
		nc:          nc,
		js:          js,
		idemStore:   idemStore,
		logger:      logger.With(log.String("component", "eventbus-nats"), log.String("group", serviceName)),
		config:      cfg,
		monitor:     monitor,
		tracer:      otel.Tracer("ngo.eventbus.nats"),
		serviceName: serviceName,
		handlers:    make(map[string][]events.EventHandler),
		ctx:         ctx,
		cancelFunc:  cancel,
	}, nil
}

func (b *NatsJetStreamBus) Health(ctx context.Context) error {
	if b.nc.Status() != nats.CONNECTED {
		return fmt.Errorf("nats is not connected, current status: %s", b.nc.Status().String())
	}
	return nil
}

func (b *NatsJetStreamBus) Subscribe(topic string, handler events.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[topic] = append(b.handlers[topic], handler)

	if b.running {
		if len(b.handlers[topic]) == 1 {
			b.logger.Info("Initializing dynamic NATS JetStream subscription", log.String("topic", topic))
			b.wg.Add(1)
			go b.consumeLoop(b.ctx, topic)
		}
	}
}

func (b *NatsJetStreamBus) Publish(ctx context.Context, event events.DomainEvent) error {
	topic := string(event.Action)
	subject := b.config.ChannelPrefix + topic

	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}
	ctx, span := b.tracer.Start(ctx, "publish_nats "+topic, trace.WithAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination.name", subject),
		attribute.String("messaging.operation", "publish"),
		attribute.String("event.id", event.ID.String()),
	))
	defer span.End()

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(event.Metadata))

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Dynamic stream creation for this subject
	streamName := strings.ReplaceAll(b.config.ChannelPrefix, ".", "_") + "stream"
	_, err = b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{b.config.ChannelPrefix + "*"},
	})
	if err != nil {
		return fmt.Errorf("stream check failed: %w", err)
	}

	_, err = b.js.Publish(ctx, subject, eventBytes)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("nats publish failed: %w", err)
	}

	return nil
}

func (b *NatsJetStreamBus) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	b.running = true
	b.logger.Info("NATS JetStream event bus starting...", log.Int("subscriptions", len(b.handlers)))

	for topic := range b.handlers {
		b.wg.Add(1)
		go b.consumeLoop(b.ctx, topic)
	}

	return nil
}

func (b *NatsJetStreamBus) consumeLoop(ctx context.Context, topic string) {
	defer b.wg.Done()

	subject := b.config.ChannelPrefix + topic
	streamName := strings.ReplaceAll(b.config.ChannelPrefix, ".", "_") + "stream"

	// Ensure stream is registered
	_, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{b.config.ChannelPrefix + "*"},
	})
	if err != nil {
		b.logger.Error("Failed to verify/create stream during consumption setup", err, log.String("topic", topic))
		return
	}

	// Create/Get consumer matching the serviceName
	consumerName := fmt.Sprintf("%s-%s", b.serviceName, strings.ReplaceAll(topic, ".", "-"))
	consumer, err := b.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		b.logger.Error("Failed to create NATS consumer", err, log.String("topic", topic))
		return
	}

	iter, err := consumer.Consume(func(msg jetstream.Msg) {
		b.handleMessage(msg, topic)
	})
	if err != nil {
		b.logger.Error("Failed to begin consuming", err, log.String("topic", topic))
		return
	}
	defer iter.Stop()

	<-ctx.Done()
	b.logger.Info("Stopped NATS JetStream consume loop", log.String("topic", topic))
}

func (b *NatsJetStreamBus) handleMessage(msg jetstream.Msg, topic string) {
	// Track whether every handler succeeded. With AckExplicitPolicy the
	// message is only acked on success; on failure it is Nak'd so JetStream
	// redelivers it (retry semantics match the Redis stream bus).
	success := true

	// A malformed payload can never succeed on redelivery — ack and drop it.
	var event events.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		b.logger.Error("Failed to unmarshal NATS event payload", err)
		_ = msg.Ack()
		return
	}

	// Trace context propagation
	carrier := propagation.MapCarrier(event.Metadata)
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	ctx, span := b.tracer.Start(ctx, "consume "+topic, trace.WithAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination.name", msg.Subject()),
		attribute.String("messaging.operation", "process"),
		attribute.String("event.id", event.ID.String()),
	))
	defer span.End()

	// Enforce Idempotency (same pattern as the Redis stream bus).
	idemKey := fmt.Sprintf("evt:%s:%s", event.ID.String(), b.serviceName)
	if b.idemStore != nil {
		_, _, found, err := b.idemStore.GetResult(ctx, idemKey)
		if err != nil {
			b.logger.Error("Idempotency check failed", err, log.String("event_id", event.ID.String()))
			_ = msg.Nak()
			return
		}
		if found {
			b.logger.Warn("Duplicate event detected, ignoring", log.String("event_id", event.ID.String()))
			_ = msg.Ack()
			return
		}
	}

	b.mu.RLock()
	handlers := b.handlers[topic]
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			span.RecordError(err)
			b.logger.Error("Failed running event handler", err, log.String("event_id", event.ID.String()))
			success = false
		}
	}

	if success {
		// Record the processed marker so redeliveries are skipped as duplicates.
		if b.idemStore != nil {
			_ = b.idemStore.StoreResult(ctx, idemKey, json.RawMessage(`"ok"`), nil, 24*time.Hour)
		}
		_ = msg.Ack()
	} else {
		// Do not ack on failure — let JetStream redeliver (retry/DLQ semantics).
		_ = msg.Nak()
	}
}

func (b *NatsJetStreamBus) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}
	b.running = false
	b.mu.Unlock()

	b.logger.Info("Shutting down NATS JetStream event bus...")
	b.cancelFunc()

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("NATS JetStream event bus shutdown cleanly")
		return nil
	case <-ctx.Done():
		return errors.New("timeout waiting for consumer loops to shut down")
	}
}
