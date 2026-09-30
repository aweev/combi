// internal/platform/queue/redis_broker.go
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisConsumerGroup string
type RedisConsumerName string

const (
	maxPriority = 10
	minPriority = 0
)

type RedisStreamBroker struct {
	client       *redis.Client
	logger       log.Logger
	groupName    string // e.g., "NGO-workers"
	consumerName string
	keyPrefix    string
}

var _ queue.JobQueue = (*RedisStreamBroker)(nil)

func NewRedisStreamBroker(client *redis.Client, logger log.Logger, groupName RedisConsumerGroup, consumerName RedisConsumerName) *RedisStreamBroker {
	return &RedisStreamBroker{
		client:       client,
		logger:       logger.With(log.String("component", "redis_broker")),
		groupName:    string(groupName),
		consumerName: string(consumerName),
		keyPrefix:    "ngo:queue:",
	}
}

func (b *RedisStreamBroker) Publish(ctx context.Context, streamKey string, values map[string]interface{}) error {
	err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: values,
	}).Err()
	if err != nil {
		b.logger.Error("failed to publish message to stream", err, log.String("stream", streamKey))
		return err
	}
	return nil
}

func (b *RedisStreamBroker) streamKey(queueName string, priority int) string {
	return fmt.Sprintf("%s{%s}:p%d", b.keyPrefix, queueName, priority)
}

func (b *RedisStreamBroker) Enqueue(ctx context.Context, jobID uuid.UUID, queueName string, priority int) error {
	key := b.streamKey(queueName, priority)
	err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		Values: map[string]interface{}{"job_id": jobID.String()},
	}).Err()
	if err != nil {
		b.logger.Error("failed to enqueue job", err, log.String("stream", key), log.String("job_id", jobID.String()))
		return err
	}
	return nil
}

func (b *RedisStreamBroker) Dequeue(ctx context.Context, queues []string, blockTimeout time.Duration) (*queue.Message, error) {
	if len(queues) == 0 {
		return nil, errors.New("dequeue called with no queues")
	}

	streams := b.buildStreamList(ctx, queues)
	if len(streams) == 0 {
		return nil, queue.ErrNoJobAvailable // Nothing to listen to
	}

	// --- STEP 1: Check for pending messages (re-claimed after a crash) ---
	// CORRECTED: The cursors must be "0-0" to check for pending messages, not empty strings.
	pendingStreamsAndCursors := append(streams, make([]string, len(streams))...)
	for i := len(streams); i < len(pendingStreamsAndCursors); i++ {
		pendingStreamsAndCursors[i] = "0-0" // Use the special ID for pending messages.
	}

	// Use a non-blocking read (timeout 0) for pending messages. We don't want to wait if there are none.
	msg, err := b.readFromStreams(ctx, pendingStreamsAndCursors, 0)
	if err == nil {
		// We successfully read a pending message. Process it immediately.
		b.logger.Info("dequeued a pending (re-claimed) job", log.String("job_id", msg.JobID.String()))
		return msg, nil
	}
	// If the error is anything other than "no job available", it's a real problem.
	if !errors.Is(err, queue.ErrNoJobAvailable) {
		return nil, err
	}

	// --- STEP 2: Block and wait for new messages ---
	// If there were no pending messages, now we wait for new ones using the ">" cursor.
	newStreamsAndCursors := append(streams, make([]string, len(streams))...)
	for i := len(streams); i < len(newStreamsAndCursors); i++ {
		newStreamsAndCursors[i] = ">"
	}

	return b.readFromStreams(ctx, newStreamsAndCursors, blockTimeout)
}

func (b *RedisStreamBroker) buildStreamList(ctx context.Context, queues []string) []string {
	streams := make([]string, 0, len(queues)*11)
	for p := 10; p >= 0; p-- { // High to low priority
		for _, q := range queues {
			key := b.streamKey(q, p)
			// Ensure the group exists. It's idempotent, so it's safe to call every time.
			_, _ = b.client.XGroupCreateMkStream(ctx, key, b.groupName, "0-0").Result()
			streams = append(streams, key)
		}
	}
	return streams
}

func (b *RedisStreamBroker) readFromStreams(ctx context.Context, streamsAndCursors []string, blockTimeout time.Duration) (*queue.Message, error) {
	result, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    b.groupName,
		Consumer: b.consumerName,
		Streams:  streamsAndCursors,
		Count:    1,
		Block:    blockTimeout,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, queue.ErrNoJobAvailable
		}
		b.logger.Error("error during XReadGroup", err)
		return nil, err
	}

	if len(result) == 0 || len(result[0].Messages) == 0 {
		return nil, queue.ErrNoJobAvailable
	}

	rawMsg := result[0].Messages[0]
	streamName := result[0].Stream

	jobIDStr, ok := rawMsg.Values["job_id"].(string)
	if !ok {
		_ = b.client.XAck(ctx, streamName, b.groupName, rawMsg.ID).Err() // Acknowledge invalid message
		err := errors.New("critical: job_id missing or not a string in message")
		b.logger.Error(err.Error(), err, log.String("message_id", rawMsg.ID), log.String("stream", streamName))
		return nil, err
	}

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		_ = b.client.XAck(ctx, streamName, b.groupName, rawMsg.ID).Err() // Acknowledge invalid message
		b.logger.Error("critical: failed to parse job_id in message", err, log.String("raw_id", jobIDStr), log.String("message_id", rawMsg.ID))
		return nil, err
	}

	return &queue.Message{
		JobID:  jobID,
		Handle: rawMsg.ID,
		Source: streamName,
	}, nil
}

