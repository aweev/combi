package queue_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	pqueue "github.com/combi/pkg/queue"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBroker is a test helper that initializes a real Redis container
// and a RedisStreamBroker instance for isolated testing.
func setupBroker(t *testing.T) (*pqueue.RedisStreamBroker, *redis.Client, func()) {
	t.Helper()

	client, teardown := testutil.NewTestRedis(t)

	broker := pqueue.NewRedisStreamBroker(
		client,
		&log.NoOpLogger{},
		"test-group",
		pqueue.RedisConsumerName("test-consumer-1"),
	)

	return broker, client, teardown
}

func TestRedisStreamBroker_Integration_Enqueue(t *testing.T) {
	broker, client, teardown := setupBroker(t)
	defer teardown()

	ctx := context.Background()
	jobID := uuid.New()
	queueName := "email-notifications"
	priority := 3
	streamKeyPattern := fmt.Sprintf("ngo:queue:{%s}:p%d", queueName, priority)

	// --- Act ---
	err := broker.Enqueue(ctx, jobID, queueName, priority)
	require.NoError(t, err)

	// --- Assert ---
	length, err := client.XLen(ctx, streamKeyPattern).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), length)

	messages, err := client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKeyPattern, "0-0"},
		Count:   1,
	}).Result()
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Messages, 1)

	payload := messages[0].Messages[0].Values
	assert.Equal(t, jobID.String(), payload["job_id"])
}

func TestRedisStreamBroker_Integration_DequeueAndAcknowledge(t *testing.T) {
	broker, client, teardown := setupBroker(t)
	defer teardown()

	ctx := context.Background()
	jobID := uuid.New()
	queueName := "default"
	priority := 5
	streamKey := fmt.Sprintf("ngo:queue:{%s}:p%d", queueName, priority)
	groupName := "test-group"

	err := broker.Enqueue(ctx, jobID, queueName, priority)
	require.NoError(t, err)

	dequeuedMsg, err := broker.Dequeue(ctx, []string{queueName}, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, dequeuedMsg)

	assert.Equal(t, jobID, dequeuedMsg.JobID)
	assert.NotEmpty(t, dequeuedMsg.Handle)
	assert.Equal(t, streamKey, dequeuedMsg.Source)

	pendingResult, err := client.XPending(ctx, streamKey, groupName).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), pendingResult.Count, "message should be pending after dequeue")

	err = broker.Acknowledge(ctx, dequeuedMsg)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	pendingResult, err = client.XPending(ctx, streamKey, groupName).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), pendingResult.Count, "message should be gone from PEL after acknowledge")
}

func TestRedisStreamBroker_Integration_ClaimStalledMessages(t *testing.T) {
	broker, client, teardown := setupBroker(t)
	defer teardown()

	ctx := context.Background()
	jobID := uuid.New()
	queueName := "processing"
	streamKey := fmt.Sprintf("ngo:queue:{%s}:p5", queueName)
	groupName := "test-group"
	consumerName := "test-consumer-1"
	reaperConsumerID := "reaper-consumer"

	err := client.XGroupCreateMkStream(ctx, streamKey, groupName, "0-0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		require.NoError(t, err)
	}

	msgID, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{"job_id": jobID.String()},
	}).Result()
	require.NoError(t, err)

	_, err = client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamKey, ">"},
		Count:    1,
	}).Result()
	require.NoError(t, err)

	minIdleTime := 100 * time.Millisecond
	time.Sleep(minIdleTime + 50*time.Millisecond)

	stalledMessages, err := broker.ClaimStalledMessages(ctx, streamKey, minIdleTime, reaperConsumerID)
	require.NoError(t, err)

	require.Len(t, stalledMessages, 1, "should have claimed one stalled message")
	stalled := stalledMessages[0]
	assert.Equal(t, streamKey, stalled.StreamKey)
	assert.Equal(t, msgID, stalled.MessageHandle)
	assert.True(t, stalled.IdleTime >= minIdleTime)
	assert.Equal(t, jobID.String(), stalled.Payload["job_id"])

	pendingList, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  groupName,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pendingList, 1)
	assert.Equal(t, reaperConsumerID, pendingList[0].Consumer, "owner of the stalled message should now be the reaper")
}

func TestRedisStreamBroker_Integration_DLQ(t *testing.T) {
	broker, client, teardown := setupBroker(t)
	defer teardown()

	ctx := context.Background()
	jobID := uuid.New()
	msg := &queue.Message{
		JobID:  jobID,
		Handle: "123-0",
		Source: "ngo:queue:{source-queue}:p5",
	}
	payload := json.RawMessage(`{"failed": true}`)
	dlqKey := "ngo:dlq"

	// --- Act: Move to DLQ ---
	err := broker.MoveToDLQ(ctx, msg, payload)
	require.NoError(t, err)

	// --- Assert: Verify DLQ content ---
	dlqMessages, err := client.XRange(ctx, dlqKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, dlqMessages, 1)

	dlqMsg := dlqMessages[0]
	assert.Equal(t, msg.Source, dlqMsg.Values["_original_stream"])
	assert.Equal(t, msg.Handle, dlqMsg.Values["_original_message_id"])
	// CORRECTED: go-redis marshals a boolean `true` into the string "1".
	assert.Equal(t, "1", dlqMsg.Values["failed"])

	// --- List and Get ---
	listResp, nextCursor, err := broker.ListDLQMessages(ctx, 10, "")
	require.NoError(t, err)
	require.Len(t, listResp, 1)
	assert.Empty(t, nextCursor)
	assert.Equal(t, dlqMsg.ID, listResp[0].MessageID)

	getResp, err := broker.GetDLQMessage(ctx, dlqMsg.ID)
	require.NoError(t, err)
	assert.Equal(t, dlqMsg.ID, getResp.MessageID)

	// --- Requeue ---
	err = broker.RequeueDLQMessage(ctx, dlqMsg.ID)
	require.NoError(t, err)

	// Verify it's back in the original stream
	requeuedMsgs, err := client.XRange(ctx, msg.Source, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, requeuedMsgs, 1)
	// CORRECTED: The same type conversion applies here.
	assert.Equal(t, "1", requeuedMsgs[0].Values["failed"])

	// Verify it's gone from the DLQ
	dlqLen, err := client.XLen(ctx, dlqKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), dlqLen)

	// --- Delete ---
	// Acknowledge the requeued message to clean up the stream before adding to it again
	_ = client.XGroupCreateMkStream(ctx, msg.Source, "test-group", "0-0")
	_ = client.XAck(ctx, msg.Source, "test-group", requeuedMsgs[0].ID)

	// Move it back to the DLQ to test deletion
	err = broker.MoveToDLQ(ctx, msg, payload)
	require.NoError(t, err)
	newDlqMsgs, _ := client.XRange(ctx, dlqKey, "-", "+").Result()
	require.Len(t, newDlqMsgs, 1)
	newDlqID := newDlqMsgs[0].ID

	err = broker.DeleteDLQMessage(ctx, newDlqID)
	require.NoError(t, err)

	// Verify it's gone from the DLQ again
	dlqLen, err = client.XLen(ctx, dlqKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), dlqLen)
}
