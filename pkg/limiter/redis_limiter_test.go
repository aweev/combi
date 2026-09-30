// internal/platform/limiter/redis_limiter_test.go
package limiter_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/limiter"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisRateLimiter_Allow(t *testing.T) {
	// 1. Setup Test Redis
	client, teardown := testutil.NewTestRedis(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	ctx := context.Background()

	// 2. Define Configuration
	config := map[string]limiter.Rate{
		"strict-limit": {Limit: 2, Period: 1 * time.Second},  // Strict: 2 requests per second
		"high-limit":   {Limit: 50, Period: 1 * time.Minute}, // High: 50 requests per minute
		"disabled":     {Limit: 0, Period: 1 * time.Second},  // Disabled: Should always allow
	}
	defaultRate := limiter.Rate{Limit: 5, Period: 1 * time.Second}

	// 3. Initialize Limiter
	rl := limiter.NewRedisRateLimiter(client, config, defaultRate, "test:rl:", logger)

	t.Run("should allow requests within limit", func(t *testing.T) {

		// First request
		allowed, err := rl.Allow(ctx, "strict-limit")
		require.NoError(t, err)
		assert.True(t, allowed, "First request should be allowed")

		// Second request
		allowed, err = rl.Allow(ctx, "strict-limit")
		require.NoError(t, err)
		assert.True(t, allowed, "Second request should be allowed")
	})

	t.Run("should deny requests exceeding limit", func(t *testing.T) {

		// Manually override key logic by passing specific config key for the test
		// Note: In the real implementation, the key passed to Allow() looks up the config.
		// So we use "strict-limit" to lookup config, but we need unique Redis keys for isolation.
		// *However*, the current implementation uses the key string passed to Allow() BOTH for config lookup AND Redis key.
		// This means to test isolation we need to configure specific keys in the map or clean redis.
		// Let's stick to the implementation: Allow(ctx, key). Key is used for config map lookup.

		// To test isolation properly with the current implementation, we need to flush DB or wait.
		// Since we used "strict-limit" in the previous test, the counter is already at 2.

		allowed, err := rl.Allow(ctx, "strict-limit")
		require.NoError(t, err)
		assert.False(t, allowed, "Third request (exceeding limit of 2) should be denied")
	})

	t.Run("should reset limit after period expires", func(t *testing.T) {
		// Wait for the 1s period defined in "strict-limit" to pass
		time.Sleep(1100 * time.Millisecond)

		allowed, err := rl.Allow(ctx, "strict-limit")
		require.NoError(t, err)
		assert.True(t, allowed, "Request should be allowed after window expiration")
	})

	t.Run("should use default rate for unknown keys", func(t *testing.T) {
		// "unknown-key" is not in the map, so it uses defaultRate (Limit: 5)
		for i := 0; i < 5; i++ {
			allowed, err := rl.Allow(ctx, "unknown-key")
			require.NoError(t, err)
			assert.True(t, allowed, "Request %d within default limit should be allowed", i+1)
		}

		// 6th request should fail
		allowed, err := rl.Allow(ctx, "unknown-key")
		require.NoError(t, err)
		assert.False(t, allowed, "Request exceeding default limit should be denied")
	})

	t.Run("should always allow if limit is 0 or negative", func(t *testing.T) {
		// "disabled" has Limit: 0
		for i := 0; i < 10; i++ {
			allowed, err := rl.Allow(ctx, "disabled")
			require.NoError(t, err)
			assert.True(t, allowed, "Should allow infinite requests when limit is 0")
		}
	})
}

func TestRedisRateLimiter_DebounceThrottle(t *testing.T) {
	client, teardown := testutil.NewTestRedis(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	ctx := context.Background()
	rl := limiter.NewRedisRateLimiter(client, nil, limiter.DefaultRate, "test:dt:", logger)

	t.Run("should allow first request", func(t *testing.T) {
		allowed, err := rl.AllowDebounceThrottle(ctx, "action:123", 0, 500*time.Millisecond)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should deny immediate subsequent request", func(t *testing.T) {
		allowed, err := rl.AllowDebounceThrottle(ctx, "action:123", 0, 500*time.Millisecond)
		require.NoError(t, err)
		assert.False(t, allowed, "Should be throttled")
	})

	t.Run("should allow request after throttle duration", func(t *testing.T) {
		time.Sleep(600 * time.Millisecond)
		allowed, err := rl.AllowDebounceThrottle(ctx, "action:123", 0, 500*time.Millisecond)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should respect debounce over throttle if longer", func(t *testing.T) {
		// Debounce (1s) > Throttle (100ms) -> Should use 1s
		allowed, err := rl.AllowDebounceThrottle(ctx, "debounce:1", 1*time.Second, 100*time.Millisecond)
		require.NoError(t, err)
		assert.True(t, allowed)

		time.Sleep(200 * time.Millisecond) // Wait longer than throttle, shorter than debounce

		allowed, err = rl.AllowDebounceThrottle(ctx, "debounce:1", 1*time.Second, 100*time.Millisecond)
		require.NoError(t, err)
		assert.False(t, allowed, "Should be blocked by debounce duration")
	})
}

func TestParseRateLimits(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]string
		want      map[string]limiter.Rate
		wantErr   bool
		errSubStr string
	}{
		{
			name: "Valid configuration",
			input: map[string]string{
				"auth": "5/s",
				"api":  "1000/h",
				"job":  "30/m",
			},
			want: map[string]limiter.Rate{
				"auth": {Limit: 5, Period: time.Second},
				"api":  {Limit: 1000, Period: time.Hour},
				"job":  {Limit: 30, Period: time.Minute},
			},
			wantErr: false,
		},
		{
			name: "Invalid format (missing separator)",
			input: map[string]string{
				"bad": "5s",
			},
			wantErr:   true,
			errSubStr: "invalid rate limit format",
		},
		{
			name: "Invalid format (not a number)",
			input: map[string]string{
				"bad": "foo/s",
			},
			wantErr:   true,
			errSubStr: "invalid number",
		},
		{
			name: "Invalid format (invalid period unit)",
			input: map[string]string{
				"bad": "5/x",
			},
			wantErr:   true,
			errSubStr: "invalid period",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := limiter.ParseRateLimits(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubStr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
