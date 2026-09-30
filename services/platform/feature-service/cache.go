package feature

import (
	"context"
	"sync"
	"time"

	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/log"
)

var _ feature.FlagCacheReader = (*FlagCache)(nil)

// FlagCache provides a thread-safe, in-memory cache for feature flag features.
type FlagCache struct {
	repo         feature.Repository
	logger       log.Logger
	pollInterval time.Duration

	mu    sync.RWMutex
	flags map[string]*feature.Flag // Map from flag key to flag object
}

// NewFlagCache creates and initializes a new in-memory flag cache.
func NewFlagCache(repo feature.Repository, logger log.Logger, pollInterval time.Duration) (*FlagCache, error) {
	cache := &FlagCache{
		repo:         repo,
		logger:       logger.With(log.String("component", "flag_cache")),
		pollInterval: pollInterval,
		flags:        make(map[string]*feature.Flag),
	}

	// Perform an initial, blocking load to ensure the cache is ready before the app serves requests.
	if err := cache.fullReload(context.Background()); err != nil {
		return nil, err
	}
	return cache, nil
}

// Start begins the background polling goroutine.
// Note: This does not need a Stop method. It stops when the passed 'ctx' is cancelled
// by the LifecycleManager.
func (c *FlagCache) Start(ctx context.Context) {
	c.logger.Info("Starting background flag cache poller", log.String("interval", c.pollInterval.String()))
	go c.poller(ctx)
}

// Refresh triggers a reload of a specific flag from the DB.
func (c *FlagCache) Refresh(ctx context.Context, key string) {
	c.logger.Info("Refreshing single flag in cache from notification", log.String("flag_key", key))

	// 1. Fetch the latest state from the DB
	flag, err := c.repo.Get(ctx, key)
	if err != nil {
		if err == feature.ErrFlagNotFound {
			// If not found in DB, it was deleted. Remove from cache.
			c.mu.Lock()
			delete(c.flags, key)
			c.mu.Unlock()
			c.logger.Info("Removed deleted/archived flag from cache", log.String("flag_key", key))
		} else {
			c.logger.Error("Failed to fetch updated flag for cache refresh", err, log.String("flag_key", key))
		}
		return
	}

	// 2. Update Cache with Version Check
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the flag currently in cache is newer or same version.
	// This prevents overwriting a newer version loaded by the background poller
	// if the Pub/Sub message arrives late.
	existing, exists := c.flags[key]
	if exists && existing.Version >= flag.Version {
		c.logger.Debug("Ignoring stale flag update from pubsub",
			log.String("key", key),
			log.Int("cached_version", existing.Version),
			log.Int("incoming_version", flag.Version),
		)
		return
	}

	// Update the map
	c.flags[key] = flag
	c.logger.Info("Refreshed flag in cache", log.String("key", key), log.Int("version", flag.Version))
}

// GetFlag retrieves a single flag from the in-memory cache. It returns a copy.
func (c *FlagCache) GetFlag(key string) *feature.Flag {
	c.mu.RLock()
	defer c.mu.RUnlock()
	flag, ok := c.flags[key]
	if !ok {
		return nil
	}
	flagCopy := *flag
	return &flagCopy
}

// ListActiveFlags returns a slice of all active, non-archived flags.
func (c *FlagCache) ListActiveFlags() []*feature.Flag {
	c.mu.RLock()
	defer c.mu.RUnlock()
	activeFlags := make([]*feature.Flag, 0, len(c.flags))
	for _, flag := range c.flags {
		if flag.IsActive && !flag.Archived {
			flagCopy := *flag
			activeFlags = append(activeFlags, &flagCopy)
		}
	}
	return activeFlags
}

// poller is the background goroutine that periodically performs a full reload.
func (c *FlagCache) poller(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.fullReload(ctx); err != nil {
				c.logger.Error("Failed to perform periodic full reload of flags", err)
			}
		case <-ctx.Done():
			c.logger.Info("Flag cache poller stopping.")
			return
		}
	}
}

// fullReload fetches all non-archived flags and atomically replaces the cache.
func (c *FlagCache) fullReload(ctx context.Context) error {
	c.logger.Debug("Performing full reload of flag cache...")
	flagsFromRepo, err := c.repo.ListAll(ctx)
	if err != nil {
		return err
	}

	newFlagMap := make(map[string]*feature.Flag, len(flagsFromRepo))
	for _, flag := range flagsFromRepo {
		f := flag
		newFlagMap[f.Key] = &f
	}

	c.mu.Lock()
	c.flags = newFlagMap
	c.mu.Unlock()

	c.logger.Info("Flag cache reloaded successfully", log.Int("flag_count", len(newFlagMap)))
	return nil
}
