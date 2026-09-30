package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// Statically verify implementation.
var _ queue.JobProcessor = (*SyncWorker)(nil)

// SyncWorker periodically checks the health of active integrations.
type SyncWorker struct {
	repo          integration.Repository
	adminService  integration.AdminService
	auditRecorder audit.EventRecorder
	logger        log.Logger
}

func NewSyncWorker(
	repo integration.Repository,
	adminService integration.AdminService,
	auditRecorder audit.EventRecorder,
	logger log.Logger,
) *SyncWorker {
	return &SyncWorker{
		repo:          repo,
		adminService:  adminService,
		auditRecorder: auditRecorder,
		logger:        logger.With(log.String("worker", "integration_sync")),
	}
}

func (w *SyncWorker) JobType() core.JobType {
	return integration.JobIntegrationRunSync
}

// ProcessJob iterates through all integrations or a specific one if ID is provided.
func (w *SyncWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload struct {
		IntegrationID string `json:"integration_id,omitempty"` // Optional: run for specific ID
	}
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
		}
	}

	var integrations []*integration.Integration
	var err error

	if payload.IntegrationID != "" {
		id, err := uuid.Parse(payload.IntegrationID)
		if err != nil {
			return nil, queue.NewNonRetryableError(fmt.Errorf("invalid integration_id: %w", err))
		}
		integ, err := w.repo.GetIntegration(ctx, id)
		if err != nil {
			return nil, queue.NewNonRetryableError(fmt.Errorf("integration not found: %w", err))
		}
		integrations = []*integration.Integration{integ}
	} else {
		// In a real scenario with many integrations, you'd use pagination here.
		// For now, fetching active ones is sufficient.
		status := integration.StatusActive
		filter := integration.IntegrationFilter{Status: &status, Limit: 100}
		integrations, _, err = w.repo.ListIntegrations(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list integrations: %w", err)
		}
	}

	results := make(map[string]string)
	errorCount := 0

	for _, integ := range integrations {
		w.logger.Info("Syncing integration status", log.String("name", integ.Name), log.String("provider", string(integ.Provider)))

		// Use the AdminService's TestConnection logic
		success, message, err := w.adminService.TestConnection(ctx, integ.ID)

		if !success || err != nil {
			w.logger.Error("Integration health check failed", err, log.String("name", integ.Name))
			w.handleHealthFailure(ctx, integ, message)
			results[integ.Name] = "failed: " + message
			errorCount++
		} else {
			// If it was in error state, recover it
			if integ.Status == integration.StatusError {
				integ.Status = integration.StatusActive
				integ.LastError = nil
				if err := w.repo.UpdateIntegration(ctx, integ); err != nil {
					w.logger.Error("failed to update integration status to active", err, log.String("integration_id", integ.ID.String()))
				}
			}
			results[integ.Name] = "healthy"
		}
	}

	if err := w.auditRecorder.Record(ctx, audit.NewEvent(audit.SystemActorID, "system", core.EventIntegrationConnectionTested).
		WithMetadata(map[string]interface{}{"total": len(integrations), "errors": errorCount}).
		WithSuccess().Build()); err != nil {
		w.logger.Error("failed to record audit event for integration sync", err)
	}

	resBytes, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job results: %w", err)
	}
	return resBytes, nil
}

func (w *SyncWorker) handleHealthFailure(ctx context.Context, integ *integration.Integration, msg string) {
	integ.Status = integration.StatusError
	now := time.Now().UTC()
	integ.LastError = &msg
	integ.LastErrorAt = &now

	if err := w.repo.UpdateIntegration(ctx, integ); err != nil {
		w.logger.Error("Failed to update integration status to ERROR", err)
	}
}