func (b *RedisStreamBroker) Acknowledge(ctx context.Context, msg *queue.Message) error {
	return b.client.XAck(ctx, msg.Source, b.groupName, msg.Handle).Err()
}

func (b *RedisStreamBroker) ClaimStalledJobs(ctx context.Context, queueName string, minIdleTime time.Duration) (int, error) {
	var totalClaimed int
	for p := maxPriority; p >= minPriority; p-- {
		streamKey := b.streamKey(queueName, p)
		messages, err := b.ClaimStalledMessages(ctx, streamKey, minIdleTime, b.consumerName)
		if err != nil {
			// Ignore "no such key" errors
			if !strings.Contains(err.Error(), "no such key") {
				return totalClaimed, err
			}
		}
		totalClaimed += len(messages)
	}
	return totalClaimed, nil
}

// ClaimStalledJobs finds messages in a consumer group that have been pending for longer than minIdleTime
// and re-claims them for the current consumer to be re-processed.
func (b *RedisStreamBroker) ClaimStalledMessages(ctx context.Context, streamKey string, minIdleTime time.Duration, consumerID string) ([]queue.StalledMessage, error) {
	pendingResult, err := b.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  b.groupName,
		Start:  "-",
		End:    "+",
		Count:  100,
		Idle:   minIdleTime,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check for pending messages: %w", err)
	}

	if len(pendingResult) == 0 {
		return nil, nil
	}

	messageIDs := make([]string, len(pendingResult))
	pendingMap := make(map[string]redis.XPendingExt, len(pendingResult))
	for i, p := range pendingResult {
		messageIDs[i] = p.ID
		pendingMap[p.ID] = p
	}

	claimResult, err := b.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   streamKey,
		Group:    b.groupName,
		Consumer: consumerID,
		MinIdle:  minIdleTime,
		Messages: messageIDs,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to claim messages: %w", err)
	}

	stalledMessages := make([]queue.StalledMessage, len(claimResult))
	for i, claimedMsg := range claimResult {
		pendingInfo := pendingMap[claimedMsg.ID]
		stalledMessages[i] = queue.StalledMessage{
			StreamKey:     streamKey,
			MessageHandle: claimedMsg.ID,
			DeliveryCount: pendingInfo.RetryCount,
			IdleTime:      pendingInfo.Idle,
			Payload:       claimedMsg.Values,
		}
	}

	b.logger.Warn("claimed stalled jobs",
		log.String("stream", streamKey),
		log.Int("count", len(stalledMessages)),
	)

	return stalledMessages, nil
}

