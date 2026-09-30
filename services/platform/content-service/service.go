package content

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ content.Service = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo       content.Repository
	audit      audit.EventRecorder
	eventBus   events.Bus
	dispatcher queue.Dispatcher
	validator  content.Validator
	logger     log.Logger
}

func NewService(
	repo content.Repository,
	audit audit.EventRecorder,
	eventBus events.Bus,
	dispatcher queue.Dispatcher,
	validator content.Validator,
	logger log.Logger,
) *Service {
	return &Service{
		repo:       repo,
		audit:      audit,
		eventBus:   eventBus,
		dispatcher: dispatcher,
		validator:  validator,
		logger:     logger.With(log.String("service", "content_public")),
	}
}

// --- Entry Management ---

func (s *Service) CreateEntry(ctx context.Context, params content.CreateEntryParams) (*content.Entry, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, content.ErrUnauthorized // Should be caught by middleware but good for safety
	}

	// 1. Validate Model Schema
	model, err := s.repo.GetModel(ctx, params.ModelID)
	if err != nil {
		return nil, err
	}
	if model.SpaceID != params.SpaceID {
		return nil, content.ErrModelNotFound // Integrity check
	}

	if err := s.validator.Validate(ctx, params.Fields, model); err != nil {
		return nil, err
	}

	// 2. Create Entry
	entry := &content.Entry{
		ID:        uuid.New(),
		ModelID:   params.ModelID,
		SpaceID:   params.SpaceID,
		Status:    content.StatusDraft,
		Fields:    params.Fields,
		Version:   1,
		CreatedBy: actor.UserID,
		UpdatedBy: actor.UserID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// 3. Persist (Entry + Initial Version)
	err = s.repo.WithTx(ctx, func(txRepo content.Repository) error {
		if err := txRepo.CreateEntry(ctx, entry); err != nil {
			return err
		}
		// Create V1 snapshot
		version := &content.Version{
			ID:        uuid.New(),
			EntryID:   entry.ID,
			Version:   1,
			Fields:    entry.Fields,
			CreatedBy: actor.UserID,
			CreatedAt: entry.CreatedAt,
		}
		return txRepo.CreateVersion(ctx, version)
	})
	if err != nil {
		return nil, err
	}

	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventContentCreated).
		WithTarget(entry.ID, "content_entry", "").WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record content creation audit", log.ErrorAttr(err))
	}

	s.publishEvent(ctx, core.EventContentCreated, entry.ID, entry)
	s.dispatchReindex(ctx, entry.ID)
	return entry, nil
}

func (s *Service) UpdateEntry(ctx context.Context, entryID uuid.UUID, params content.UpdateEntryParams) (*content.Entry, error) {
	actor := auth.ActorFromContext(ctx)

	var updatedEntry *content.Entry

	err := s.repo.WithTx(ctx, func(txRepo content.Repository) error {
		entry, err := txRepo.GetEntry(ctx, entryID)
		if err != nil {
			return err
		}

		// Optimistic Locking
		if entry.Version != params.Version {
			return content.ErrConflict
		}

		model, err := txRepo.GetModel(ctx, entry.ModelID)
		if err != nil {
			return err
		}

		if err := s.validator.Validate(ctx, params.Fields, model); err != nil {
			return err
		}

		// Update State
		entry.Fields = params.Fields
		entry.Version++
		entry.UpdatedBy = actor.UserID
		entry.UpdatedAt = time.Now().UTC()

		// If it was published, it is now CHANGED (Draft ahead of Published)
		if entry.Status == content.StatusPublished {
			entry.Status = content.StatusChanged
		}

		if err := txRepo.UpdateEntry(ctx, entry); err != nil {
			return err
		}

		// Create new Version snapshot
		version := &content.Version{
			ID:        uuid.New(),
			EntryID:   entry.ID,
			Version:   entry.Version,
			Fields:    entry.Fields,
			CreatedBy: actor.UserID,
			CreatedAt: entry.UpdatedAt,
		}
		if err := txRepo.CreateVersion(ctx, version); err != nil {
			return err
		}

		updatedEntry = entry
		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventContentUpdated).
		WithTarget(entryID, "content_entry", "").WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record audit", log.ErrorAttr(err))
	}

	s.publishEvent(ctx, core.EventContentUpdated, entryID, updatedEntry)

	s.dispatchReindex(ctx, entryID)

	return updatedEntry, nil
}

