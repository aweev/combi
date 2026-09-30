package secret

import (
	"context"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
)

// cachedSecretService is a decorator that adds a caching layer for GetSecretValue.
type cachedSecretService struct {
	next   secret.Service // The actual service implementation
	cache  cache.Service
	ttl    time.Duration
	logger log.Logger
}

var _ secret.Service = (*cachedSecretService)(nil)

// NewCachedSecretService creates a new caching decorator.
func NewCachedSecretService(next secret.Service, cacheSvc cache.Service, ttl time.Duration, logger log.Logger) secret.Service {
	return &cachedSecretService{
		next:   next,
		cache:  cacheSvc,
		ttl:    ttl,
		logger: logger.With(log.String("component", "cached_secret_service")),
	}
}

func (s *cachedSecretService) buildCacheKey(key string) string {
	return fmt.Sprintf("secret:%s", key)
}

func (s *cachedSecretService) GetSecretValue(ctx context.Context, key string) (secret.SecretValue, error) {
	cacheKey := s.buildCacheKey(key)

	// 1. Try to get the secret from the cache.
	var cachedValue secret.SecretValue
	found, err := s.cache.GetObject(ctx, cacheKey, &cachedValue)
	if err != nil {
		s.logger.Warn("Failed to read from secret cache, falling back to source", log.ErrorAttr(err))
	}
	if found {
		s.logger.Debug("Secret cache hit", log.String("key", key))
		return cachedValue, nil
	}

	s.logger.Debug("Secret cache miss", log.String("key", key))

	// 2. If not in cache, call the underlying service.
	value, err := s.next.GetSecretValue(ctx, key)
	if err != nil {
		return nil, err // Pass through the error
	}

	// 3. Store the retrieved value in the cache for next time.
	// Use a background context to ensure the cache is set even if the client disconnects.
	go func() {
		bgCtx := context.Background()
		if err := s.cache.Set(bgCtx, cacheKey, value, s.ttl); err != nil {
			s.logger.Error("Failed to store secret in cache", err, log.String("key", key))
		}
	}()

	return value, nil
}
