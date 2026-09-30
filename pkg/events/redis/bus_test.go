package redis_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/events/redis"
	"github.com/combi/pkg/log"
	prometheus "github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisStreamBus_Integration(t *testing.T) {
	// 1. Setup Infrastructure (Shared Redis container is fine)
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	monitor := prometheus.NewRecorder("test-service")

	// Helper to create a fresh bus for each sub-test
	setupBus := func(t *testing.T, groupID string) (*redis.RedisStreamBus, cache.IdempotencyStore) {
		// Use a unique prefix per test to ensure total isolation
		prefix := "test:" + uuid.New().String() + ":"

		cfg := config.EventBusConfig{
			ChannelPrefix:      prefix,
			DLQSuffix:          ":dlq",
			MaxRetries:         1, // Fail once -> Retry -> Fail Again -> DLQ
			RetryDelay:         10 * time.Millisecond,
			ReaperPollInterval: 50 * time.Millisecond,
			ReaperGracePeriod:  100 * time.Millisecond,
		}

		idemStore := cache.NewRedisIdempotencyStore(rdb, prefix+"idem:", logger)
		bus := redis.NewRedisStreamBus(rdb, idemStore, logger, monitor, groupID, cfg)
		return bus, idemStore
	}

	t.Run("End-to-End Publish and Consume", func(t *testing.T) {
		bus, _ := setupBus(t, "group-e2e")
		defer bus.Shutdown(context.Background())

		var wg sync.WaitGroup
		wg.Add(1)
		var receivedID uuid.UUID

		// 1. Subscribe BEFORE Start
		bus.Subscribe("user.created", func(ctx context.Context, evt events.DomainEvent) error {
			receivedID = evt.ID
			wg.Done()
			return nil
		})

		// 2. Start
		require.NoError(t, bus.Start(context.Background()))

		// 3. Publish
		evt, _ := events.NewEvent("user.created", uuid.New(), "system", map[string]string{"foo": "bar"})
		err := bus.Publish(context.Background(), evt)
		require.NoError(t, err)

		// 4. Wait
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()

		select {
		case <-done:
			assert.Equal(t, evt.ID, receivedID)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for event")
		}
	})

	t.Run("Idempotency Protection", func(t *testing.T) {
		bus, idemStore := setupBus(t, "group-idemp")
		defer bus.Shutdown(context.Background())

		evt, _ := events.NewEvent("user.updated", uuid.New(), "system", nil)

		// Key must match logic in consumer.go: evt:{ID}:{GroupID}
		// Note: The idempotency store prefix is handled inside the store,
		// but the Key passed to it is generated in consumer.go
		key := fmt.Sprintf("evt:%s:%s", evt.ID.String(), "group-idemp")

		// Pre-seed the store with a success result
		err := idemStore.StoreResult(context.Background(), key, []byte(`"processed"`), nil, time.Hour)
		require.NoError(t, err)

		var callCount int
		bus.Subscribe("user.updated", func(ctx context.Context, evt events.DomainEvent) error {
			callCount++
			return nil
		})

		require.NoError(t, bus.Start(context.Background()))

		_ = bus.Publish(context.Background(), evt)

		// Wait enough time for potential processing
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, 0, callCount, "Handler should NOT be called for duplicate event")
	})

	t.Run("DLQ Logic", func(t *testing.T) {
		bus, _ := setupBus(t, "group-dlq")
		defer bus.Shutdown(context.Background())

		failingTopic := "payment.failed"

		// Handler always fails
		bus.Subscribe(failingTopic, func(ctx context.Context, evt events.DomainEvent) error {
			return assert.AnError
		})

		require.NoError(t, bus.Start(context.Background()))

		evt, _ := events.NewEvent(core.EventAction(failingTopic), uuid.New(), "system", nil)
		_ = bus.Publish(context.Background(), evt)

		// We wait 600ms to allow Reaper cycles (100ms grace, 50ms poll)
		// Cycle 1 (T+150ms): Claims (Retry 1 -> 2)
		// Cycle 2 (T+300ms): Claims/Checks (Retry 2 > 1) -> DLQ
		time.Sleep(600 * time.Millisecond)

		keys, err := rdb.Keys(context.Background(), "*:dlq").Result()
		require.NoError(t, err)

		var totalDLQMessages int64
		for _, k := range keys {
			count, _ := rdb.XLen(context.Background(), k).Result()
			totalDLQMessages += count
		}

		assert.GreaterOrEqual(t, totalDLQMessages, int64(1), "Expected at least 1 message in a DLQ")
	})
}
