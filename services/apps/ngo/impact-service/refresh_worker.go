package impact

import (
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/core" // For JobType
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// RefreshWorker handles the background recalculation of goal progress.
// It is triggered via cron or manually via the queue.
type RefreshWorker struct {
	service impact.Service
	logger  log.Logger
}

func NewRefreshWorker(service impact.Service, logger log.Logger) *RefreshWorker {
	return &RefreshWorker{
		service: service,
		logger:  logger.With(log.String("worker", "impact_refresh_goals")),
	}
}

// JobType matches the definition in the manifest.
func (w *RefreshWorker) JobType() core.JobType {
	return impact.JobRefreshGoals
}

// ProcessJob calls the service to update all active goals.
func (w *RefreshWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	w.logger.Info("starting goal progress refresh")

	// Call the service logic to recalculate metrics
	if err := w.service.RefreshGoalProgress(ctx); err != nil {
		w.logger.Error("failed to refresh goal progress", err)
		return nil, err // Returning error triggers retry logic depending on queue config
	}

	w.logger.Info("goal progress refresh complete")

	// No result payload needed
	return nil, nil
}
