package loader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/combi/pkg/compliance/kyctiers"
	"github.com/combi/pkg/compliance/limits"
)

// RemoteProvider defines the interface for fetching live rules from svc-compliance.
// In a real service, your compiled gRPC client struct implements this.
type RemoteProvider interface {
	FetchKYCTier(ctx context.Context, country string, level int) (kyctiers.Tier, error)
	FetchLimit(ctx context.Context, country string, level int) (limits.Limit, error)
}

type cacheEntry struct {
	data      any
	expiresAt time.Time
}

// Loader provides hot-reloading of compliance data from svc-compliance.
// It falls back to the embedded defaults if the service is unavailable.
type Loader struct {
	remote RemoteProvider
	cache  map[string]cacheEntry
	ttl    time.Duration
	mu     sync.RWMutex
}

// New creates a Loader. If remote is nil, it operates entirely offline
// using the embedded YAML defaults (useful for tests and local dev).
func New(remote RemoteProvider) *Loader {
	return &Loader{
		remote: remote,
		cache:  make(map[string]cacheEntry),
		ttl:    5 * time.Minute,
	}
}

// KYCTier returns the live KYC tier definition for country+level.
func (l *Loader) KYCTier(ctx context.Context, country string, level int) kyctiers.Tier {
	key := fmt.Sprintf("kyc:%s:%d", country, level)

	// 1. Check Cache
	if val, ok := l.checkCache(key); ok {
		if tier, ok := val.(kyctiers.Tier); ok {
			return tier
		}
	}

	// 2. Fetch from Remote
	if l.remote != nil {
		if tier, err := l.remote.FetchKYCTier(ctx, country, level); err == nil {
			l.setCache(key, tier)
			return tier
		}
	}

	// 3. Fallback to Embedded Static Rule
	tier, _ := kyctiers.For(country, level)
	return tier
}

// Limit returns the live payment limit for country+level.
func (l *Loader) Limit(ctx context.Context, country string, level int) limits.Limit {
	key := fmt.Sprintf("limit:%s:%d", country, level)

	// 1. Check Cache
	if val, ok := l.checkCache(key); ok {
		if limit, ok := val.(limits.Limit); ok {
			return limit
		}
	}

	// 2. Fetch from Remote
	if l.remote != nil {
		if limit, err := l.remote.FetchLimit(ctx, country, level); err == nil {
			l.setCache(key, limit)
			return limit
		}
	}

	// 3. Fallback to Embedded Static Rule
	limit, _ := limits.For(country, level)
	return limit
}

// InvalidateCache forces an immediate re-fetch on the next call for the given key.
func (l *Loader) InvalidateCache(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.cache, key)
}

// ── Internal Helpers ─────────────────────────────────────────────────────────

func (l *Loader) checkCache(key string) (any, bool) {
	l.mu.RLock()
	entry, exists := l.cache[key]
	l.mu.RUnlock()

	if !exists {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (l *Loader) setCache(key string, data any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache[key] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(l.ttl),
	}
}
