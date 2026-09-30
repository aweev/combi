package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type ReindexWorker struct {
	repo     content.Repository
	eventBus events.Bus
	logger   log.Logger
}

func NewReindexWorker(repo content.Repository, eventBus events.Bus, logger log.Logger) *ReindexWorker {
	return &ReindexWorker{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger.With(log.String("worker", "content_reindex")),
	}
}

func (w *ReindexWorker) JobType() core.JobType {
	return content.JobContentReindexEntry
}

func (w *ReindexWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p struct {
		EntryID string `json:"entry_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}
	entryID, _ := uuid.Parse(p.EntryID)

	// 1. Fetch Entry
	entry, err := w.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch entry: %w", err)
	}

	// 2. Emit "Updated" event
	// The Search Service listens to this event, fetches the data, and updates the index.
	payload, _ := json.Marshal(entry)
	targetType := "content_entry"

	evt, _ := events.NewEvent(core.EventContentUpdated, entry.UpdatedBy, "user", payload)
	evt.TargetID = &entry.ID
	evt.TargetType = &targetType

	if err := w.eventBus.Publish(ctx, evt); err != nil {
		return nil, err
	}

	w.logger.Info("Triggered re-index event for entry", log.String("entry_id", entry.ID.String()))
	return nil, nil
}
