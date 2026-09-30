package impact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type EventWorker struct {
	service impact.Service
	repo    impact.Repository
	logger  log.Logger
}

func NewEventWorker(service impact.Service, repo impact.Repository, logger log.Logger) *EventWorker {
	return &EventWorker{service: service, repo: repo, logger: logger}
}

func (w *EventWorker) HandleEvent(ctx context.Context, evt events.DomainEvent) error {
	// Look up rules in DB for this event type
	mappings, err := w.repo.GetMappingsForEvent(ctx, evt.Action.String())
	if err != nil {
		w.logger.Error("failed to get impact mappings for event", err, log.String("event_action", string(evt.Action)))
		return err
	}
	if len(mappings) == 0 {
		return nil // No impact to record, not an error.
	}

	var payload map[string]interface{}
	if err := evt.UnmarshalPayload(&payload); err != nil {
		w.logger.Warn("failed to unmarshal payload for impact mapping", log.ErrorAttr(err), log.String("event_action", string(evt.Action)))
		return nil // Can't process if payload is malformed
	}

	for _, m := range mappings {
		rawValue, err := extractValueByPath(payload, m.ValuePath)
		if err != nil {
			w.logger.Warn("failed to extract value from payload",
				log.ErrorAttr(err),
				log.String("value_path", m.ValuePath),
				log.String("metric_id", m.MetricID.String()),
			)
			continue
		}

		entry := impact.ImpactEntry{
			MetricID:      m.MetricID,
			Value:         rawValue * m.Multiplier,
			OccurredAt:    evt.Timestamp,
			SourceService: evt.ActorType,   // Use ActorType from the event as the source
			SourceID:      evt.ID.String(), // Use Event ID as a unique source identifier for the entry
		}

		// Try to find an Organization ID if it exists in the payload
		if orgIDStr, ok := payload["organization_id"].(string); ok {
			if orgID, err := uuid.Parse(orgIDStr); err == nil {
				entry.OrganizationID = &orgID
			}
		}

		if err := w.service.RecordImpact(ctx, entry); err != nil {
			w.logger.Error("failed to record impact from event", err)
			// Continue to other mappings even if one fails
		}
	}
	return nil
}

// extractValueByPath navigates a nested map[string]interface{} using a dot-separated path.
func extractValueByPath(payload map[string]interface{}, path string) (float64, error) {
	parts := strings.Split(path, ".")
	var current interface{} = payload

	for _, part := range parts {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("path traversal failed: expected map at '%s', but got %T", part, current)
		}

		value, exists := asMap[part]
		if !exists {
			return 0, fmt.Errorf("path traversal failed: key '%s' not found", part)
		}
		current = value
	}

	// Attempt to convert the final value to float64
	switch v := current.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("final value is not a number: got type %T", current)
	}
}
