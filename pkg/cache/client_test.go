package cache

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
)

func TestMustNewRedisClient_panicsOnBadAddr(t *testing.T) {
	cfg := config.RedisConfig{Host: "127.0.0.1", Port: 1} // nothing listening
	logger := &log.NoOpLogger{}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewRedisClient(cfg, logger)
}

func TestNewRedisClient_ok(t *testing.T) {
	// Re-use the test-container helper to ensure a valid addr
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	// client already created by helper – simply ping again
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
