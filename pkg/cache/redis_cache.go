package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	redisotel"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient establishes a new Redis client connection.
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Host,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	return client, nil
}

// MustNewRedisClient creates a new Redis client or panics
func MustNewRedisClient(cfg config.RedisConfig, logger log.Logger) *redis.Client {
	// Build the full address with host:port
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to initialize Redis client", err)
		panic(fmt.Sprintf("failed to connect to redis: %v", err))
	}

	logger.Info("Redis connection established", log.String("addr", addr))

	if cfg.EnableTracing {
		if err := redisotel.InstrumentTracing(client); err != nil {
			// This is non-fatal. The app can run without Redis tracing.
			logger.Warn("Failed to add OpenTelemetry tracing hook to Redis client", log.ErrorAttr(err))
		} else {
			logger.Info("Enabled OpenTelemetry tracing for Redis client")
		}
	}
	return client
}

type redisPipeliner struct {
	pipe   redis.Pipeliner
	prefix string
}

func (p *redisPipeliner) LPush(ctx context.Context, key string, values ...interface{}) error {
	p.pipe.LPush(ctx, p.prefix+key, values...)
	return nil
}
func (p *redisPipeliner) LTrim(ctx context.Context, key string, start, stop int64) error {
	p.pipe.LTrim(ctx, p.prefix+key, start, stop)
	return nil
}

func (p *redisPipeliner) Expire(ctx context.Context, key string, expiration time.Duration) error {
	p.pipe.Expire(ctx, key, expiration)
	return nil
}

func (p *redisPipeliner) Exec(ctx context.Context) error {
	_, err := p.pipe.Exec(ctx)
	return err
}

type RedisService struct {
	Client    *redis.Client
	KeyPrefix string
}

var _ Service = (*RedisService)(nil)

func NewRedisService(client *redis.Client, keyPrefix string) *RedisService {
	if client == nil {
		return nil
	}
	return &RedisService{Client: client}
}

func (s *RedisService) Pipeline() Pipeliner {
	return &redisPipeliner{
		pipe:   s.Client.Pipeline(),
		prefix: s.KeyPrefix,
	}
}

func (s *RedisService) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := s.KeyPrefix + key
	val, err := s.Client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis GET failed for key %s: %w", fullKey, err)
	}
	return val, nil
}

func (s *RedisService) GetObject(ctx context.Context, key string, value interface{}) (bool, error) {
	data, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, fmt.Errorf("failed to unmarshal cached object for key %s: %w", key, err)
	}
	return true, nil
}

func (s *RedisService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	fullKey := s.KeyPrefix + key
	var dataToStore interface{}
	switch v := value.(type) {
	case []byte, string, int, int64, float64, bool:
		dataToStore = v
	default:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to json marshal value for key %s: %w", fullKey, err)
		}
		dataToStore = jsonData
	}
	return s.Client.Set(ctx, fullKey, dataToStore, expiration).Err()
}

func (s *RedisService) Delete(ctx context.Context, key string) error {
	return s.Client.Del(ctx, s.KeyPrefix+key).Err()
}

func (s *RedisService) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return s.Client.LRange(ctx, s.KeyPrefix+key, start, stop).Result()
}

func (s *RedisService) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = s.KeyPrefix + k
	}
	return s.Client.MGet(ctx, fullKeys...).Result()
}