func (s *Service) GetEntry(ctx context.Context, entryID uuid.UUID) (*content.Entry, error) {
	return s.repo.GetEntry(ctx, entryID)
}

func (s *Service) ListEntries(ctx context.Context, modelID uuid.UUID, filter content.EntryFilter) ([]*content.Entry, int, error) {
	return s.repo.ListEntries(ctx, modelID, filter)
}

func (s *Service) ListEntriesByModelKey(ctx context.Context, modelKey string, filter content.EntryFilter) ([]*content.Entry, int, error) {
	model, err := s.repo.GetModelByAPIKey(ctx, modelKey)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.ListEntries(ctx, model.ID, filter)
}

func (s *Service) DeleteEntry(ctx context.Context, entryID uuid.UUID) error {
	// Soft delete or hard delete depending on requirements.
	// Interface says DeleteEntry, repo typically does hard delete in this phase.
	// Logic: Archive first?

	// For simplicity: Hard delete
	if err := s.repo.DeleteEntry(ctx, entryID); err != nil {
		return err
	}
	return nil
}

func (s *Service) ArchiveEntry(ctx context.Context, entryID uuid.UUID) error {
	entry, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}

	entry.Status = content.StatusArchived
	entry.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateEntry(ctx, entry)
}

// --- Publishing ---

func (s *Service) PublishEntry(ctx context.Context, entryID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)

	err := s.repo.WithTx(ctx, func(txRepo content.Repository) error {
		entry, err := txRepo.GetEntry(ctx, entryID)
		if err != nil {
			return err
		}

		// Logic: The current draft (fields) becomes the published version.
		now := time.Now().UTC()

		entry.Status = content.StatusPublished
		entry.PublishedVersion = &entry.Version
		entry.PublishedAt = &now
		entry.UpdatedBy = actor.UserID // Publisher is the updater of status

		if err := txRepo.UpdateEntry(ctx, entry); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventContentPublished).
		WithTarget(entryID, "content_entry", "").WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record audit", log.ErrorAttr(err))
	}

	s.publishEvent(ctx, core.EventContentPublished, entryID, map[string]string{"entry_id": entryID.String()})

	// Dispatch Re-index Job
	s.dispatchReindex(ctx, entryID)

	return nil
}

func (s *Service) UnpublishEntry(ctx context.Context, entryID uuid.UUID) error {
	entry, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}

	if entry.Status != content.StatusPublished && entry.Status != content.StatusChanged {
		return content.ErrNotPublished
	}

	// Revert status to DRAFT (content remains, just not live)
	entry.Status = content.StatusDraft
	entry.PublishedVersion = nil
	entry.PublishedAt = nil

	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return err
	}

	if err := s.audit.Record(ctx, audit.NewEvent(auth.ActorFromContext(ctx).UserID, "user", core.EventContentUnpublished).
		WithTarget(entryID, "content_entry", "").WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record audit", log.ErrorAttr(err))
	}

	s.publishEvent(ctx, core.EventContentUnpublished, entryID, map[string]string{"entry_id": entryID.String()})

	// Trigger search removal or update
	s.dispatchReindex(ctx, entryID)

	return nil
}

// --- Versioning ---

func (s *Service) ListVersions(ctx context.Context, entryID uuid.UUID) ([]*content.Version, error) {
	return s.repo.ListVersionsForEntry(ctx, entryID)
}

