package privacy

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

type ErasureWorker struct {
	manager privacy.Manager
	logger  log.Logger
}

func NewErasureWorker(m privacy.Manager, logger log.Logger) *ErasureWorker {
	return &ErasureWorker{manager: m, logger: logger}
}

func (w *ErasureWorker) JobType() core.JobType {
	return "privacy:process_erasure"
}

func (w *ErasureWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload struct {
		RequestID uuid.UUID `json:"request_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, err
	}

	if err := w.manager.ProcessErasure(ctx, payload.RequestID); err != nil {
		return nil, err
	}

	return nil, nil
}
