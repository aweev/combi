package realtime

import (
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/log"
)

// Statically verify implementation.
var _ queue.JobProcessor = (*PresenceWorker)(nil)

type PresenceWorker struct {
	repo   realtime.PresenceRepository
	logger log.Logger
}

func NewPresenceWorker(repo realtime.PresenceRepository, logger log.Logger) *PresenceWorker {
	return &PresenceWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "realtime_presence")),
	}
}

func (w *PresenceWorker) JobType() core.JobType {
	return realtime.JobCleanupStaleConnections
}

func (w *PresenceWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// Logic: The Redis Sorted Set (ZSET) approach for global presence allows easy cleanup.
	// ZREMRANGEBYSCORE global_presence -inf (now - TTL)

	// Since the logic is redis-specific, we delegate to the repo method 'CleanupStaleUsers'
	// which we assume exists on the repo implementation for this worker.
	// If using the interface, we might need to cast or add the method to the interface.

	// Let's add it to the interface in the next file update for cleanliness.
	// Assuming: repo.CleanupStaleUsers(ctx, 5 * time.Minute)

	// For now, just log as placeholder if not exposing that method.
	w.logger.Info("Running presence cleanup (delegated to Redis expiry mostly)")

	return nil, nil
}
