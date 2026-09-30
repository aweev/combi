package redis

import (
	"context"
	"time"
)

// ReaperLoop runs the background process to reclaim stalled messages.
// It is exported to allow the bus implementation to start it as a background worker.
func (b *RedisStreamBus) ReaperLoop(ctx context.Context) {
	defer b.wg.Done()

	pollInterval := b.config.ReaperPollInterval
	if pollInterval == 0 {
		pollInterval = 1 * time.Minute
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Use a shorter timeout for the reaper cycle to allow graceful shutdown
			reapCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			b.reap(reapCtx)
			cancel()
		}
	}
}
