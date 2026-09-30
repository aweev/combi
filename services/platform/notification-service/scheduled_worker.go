package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

const scheduledBatchSize = 100

var _ queue.JobProcessor = (*ScheduledWorker)(nil)

// ScheduledWorker finds notifications with a future `ScheduledFor` date that are now due.
type ScheduledWorker struct {
	repo       notification.Repository
	dispatcher queue.Dispatcher
	logger     log.Logger
}

func NewScheduledWorker(repo notification.Repository, dispatcher queue.Dispatcher, logger log.Logger) *ScheduledWorker {
	return &ScheduledWorker{
		repo:       repo,
		dispatcher: dispatcher,
		logger:     logger.With(log.String("worker", "notification_scheduled")),
	}
}

func (w *ScheduledWorker) JobType() core.JobType {
	return notification.JobProcessScheduled
}

func (w *ScheduledWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	w.logger.Info("running scheduled notification processing job")
	var totalDispatchedCount int
	now := time.Now().UTC()

	for {
		dueNotifications, err := w.repo.GetScheduledNotifications(ctx, now, scheduledBatchSize)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch scheduled notifications: %w", err)
		}
		if len(dueNotifications) == 0 {
			break
		}

		w.logger.Debug("found batch of due notifications", log.Int("count", len(dueNotifications)))

		for _, n := range dueNotifications {
			jobPayload, _ := json.Marshal(map[string]string{"notification_id": n.ID.String()})
			// Dispatch a `notification:send` job for each due notification.
			_, err := w.dispatcher.Dispatch(ctx, notification.JobSendNotification, jobPayload, queue.WithQueue("notifications"))
			if err != nil {
				w.logger.Error("failed to dispatch job for scheduled notification", err, log.String("notification_id", n.ID.String()))
				continue
			}
			totalDispatchedCount++
		}

		if len(dueNotifications) < scheduledBatchSize {
			break
		}
	}

	result := map[string]int{"dispatched_count": totalDispatchedCount}
	resultBytes, _ := json.Marshal(result)

	w.logger.Info("scheduled notification processing job complete", log.Int("dispatched_count", totalDispatchedCount))
	return resultBytes, nil
}
