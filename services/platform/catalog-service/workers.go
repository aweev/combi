package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// ReindexWorker processes "catalog:reindex" jobs.
type ReindexWorker struct {
	repo     catalog.Repository
	eventBus events.Bus
	logger   log.Logger
}

func NewReindexWorker(repo catalog.Repository, eventBus events.Bus, logger log.Logger) *ReindexWorker {
	return &ReindexWorker{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger.With(log.String("worker", "catalog_reindex")),
	}
}

func (w *ReindexWorker) JobType() core.JobType {
	return catalog.JobCatalogReindex
}

func (w *ReindexWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	w.logger.Info("starting catalog reindex broadcast")

	offset := 0
	limit := 100
	totalEmitted := 0

	for {
		items, _, err := w.repo.ListItems(ctx, catalog.ItemFilter{Limit: limit, Offset: offset})
		if err != nil {
			return nil, fmt.Errorf("failed to list items for reindex: %w", err)
		}

		if len(items) == 0 {
			break
		}

		for _, item := range items {
			payload, _ := json.Marshal(item)

			evt, _ := events.NewEvent(
				core.EventItemUpdated,
				uuid.Nil, "system",
				payload,
			)

			targetType := "catalog_item"
			evt.TargetID = &item.ID
			evt.TargetType = &targetType

			if err := w.eventBus.Publish(ctx, evt); err != nil {
				w.logger.Error("failed to publish reindex event", err, log.String("item_id", item.ID.String()))
			}
			totalEmitted++
		}

		offset += limit
		if totalEmitted > 50000 {
			w.logger.Warn("reindex safety limit reached", log.Int("count", totalEmitted))
			break
		}
	}

	w.logger.Info("catalog reindex broadcast complete", log.Int("count", totalEmitted))

	res, err := json.Marshal(map[string]int{"emitted": totalEmitted})
	if err != nil {
		return nil, nil // Return nil result if marshalling somehow fails
	}
	return res, nil
}

// StatusUpdateWorker handles time-based status transitions.
type StatusUpdateWorker struct {
	repo     catalog.Repository
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewStatusUpdateWorker(repo catalog.Repository, audit audit.EventRecorder, eventBus events.Bus, logger log.Logger) *StatusUpdateWorker {
	return &StatusUpdateWorker{
		repo:     repo,
		audit:    audit,
		eventBus: eventBus,
		logger:   logger.With(log.String("worker", "catalog_status_update")),
	}
}

func (w *StatusUpdateWorker) JobType() core.JobType {
	return "catalog:update_status"
}

func (w *StatusUpdateWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	status := catalog.StatusComingSoon

	items, _, err := w.repo.ListItems(ctx, catalog.ItemFilter{
		Status: &status,
		Limit:  1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch coming soon items: %w", err)
	}

	updatedCount := 0
	now := time.Now().UTC()

	for _, item := range items {
		var props struct {
			AvailableAt *time.Time `json:"available_at"`
		}
		if err := json.Unmarshal(item.Properties, &props); err != nil {
			continue
		}

		if props.AvailableAt != nil && now.After(*props.AvailableAt) {
			item.Status = catalog.StatusAvailable
			if err := w.repo.UpdateItem(ctx, item); err != nil {
				w.logger.Error("failed to auto-update item status", err, log.String("item_id", item.ID.String()))
				continue
			}

			// FIX: Check error on Record
			if err := w.audit.Record(ctx, audit.NewEvent(uuid.Nil, "system", core.EventItemUpdated).
				WithTarget(item.ID, "catalog_item", "Status auto-updated to AVAILABLE").
				WithSuccess().Build()); err != nil {
				w.logger.Warn("failed to record status update audit", log.ErrorAttr(err))
			}

			payload, _ := json.Marshal(item)
			evt, _ := events.NewEvent(core.EventItemUpdated, uuid.Nil, "system", payload)

			targetType := "catalog_item"
			evt.TargetID = &item.ID
			evt.TargetType = &targetType

			// FIX: Check error on Publish
			if err := w.eventBus.Publish(ctx, evt); err != nil {
				w.logger.Error("failed to publish status update event", err, log.String("item_id", item.ID.String()))
			}

			updatedCount++
		}
	}

	if updatedCount > 0 {
		w.logger.Info("status update worker finished", log.Int("updated", updatedCount))
	}

	res, err := json.Marshal(map[string]int{"updated": updatedCount})
	if err != nil {
		return nil, nil
	}
	return res, nil
}
