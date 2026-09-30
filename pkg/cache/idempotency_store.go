package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/combi/pkg/log"

	"github.com/redis/go-redis/v9"
)

var _ IdempotencyStore = (*RedisIdempotencyStore)(nil)

type RedisIdempotencyStore struct {
	client    *redis.Client
	keyPrefix string
	logger    log.Logger
}

func NewRedisIdempotencyStore(client *redis.Client, keyPrefix string, logger log.Logger) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{
		client:    client,
		keyPrefix: keyPrefix,
		logger:    logger.With(log.String("component", "idempotency_store")),
	}
}

type storedResult struct {
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
	IsError bool            `json:"is_error"`
}

func (s *RedisIdempotencyStore) buildKey(key string) string {
	return s.keyPrefix + key
}

// StoreResult implementation from the domain.IdempotencyStore interface.
func (s *RedisIdempotencyStore) StoreResult(ctx context.Context, key string, result json.RawMessage, execErr error, ttl time.Duration) error {
	redisKey := s.buildKey(key)

	payload := storedResult{}
	if execErr != nil {
		payload.IsError = true
		payload.Error = execErr.Error()
	} else {
		payload.Result = result
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, redisKey, data, ttl).Err()
}

// GetResult implementation from the domain.IdempotencyStore interface.
func (s *RedisIdempotencyStore) GetResult(ctx context.Context, key string) (json.RawMessage, error, bool, error) {
	redisKey := s.buildKey(key)

	data, err := s.client.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, false, nil // Not found, no error
		}
		return nil, nil, false, err // A real error occurred
	}

	var payload storedResult
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		// If data is corrupted, treat it as not found and log the error.
		s.logger.Error("failed to unmarshal idempotency result from redis", err, log.String("key", key))
		return nil, nil, false, nil
	}

	if payload.IsError {
		return nil, errors.New(payload.Error), true, nil
	}

	return payload.Result, nil, true, nil
}
