package matchmaking

import (
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

type Worker struct {
	service matchmaking.Service
	logger  log.Logger
}

func NewWorker(s matchmaking.Service, l log.Logger) *Worker {
	return &Worker{
		service: s,
		logger:  l.With(log.String("worker", "matchmaking")),
	}
}

func (w *Worker) JobType() core.JobType {
	return matchmaking.JobRunMatchingBatch
}

func (w *Worker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	w.logger.Info("Starting batch matching job")

	// Process top 50 open requests
	if err := w.service.ProcessBatch(ctx, 50); err != nil {
		return nil, queue.NewRetryableError(err)
	}

	return nil, nil
}