func (s *Service) RestoreVersion(ctx context.Context, entryID, versionID uuid.UUID) (*content.Entry, error) {
	actor := auth.ActorFromContext(ctx)

	var restoredEntry *content.Entry

	err := s.repo.WithTx(ctx, func(txRepo content.Repository) error {
		// 1. Get Version Snapshot
		v, err := txRepo.GetVersion(ctx, versionID)
		if err != nil {
			return err
		}
		if v.EntryID != entryID {
			return content.ErrVersionNotFound // Mismatch
		}

		// 2. Get Current Entry
		entry, err := txRepo.GetEntry(ctx, entryID)
		if err != nil {
			return err
		}

		// 3. Apply Snapshot as New Draft (creates a new version number)
		entry.Fields = v.Fields
		entry.Version++
		entry.Status = content.StatusDraft // Restoring always reverts to draft
		entry.UpdatedBy = actor.UserID
		entry.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateEntry(ctx, entry); err != nil {
			return err
		}

		// 4. Save this restoration action as a new version history point
		newV := &content.Version{
			ID:        uuid.New(),
			EntryID:   entry.ID,
			Version:   entry.Version,
			Fields:    entry.Fields,
			CreatedBy: actor.UserID,
			CreatedAt: entry.UpdatedAt,
		}
		if err := txRepo.CreateVersion(ctx, newV); err != nil {
			return err
		}

		restoredEntry = entry
		return nil
	})

	if err != nil {
		return nil, err
	}

	return restoredEntry, nil
}

// ApplySyncChange handles offline modifications.
// For Content, typically only Admins modify content. Regular users consume it.
// If a non-admin tries to push changes, this will fail or be ignored.
// For the MVP, we assume Read-Only for mobile clients unless we expand scope.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	// 1. Identify Type
	if op.EntityType != "content_entry" {
		return uuid.Nil, 0, fmt.Errorf("content service does not support sync for type: %s", op.EntityType)
	}

	// 2. Validate Action (Only Admins usually)
	// We rely on the Sync Service calling this within a context that has the Actor.
	// However, we might want to enforce stricter checks here if `actorID` isn't an admin.
	// For simplicity, we reject offline content edits in MVP.
	return uuid.Nil, 0, fmt.Errorf("offline content editing is not supported in this version")
}

// FetchDelta returns published content that has changed since `since`.
// This allows mobile apps to cache news/blogs offline.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	// 1. Fetch changed entries
	// We need a repository method that finds entries where:
	// - Status = PUBLISHED
	// - UpdatedAt > since (or PublishedAt > since)
	// Since standard ListEntries usually paginates, we might need a specialized repo method `ScanChangedEntries`.
	// For MVP, assuming volume is low, we might list all published and filter in memory, or add the method.

	// Assuming `ScanPublishedEntries` exists on Repo or we reuse List with a custom filter.
	// Let's assume we filter in memory for now using ListEntries (inefficient for large datasets but safe for start).
	filter := content.EntryFilter{
		Status: func() *content.EntryStatus { s := content.StatusPublished; return &s }(),
	}

	// Note: In real prod, this needs pagination loop or a stream.
	entries, _, err := s.repo.ListEntries(ctx, uuid.Nil, filter) // uuid.Nil modelID implies "all models"
	if err != nil {
		return nil, err
	}

	results := make([]sync_domain.EntityState, 0)

	for _, e := range entries {
		// Only include if modified after 'since'
		if e.UpdatedAt.After(since) {
			// Convert fields to JSON
			data, _ := json.Marshal(e)

			results = append(results, sync_domain.EntityState{
				ID:        e.ID,
				Version:   e.Version,
				Deleted:   false, // Logic for deleted content requires Soft Delete tracking (tombstones)
				Data:      data,
				UpdatedAt: e.UpdatedAt,
			})
		}
	}

	return results, nil
}

// Helper
func (s *Service) dispatchReindex(ctx context.Context, entryID uuid.UUID) {
	payload, _ := json.Marshal(map[string]string{"entry_id": entryID.String()})
	if _, err := s.dispatcher.Dispatch(ctx, content.JobContentReindexEntry, payload, queue.WithQueue("bulk")); err != nil {
		s.logger.Error("failed to dispatch reindex job", err, log.String("entry_id", entryID.String()))
	}
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, targetID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, uuid.Nil, "system", payload)
	if err == nil {
		tType := "content_entry"
		evt.TargetID = &targetID
		evt.TargetType = &tType
		if err := s.eventBus.Publish(ctx, evt); err != nil {
			s.logger.Error("failed to publish event", err)
		}
	}
}
