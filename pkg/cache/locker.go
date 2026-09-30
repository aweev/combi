// internal/platform/cache/locker.go
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var _ Locker = (*RedisLocker)(nil)
var _ Lock = (*redisLock)(nil)

type redisLock struct {
	client *redis.Client
	key    string
	token  string
}

func (l *redisLock) Unlock(ctx context.Context) error {
	script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Result()
	if err != nil {
		return fmt.Errorf("failed to execute unlock script for key %s: %w", l.key, err)
	}

	if val, ok := res.(int64); !ok || val == 0 {
		return fmt.Errorf("could not unlock key %s: not owner or key expired", l.key)
	}

	return nil
}

func (l *redisLock) Extend(ctx context.Context, ttl time.Duration) error {
	script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("expire", KEYS[1], ARGV[2])
        else
            return 0
        end
    `
	ttlSeconds := int64(ttl.Seconds())
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttlSeconds).Result()
	if err != nil {
		return fmt.Errorf("failed to execute extend script for key %s: %w", l.key, err)
	}

	if val, ok := res.(int64); !ok || val == 0 {
		return fmt.Errorf("could not extend lock for key %s: not owner or key expired", l.key)
	}
	return nil
}

type RedisLocker struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisLocker(client *redis.Client, keyPrefix string) *RedisLocker {
	return &RedisLocker{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

func (r *RedisLocker) AcquireLock(ctx context.Context, key string, ttl time.Duration) (Lock, error) {
	fullKey := r.keyPrefix + key
	token := uuid.New().String()

	acquired, err := r.client.SetNX(ctx, fullKey, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("redis SetNX command failed for key %s: %w", fullKey, err)
	}

	if !acquired {
		return nil, ErrLockNotAcquired // <-- Use the domain error
	}

	return &redisLock{
		client: r.client,
		key:    fullKey,
		token:  token,
	}, nil
}
