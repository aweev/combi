// internal/platform/limiter/redis_limiter.go
package limiter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/combi/pkg/log"
	"github.com/redis/go-redis/v9"
)

type RateLimiter interface {
	// Allow checks if a rule execution is allowed based on its debounce/throttle settings.
	// Key should be the rule ID.
	Allow(ctx context.Context, key string) (bool, error)
	AllowDebounceThrottle(ctx context.Context, key string, debounce, throttle time.Duration) (bool, error)
}

type Rate struct {
	Limit  int
	Period time.Duration
}

// Statically assert that we implement the domain interface.
var _ RateLimiter = (*RedisRateLimiter)(nil)

var DefaultRate = Rate{Limit: 1000, Period: time.Second}

type RedisRateLimiter struct {
	client      *redis.Client
	config      map[string]Rate
	defaultRate Rate
	prefix      string
	script      *redis.Script
	logger      log.Logger
}

const redisRateLimiterScript = `
-- KEYS[1] = the rate limit key
-- ARGV[1] = the limit
-- ARGV[2] = the expiration time in seconds

local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[2])
end

if count > tonumber(ARGV[1]) then
    return 0 -- Deny
end

return 1 -- Allow
`

func NewRedisRateLimiter(
	client *redis.Client,
	config map[string]Rate,
	defaultRate Rate,
	prefix string,
	logger log.Logger,
) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:      client,
		config:      config,
		defaultRate: defaultRate,
		prefix:      prefix,
		script:      redis.NewScript(redisRateLimiterScript),
		logger:      logger.With(log.String("component", "redis_rate_limiter")),
	}
}

// Allow checks if an action identified by a key is within its configured rate limit.
func (r *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	rate, exists := r.config[key]
	if !exists {
		rate = r.defaultRate
	}

	// A limit of 0 or less means the limiter is disabled for this key.
	if rate.Limit <= 0 {
		return true, nil
	}

	redisKey := r.prefix + key
	periodSeconds := int(rate.Period.Seconds())
	if periodSeconds < 1 {
		periodSeconds = 1 // Ensure expiry is at least 1 second.
	}

	result, err := r.script.Run(ctx, r.client, []string{redisKey}, rate.Limit, periodSeconds).Result()
	if err != nil {
		r.logger.Error("rate limiter script failed, failing open", err, log.String("key", key))
		return true, fmt.Errorf("rate limiter script failed: %w", err)
	}

	allowed, ok := result.(int64)
	if !ok {
		r.logger.Error("unexpected result type from redis script, failing open", nil, log.String("key", key), log.Any("result", result))
		return true, fmt.Errorf("unexpected result type from redis script: %T", result)
	}

	return allowed == 1, nil
}

// AllowDebounceThrottle uses Redis SET with NX and EX for an atomic check, perfect for ensuring
// a minimum time between actions.
func (r *RedisRateLimiter) AllowDebounceThrottle(ctx context.Context, key string, debounce, throttle time.Duration) (bool, error) {
	// Throttle takes precedence as it's a hard minimum time between executions.
	limitDuration := throttle
	if debounce > throttle {
		limitDuration = debounce
	}
	if limitDuration <= 0 {
		return true, nil // No limit configured.
	}

	redisKey := r.prefix + key
	ok, err := r.client.SetNX(ctx, redisKey, "1", limitDuration).Result()
	if err != nil {
		r.logger.Error("debounce/throttle check failed, failing open", err, log.String("key", key))
		return true, err // Fail open on Redis errors.
	}

	return ok, nil
}

func ParseRateLimits(rawLimits map[string]string) (map[string]Rate, error) {
	parsed := make(map[string]Rate)
	for key, value := range rawLimits {
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid rate limit format for '%s': expected 'limit/period', got '%s'", key, value)
		}

		limit, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid number for rate limit '%s': %w", key, err)
		}

		var period time.Duration
		switch strings.ToLower(parts[1]) {
		case "s":
			period = time.Second
		case "m":
			period = time.Minute
		case "h":
			period = time.Hour
		default:
			return nil, fmt.Errorf("invalid period for rate limit '%s': use 's', 'm', or 'h'", key)
		}

		parsed[key] = Rate{Limit: limit, Period: period}
	}
	return parsed, nil
}
