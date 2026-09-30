package search

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/log"
)

// RebuildWorker
type RebuildWorker struct {
	service *service
	logger  log.Logger
}

func NewRebuildWorker(svc *service, logger log.Logger) *RebuildWorker {
	return &RebuildWorker{service: svc, logger: logger}
}

func (w *RebuildWorker) JobType() core.JobType { return search.JobRebuildIndex }

func (w *RebuildWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p map[string]string
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload: %w", err))
	}

	idx := search.IndexName(p["index"])
	if idx == "" {
		return nil, queue.NewNonRetryableError(fmt.Errorf("missing index param"))
	}

	if err := w.service.ProcessRebuildJob(ctx, idx); err != nil {
		w.logger.Error("rebuild failed", err, log.Any("index", idx))
		return nil, err
	}

	return json.Marshal(map[string]string{"status": "success"})
}

// IndexEntityWorker
type IndexEntityWorker struct {
	service search.Service
	logger  log.Logger
}

func NewIndexEntityWorker(svc search.Service, logger log.Logger) *IndexEntityWorker {
	return &IndexEntityWorker{service: svc, logger: logger}
}

func (w *IndexEntityWorker) JobType() core.JobType { return search.JobIndexEntity }

func (w *IndexEntityWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var doc search.SearchDocument
	if err := json.Unmarshal(job.Payload, &doc); err != nil {
		// If payload is bad, don't retry
		return nil, queue.NewNonRetryableError(err)
	}

	if err := w.service.IndexDocument(ctx, doc); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "indexed", "id": doc.ID})
}

type DeleteEntityWorker struct {
	service search.Service
	logger  log.Logger
}

func NewDeleteEntityWorker(svc search.Service, logger log.Logger) *DeleteEntityWorker {
	return &DeleteEntityWorker{service: svc, logger: logger.With(log.String("worker", "search_delete"))}
}

func (w *DeleteEntityWorker) JobType() core.JobType { return search.JobDeleteEntity }

func (w *DeleteEntityWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload map[string]string
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload: %w", err))
	}

	id := payload["id"]
	if id == "" {
		// If ID is missing, retrying won't help.
		return nil, queue.NewNonRetryableError(fmt.Errorf("missing document id"))
	}

	if err := w.service.DeleteDocument(ctx, id); err != nil {
		w.logger.Error("failed to delete document", err, log.String("doc_id", id))
		return nil, err
	}

	return json.Marshal(map[string]string{"status": "deleted", "id": id})
}
