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

var _ queue.JobProcessor = (*CleanupWorker)(nil)

type CleanupWorker struct {
	repo   notification.Repository
	logger log.Logger
}

func NewCleanupWorker(repo notification.Repository, logger log.Logger) *CleanupWorker {
	return &CleanupWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "notification_cleanup")),
	}
}

func (w *CleanupWorker) JobType() core.JobType {
	return notification.JobCleanupOld
}

type cleanupPayload struct {
	OlderThanDays int `json:"older_than_days"`
}
type cleanupResult struct {
	NotificationsDeleted int64 `json:"notifications_deleted"`
	EventsDeleted        int64 `json:"events_deleted"`
}

func (w *CleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload cleanupPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload: %w", err))
	}
	if payload.OlderThanDays <= 0 {
		payload.OlderThanDays = 90
	}
	cutoffTime := time.Now().UTC().AddDate(0, 0, -payload.OlderThanDays)

	w.logger.Info("running notification cleanup job", log.Any("cutoff_date", cutoffTime))

	deletedNotifs, err := w.repo.DeleteOldNotifications(ctx, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old notifications: %w", err)
	}

	deletedEvents, err := w.repo.DeleteOldEvents(ctx, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old notification events: %w", err)
	}

	result := cleanupResult{NotificationsDeleted: deletedNotifs, EventsDeleted: deletedEvents}
	resultBytes, _ := json.Marshal(result)

	w.logger.Info("notification cleanup job complete",
		log.Int("notifications_deleted", int(deletedNotifs)),
		log.Int("events_deleted", int(deletedEvents)),
	)
	return resultBytes, nil
}
