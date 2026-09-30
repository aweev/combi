package redis

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/queue"
	"github.com/redis/go-redis/v9"
)

var _ queue.StateRepository = (*QueueStateRepository)(nil)

const pausedSetKey = "ngo:queue:paused_queues"

type QueueStateRepository struct {
	client *redis.Client
}

func NewQueueStateRepository(client *redis.Client) *QueueStateRepository {
	return &QueueStateRepository{client: client}
}

func (r *QueueStateRepository) PauseQueue(ctx context.Context, queueName string) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	return r.client.SAdd(ctx, pausedSetKey, queueName).Err()
}

func (r *QueueStateRepository) ResumeQueue(ctx context.Context, queueName string) error {
	if queueName == "" {
		return fmt.Errorf("queue name cannot be empty")
	}
	return r.client.SRem(ctx, pausedSetKey, queueName).Err()
}

func (r *QueueStateRepository) IsQueuePaused(ctx context.Context, queueName string) (bool, error) {
	return r.client.SIsMember(ctx, pausedSetKey, queueName).Result()
}

func (r *QueueStateRepository) GetPausedQueues(ctx context.Context) ([]string, error) {
	return r.client.SMembers(ctx, pausedSetKey).Result()
}
