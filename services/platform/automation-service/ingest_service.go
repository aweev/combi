package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/queue"
)

// Statically verify implementation.
var _ automation.EventIngestService = (*ingestService)(nil)

type ingestService struct {
	dispatcher queue.Dispatcher
}

// NewIngestService creates a new, simple ingest service that dispatches events to the queue.
func NewIngestService(dispatcher queue.Dispatcher) automation.EventIngestService {
	return &ingestService{dispatcher: dispatcher}
}

// Ingest implements the EventIngestService interface by enqueuing a job.
func (s *ingestService) Ingest(ctx context.Context, event automation.Event) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal automation event: %w", err)
	}

	_, err = s.dispatcher.Dispatch(ctx, automation.JobAutomationProcessEvent, payload, queue.WithQueue("automation"))
	if err != nil {
		return fmt.Errorf("failed to dispatch automation event job: %w", err)
	}

	return nil
}
