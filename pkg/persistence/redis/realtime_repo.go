package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/realtime"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	presenceKeyPrefix = "ngo:presence:"
	globalPresenceKey = "ngo:presence:global"
)

type RedisPresenceRepository struct {
	client *redis.Client
}

func NewRedisPresenceRepository(client *redis.Client) *RedisPresenceRepository {
	return &RedisPresenceRepository{client: client}
}

func (r *RedisPresenceRepository) Heartbeat(ctx context.Context, userID uuid.UUID, clientID uuid.UUID, ttl time.Duration) error {
	// 1. Store Client Session in a Set for the User
	userKey := fmt.Sprintf("%suser:%s", presenceKeyPrefix, userID.String())
	pipe := r.client.Pipeline()

	// Add Client ID to User's active sessions
	pipe.SAdd(ctx, userKey, clientID.String())
	pipe.Expire(ctx, userKey, ttl)

	// 2. Add User to Global/Channel presence sets if needed (simplified here to global)
	// Storing just UserID in global set with score = timestamp for cleanup
	expiry := time.Now().Add(ttl).Unix()
	pipe.ZAdd(ctx, globalPresenceKey, redis.Z{
		Score:  float64(expiry),
		Member: userID.String(),
	})

	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisPresenceRepository) GetOnlineUsers(ctx context.Context, channel realtime.ChannelID) ([]uuid.UUID, error) {
	// Get users who haven't expired yet
	now := float64(time.Now().Unix())
	userIDsStr, err := r.client.ZRangeByScore(ctx, globalPresenceKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", now),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(userIDsStr))
	for _, s := range userIDsStr {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *RedisPresenceRepository) RemoveClient(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) error {
	userKey := fmt.Sprintf("%suser:%s", presenceKeyPrefix, userID.String())
	return r.client.SRem(ctx, userKey, clientID.String()).Err()
}

func (r *RedisPresenceRepository) CleanupStaleUsers(ctx context.Context) (int64, error) {
	now := float64(time.Now().Unix())
	return r.client.ZRemRangeByScore(ctx, globalPresenceKey, "-inf", fmt.Sprintf("%f", now)).Result()
}
