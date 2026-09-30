package workers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// StatusUpdateWorker transitions past events to 'COMPLETED'.
type StatusUpdateWorker struct {
	repo   event.Repository
	logger log.Logger
}

func NewStatusUpdateWorker(repo event.Repository, logger log.Logger) *StatusUpdateWorker {
	return &StatusUpdateWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "event_status_update")),
	}
}

func (w *StatusUpdateWorker) JobType() core.JobType {
	return event.JobEventUpdateStatus
}

func (w *StatusUpdateWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	w.logger.Info("running event status updates")

	status := event.StatusPublished
	allPublished, _, err := w.repo.ListEvents(ctx, event.EventFilter{Status: &status, Limit: 1000})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	count := 0

	for _, e := range allPublished {
		if e.EndTime.Before(now) {
			e.Status = event.StatusCompleted
			if err := w.repo.UpdateEvent(ctx, e); err != nil {
				w.logger.Error("failed to complete event", err, log.String("event_id", e.ID.String()))
			} else {
				count++

			}
		}
	}

	w.logger.Info("event status update complete", log.Int("completed_count", count))
	data, err := json.Marshal(map[string]int64{"completed": int64(count)})
	if err != nil {
		return nil, err
	}
	return data, nil
}
