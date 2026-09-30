package realtime

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/log"
)

type RedisBroker struct {
	client             *redis.Client
	logger             log.Logger
	mu                 sync.Mutex
	pubsub             *redis.PubSub
	msgChan            chan *realtime.Message
	subscribedChannels map[realtime.ChannelID]struct{}
}

func NewRedisBroker(client *redis.Client, logger log.Logger) *RedisBroker {
	return &RedisBroker{
		client:             client,
		logger:             logger.With(log.String("component", "redis_broker")),
		subscribedChannels: make(map[realtime.ChannelID]struct{}),
		msgChan:            make(chan *realtime.Message, 512),
	}
}

func (b *RedisBroker) Publish(ctx context.Context, channel realtime.ChannelID, message *realtime.Message) error {
	// Ensure channel is set on the message for routing
	message.Channel = channel
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, string(channel), payload).Err()
}

func (b *RedisBroker) Subscribe(ctx context.Context, channels ...realtime.ChannelID) (<-chan *realtime.Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var newChannels []string
	for _, ch := range channels {
		if _, ok := b.subscribedChannels[ch]; !ok {
			newChannels = append(newChannels, string(ch))
			b.subscribedChannels[ch] = struct{}{}
		}
	}

	if b.pubsub == nil {
		if len(newChannels) == 0 {
			// Must subscribe to something to start the loop
			newChannels = append(newChannels, "ngo:internal:heartbeat")
		}

		b.pubsub = b.client.Subscribe(ctx, newChannels...)
		go b.listenLoop()
	} else if len(newChannels) > 0 {
		if err := b.pubsub.Subscribe(ctx, newChannels...); err != nil {
			// Rollback local map state on error
			for _, ch := range channels {
				delete(b.subscribedChannels, ch)
			}
			return nil, err
		}
	}

	return b.msgChan, nil
}

func (b *RedisBroker) Unsubscribe(ctx context.Context, channels ...realtime.ChannelID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pubsub == nil {
		return nil
	}

	var toUnsub []string
	for _, ch := range channels {
		if _, ok := b.subscribedChannels[ch]; ok {
			toUnsub = append(toUnsub, string(ch))
			delete(b.subscribedChannels, ch)
		}
	}

	if len(toUnsub) > 0 {
		return b.pubsub.Unsubscribe(ctx, toUnsub...)
	}
	return nil
}

func (b *RedisBroker) listenLoop() {
	ch := b.pubsub.Channel()
	for redisMsg := range ch {
		var msg realtime.Message
		if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
			b.logger.Error("Invalid realtime message", err)
			continue
		}
		// Ensure the channel is set correctly from the redis topic
		msg.Channel = realtime.ChannelID(redisMsg.Channel)

		select {
		case b.msgChan <- &msg:
		default:
			b.logger.Warn("Realtime message buffer full, dropping", log.String("channel", redisMsg.Channel))
		}
	}
	close(b.msgChan)
}

func (b *RedisBroker) Close() error {
	if b.pubsub != nil {
		return b.pubsub.Close()
	}
	return nil
}
