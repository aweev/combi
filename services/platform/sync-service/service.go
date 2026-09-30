package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/log"
)

var _ sync_domain.Service = (*service)(nil)

type service struct {
	logger log.Logger
	audit  audit.EventRecorder
	repo   sync_domain.Repository

	domains   map[string]sync_domain.SyncableService
	domainsMu sync.RWMutex
}

func NewService(repo sync_domain.Repository, logger log.Logger, audit audit.EventRecorder) *service {
	return &service{
		repo:    repo,
		logger:  logger.With(log.String("service", "offline_sync")),
		audit:   audit,
		domains: make(map[string]sync_domain.SyncableService),
	}
}

// --- Registry ---

func (s *service) RegisterDomain(entityType string, svc sync_domain.SyncableService) {
	s.domainsMu.Lock()
	defer s.domainsMu.Unlock()
	s.domains[entityType] = svc
	s.logger.Info("Registered sync domain", log.String("entity_type", entityType))
}

func (s *service) getDomain(entityType string) (sync_domain.SyncableService, error) {
	s.domainsMu.RLock()
	defer s.domainsMu.RUnlock()
	svc, ok := s.domains[entityType]
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
	return svc, nil
}

// --- Push (Client -> Server) ---

func (s *service) Push(ctx context.Context, actorID uuid.UUID, packet sync_domain.SyncPacket) (*sync_domain.SyncResponse, error) {
	// 1. Device Security Check (New)
	if err := s.repo.EnsureDevice(ctx, actorID, packet.DeviceID); err != nil {
		s.logger.Error("Sync blocked", err, log.String("device", packet.DeviceID))
		return nil, fmt.Errorf("sync denied: %w", err)
	}

	response := &sync_domain.SyncResponse{
		Results:   make([]sync_domain.SyncResult, 0, len(packet.Operations)),
		SyncToken: time.Now().UTC().Format(time.RFC3339Nano),
	}

	// 2. Process Operations
	for _, op := range packet.Operations {
		res := s.processOperation(ctx, actorID, op)
		response.Results = append(response.Results, res)
	}

	// 3. Log Session (New)
	// We log it asynchronously or synchronously depending on preference.
	// This helps debug specific user sync issues.
	go func() {
		// Create a detached context for logging
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status := "SUCCESS" // You might calculate PARTIAL if some ops failed
		_ = s.repo.LogSession(logCtx, actorID, packet.DeviceID, "PUSH", status, len(packet.Operations), nil)
	}()

	// Audit the batch
	if err := s.audit.Record(ctx, audit.NewEvent(actorID, "user", "sync.push").
		WithSuccess().
		WithMetadata(map[string]any{
			"device_id": packet.DeviceID,
			"ops_count": len(packet.Operations),
		}).Build()); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return response, nil
}

func (s *service) processOperation(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) sync_domain.SyncResult {
	result := sync_domain.SyncResult{
		TempID: op.TempID,
		Status: sync_domain.StatusError,
	}

	// 1. Resolve Domain Handler
	domainSvc, err := s.getDomain(op.EntityType)
	if err != nil {
		msg := err.Error()
		result.Error = &msg
		return result
	}

	// 2. Delegate to Domain
	realID, newVersion, err := domainSvc.ApplySyncChange(ctx, actorID, op)

	if err != nil {
		// Distinguish Conflict vs Error
		// Assuming domain returns specific error types, or we handle it generically here.
		// For MVP, we treat errors as failures.
		// If it was a version conflict, the domain logic should handle returning ErrConflict.
		// Here we just map it.
		msg := err.Error()
		result.Error = &msg

		// If domain supports specific conflict error checking:
		// if errors.Is(err, sync_domain.ErrVersionConflict) { result.Status = StatusConflict ... }

		return result
	}

	result.Status = sync_domain.StatusSuccess
	result.EntityID = &realID
	result.NewVersion = newVersion
	return result
}

// --- Pull (Server -> Client) ---

func (s *service) Pull(ctx context.Context, actorID uuid.UUID, deviceID string, since *time.Time) (*sync_domain.DataDelta, error) {
	// 1. Device Security Check (New)
	if err := s.repo.EnsureDevice(ctx, actorID, deviceID); err != nil {
		return nil, fmt.Errorf("sync denied: %w", err)
	}

	delta := &sync_domain.DataDelta{
		SyncToken: time.Now().UTC().Format(time.RFC3339Nano),
		Changes:   make(map[string][]sync_domain.EntityState),
	}

	startTime := time.Time{}
	if since != nil {
		startTime = *since
	}

	s.domainsMu.RLock()
	defer s.domainsMu.RUnlock()

	totalChanges := 0

	for entityType, svc := range s.domains {
		changes, err := svc.FetchDelta(ctx, actorID, startTime)
		if err != nil {
			s.logger.Error("Failed to fetch delta", err, log.String("entity", entityType))
			continue
		}
		if len(changes) > 0 {
			delta.Changes[entityType] = changes
			totalChanges += len(changes)
		}
	}

	// 2. Log Session (New)
	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.LogSession(logCtx, actorID, deviceID, "PULL", "SUCCESS", totalChanges, nil)
	}()

	return delta, nil
}
