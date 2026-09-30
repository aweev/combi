package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	redisotel"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func NewClient(cfg config.RedisConfig, logger log.Logger) *redis.Client {
	logger = logger.With(log.String("component", "redis_client"))

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		PoolTimeout:  cfg.PoolTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Failed to connect to Redis", err, log.String("addr", cfg.Addr()))
		panic(fmt.Sprintf("failed to connect to redis: %v", err))
	}

	logger.Info("Redis connection pool established", log.String("addr", cfg.Addr()))

	if cfg.EnableTracing {
		if err := redisotel.InstrumentTracing(client); err != nil {
			logger.Warn("Failed to add OpenTelemetry tracing hook to Redis client", log.ErrorAttr(err))
		} else {
			logger.Info("Enabled OpenTelemetry tracing for Redis client")
		}
	}

	return client
}
