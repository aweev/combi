package feature

import (
	"context"
	"sync"

	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/log"
)

// LifecycleManager coordinates background processes for the feature service.
// It implements feature.CacheLifecycle.
type LifecycleManager struct {
	cache    *FlagCache
	notifier feature.Notifier
	logger   log.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewLifecycleManager creates the coordinator.
func NewLifecycleManager(cache *FlagCache, notifier feature.Notifier, logger log.Logger) *LifecycleManager {
	return &LifecycleManager{
		cache:    cache,
		notifier: notifier,
		logger:   logger,
	}
}

// Start launches the Cache Poller and the Redis Subscriber.
func (l *LifecycleManager) Start(ctx context.Context) {
	// Create a cancellable context for all feature background tasks
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	// 1. Start Cache Poller
	// FlagCache.Start spawns its own goroutine but respects ctx.Done()
	l.cache.Start(ctx)

	// 2. Start Redis Notifier Subscriber
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		// This blocks until ctx is cancelled
		l.notifier.Subscribe(ctx, func(key string) {
			l.Refresh(context.Background(), key)
		})
	}()

	l.logger.Info("Feature service background workers started")
}

// Stop cancels the context, which stops the Poller and the Subscriber.
func (l *LifecycleManager) Stop() {
	l.logger.Info("Stopping feature service background workers...")
	if l.cancel != nil {
		l.cancel() // Signals Notifier.Subscribe and FlagCache.poller to exit
	}
	l.wg.Wait() // Wait for Redis Subscriber to finish cleaning up
	l.logger.Info("Feature service background workers stopped")
}

// Refresh manually triggers a cache update (delegates to cache).
func (l *LifecycleManager) Refresh(ctx context.Context, key string) {
	l.cache.Refresh(ctx, key)
}
