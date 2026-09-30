package realtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/log"
	rt_service "github.com/combi/realtime"
	"github.com/combi/pkg/testutil"
)

func TestRedisBroker_Integration(t *testing.T) {
	// 1. Spin up Redis Container
	client, teardown := testutil.NewTestRedis(t)
	defer teardown()

	broker := rt_service.NewRedisBroker(client, &log.NoOpLogger{})
	defer broker.Close()

	ctx := context.Background()
	channel := realtime.ChannelID("test:integration")

	t.Run("Publish and Subscribe", func(t *testing.T) {
		// 1. Subscribe
		msgChan, err := broker.Subscribe(ctx, channel)
		require.NoError(t, err)

		// Wait briefly for subscription to propagate in Redis
		time.Sleep(100 * time.Millisecond)

		// 2. Publish
		msg := &realtime.Message{
			Event:     "ping",
			Payload:   []byte(`{"hello":"world"}`),
			Timestamp: time.Now().UTC(),
			Channel:   channel,
		}
		err = broker.Publish(ctx, channel, msg)
		require.NoError(t, err)

		// 3. Receive
		select {
		case received := <-msgChan:
			assert.Equal(t, msg.Event, received.Event)
			assert.Equal(t, channel, received.Channel)
			assert.JSONEq(t, string(msg.Payload), string(received.Payload))
		case <-time.After(2 * time.Second):
			t.Fatal("Timed out waiting for message")
		}
	})

	t.Run("Unsubscribe stops messages", func(t *testing.T) {
		// 1. Subscribe
		msgChan, err := broker.Subscribe(ctx, "channel:unsub")
		require.NoError(t, err)

		// 2. Unsubscribe
		err = broker.Unsubscribe(ctx, "channel:unsub")
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)

		// 3. Publish (should not be received)
		err = broker.Publish(ctx, "channel:unsub", &realtime.Message{Event: "missed"})
		require.NoError(t, err)

		select {
		case <-msgChan:
			t.Fatal("Should not have received message after unsubscribe")
		case <-time.After(200 * time.Millisecond):
			// Success (Timeout expected)
		}
	})
}