// ListQueues scans for all stream keys and extracts the unique queue names.
// It uses SCAN for safe iteration in production environments.
func (b *RedisStreamBroker) ListQueues(ctx context.Context) ([]string, error) {
	// The pattern matches our streamKey format: ngo:queue:{queueName}:p<priority>
	// The part in curly braces `{}` is the hash tag, ensuring all priority streams for a single queue
	// are on the same Redis cluster node. We need to extract just that part.
	pattern := b.keyPrefix + "{*}"
	var cursor uint64
	queueSet := make(map[string]struct{})

	for {
		keys, nextCursor, err := b.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			b.logger.Error("failed to scan for queue keys in redis", err)
			return nil, fmt.Errorf("failed to scan for queue keys: %w", err)
		}

		for _, key := range keys {
			// Extract the queue name from the key.
			// Example key: ngo:queue:{default}:p5
			// 1. Remove prefix: {default}:p5
			trimmed := strings.TrimPrefix(key, b.keyPrefix)
			// 2. Find start and end of hash tag:
			start := strings.Index(trimmed, "{")
			end := strings.Index(trimmed, "}")
			if start != -1 && end != -1 && start < end {
				queueName := trimmed[start+1 : end]
				queueSet[queueName] = struct{}{}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	queues := make([]string, 0, len(queueSet))
	for q := range queueSet {
		queues = append(queues, q)
	}

	return queues, nil
}

func (b *RedisStreamBroker) ListStreamsForQueue(ctx context.Context, queueName string) ([]string, error) {
	// For Redis Streams, this means generating all possible priority stream keys.
	streams := make([]string, 0, maxPriority+1)
	for p := maxPriority; p >= minPriority; p-- {
		streams = append(streams, b.streamKey(queueName, p))
	}
	return streams, nil
}

// dlqKey returns the fixed global key for the Dead-Letter Queue stream.
func (b *RedisStreamBroker) dlqKey() string {
	return "ngo:dlq"
}

// PurgeQueue is a destructive operation that removes all underlying Redis streams
// for a given queue name (across all priorities).
func (b *RedisStreamBroker) PurgeQueue(ctx context.Context, queueName string) (int, error) {
	keysToDelete := make([]string, 0, 11)
	for p := 0; p <= 10; p++ {
		streamKey := b.streamKey(queueName, p)
		keysToDelete = append(keysToDelete, streamKey)
	}

	deletedCount, err := b.client.Del(ctx, keysToDelete...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete stream keys for queue '%s': %w", queueName, err)
	}

	return int(deletedCount), nil
}

func (b *RedisStreamBroker) MoveToDLQ(ctx context.Context, msg *queue.Message, originalPayload json.RawMessage) error {
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(originalPayload, &payloadMap); err != nil {
		// If we can't unmarshal, we'll just store the raw string.
		payloadMap = map[string]interface{}{"payload": string(originalPayload)}
	}

	values := make(map[string]interface{})
	for k, v := range payloadMap {
		values[k] = v
	}
	values["_original_stream"] = msg.Source
	values["_original_message_id"] = msg.Handle
	values["_moved_at"] = time.Now().UTC().Format(time.RFC3339)

	err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.dlqKey(),
		Values: values,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add message to global DLQ: %w", err)
	}

	return b.Acknowledge(ctx, msg)
}

// ListDLQMessages retrieves messages from the global DLQ, supporting cursor-based pagination.
// It uses XREVRANGE to show newest failures first.
func (b *RedisStreamBroker) ListDLQMessages(ctx context.Context, limit int, cursor string) ([]queue.DLQMessage, string, error) {
	start := "+"
	if cursor != "" {
		start = cursor
	}

	// Read one more than limit to determine the next cursor
	results, err := b.client.XRevRangeN(ctx, b.dlqKey(), start, "-", int64(limit+1)).Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to list DLQ messages: %w", err)
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[len(results)-1].ID
		results = results[:limit]
	}

	messages := make([]queue.DLQMessage, len(results))
	for i, msg := range results {
		messages[i] = b.mapXMessageToDLQ(msg)
	}

	return messages, nextCursor, nil
}

func (b *RedisStreamBroker) GetDLQMessage(ctx context.Context, messageID string) (*queue.DLQMessage, error) {
	// XRANGE with same start/end ID fetches a single message
	results, err := b.client.XRange(ctx, b.dlqKey(), messageID, messageID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ message: %w", err)
	}
	if len(results) == 0 {
		return nil, queue.ErrDLQMessageNotFound
	}

	msg := b.mapXMessageToDLQ(results[0])
	return &msg, nil
}

func (b *RedisStreamBroker) DeleteDLQMessage(ctx context.Context, messageID string) error {
	count, err := b.client.XDel(ctx, b.dlqKey(), messageID).Result()
	if err != nil {
		return fmt.Errorf("failed to delete DLQ message: %w", err)
	}
	if count == 0 {
		return queue.ErrDLQMessageNotFound
	}
	return nil
}

func (b *RedisStreamBroker) RequeueDLQMessage(ctx context.Context, messageID string) error {
	// 1. Fetch the message to get its original payload and destination
	msg, err := b.GetDLQMessage(ctx, messageID)
	if err != nil {
		return err
	}

	// 2. Clean up the payload for requeueing (remove DLQ metadata)
	requeuePayload := make(map[string]interface{})
	for k, v := range msg.Payload {
		if !strings.HasPrefix(k, "_") {
			requeuePayload[k] = v
		}
	}

	// 3. Use a pipeline to atomically (mostly) add back to original stream and remove from DLQ
	pipe := b.client.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: msg.OriginalStream, // Send it back where it came from
		Values: requeuePayload,
	})
	pipe.XDel(ctx, b.dlqKey(), messageID)

	_, execErr := pipe.Exec(ctx)
	if execErr != nil {
		return fmt.Errorf("failed to requeue DLQ message: %w", execErr)
	}

	return nil
}

// Helper to map Redis XMessage to domain DLQMessage
func (b *RedisStreamBroker) mapXMessageToDLQ(msg redis.XMessage) queue.DLQMessage {
	originalStream, _ := msg.Values["_original_stream"].(string)
	originalMsgID, _ := msg.Values["_original_message_id"].(string)
	movedAtStr, _ := msg.Values["_moved_at"].(string)

	movedAt, _ := time.Parse(time.RFC3339, movedAtStr)
	if movedAt.IsZero() {
		// Fallback: try to extract time from Redis ID (Unix MS timestamp is first part of ID)
		parts := strings.Split(msg.ID, "-")
		if len(parts) > 0 {
			var ms int64
			// FIX: Explicitly ignore error return to satisfy errcheck linter
			_, _ = fmt.Sscanf(parts[0], "%d", &ms)
			movedAt = time.UnixMilli(ms).UTC()
		}
	}

	return queue.DLQMessage{
		MessageID:         msg.ID,
		OriginalStream:    originalStream,
		OriginalMessageID: originalMsgID,
		MovedAt:           movedAt,
		Payload:           msg.Values,
	}
}
