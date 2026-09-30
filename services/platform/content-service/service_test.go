package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/content"
	content_mocks "github.com/combi/pkg/domain/content/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	content_service "github.com/combi/content"
)

type serviceTestSuite struct {
	repo       *content_mocks.MockRepository
	audit      *audit_mocks.MockEventRecorder
	bus        *events_mocks.MockBus
	dispatcher *queue_mocks.MockDispatcher
	validator  *content_mocks.MockValidator
	service    *content_service.Service
}

func setupService(t *testing.T) serviceTestSuite {
	ts := serviceTestSuite{
		repo:       content_mocks.NewMockRepository(t),
		audit:      audit_mocks.NewMockEventRecorder(t),
		bus:        events_mocks.NewMockBus(t),
		dispatcher: queue_mocks.NewMockDispatcher(t),
		validator:  content_mocks.NewMockValidator(t),
	}
	ts.service = content_service.NewService(
		ts.repo, ts.audit, ts.bus, ts.dispatcher, ts.validator, &log.NoOpLogger{},
	)
	return ts
}

func contextWithUser() (context.Context, uuid.UUID) {
	id := uuid.New()
	actor := &auth.Actor{UserID: id, AccountID: id}
	return auth.NewContextWithActor(context.Background(), actor), id
}

func TestService_CreateEntry(t *testing.T) {
	ctx, _ := contextWithUser()
	modelID := uuid.New()
	spaceID := uuid.New()

	fields := map[string]map[string]any{
		"en-US": {"title": "My Post"},
	}

	params := content.CreateEntryParams{
		ModelID: modelID, SpaceID: spaceID, Fields: fields,
	}

	model := &content.Model{ID: modelID, SpaceID: spaceID}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().GetModel(ctx, modelID).Return(model, nil).Once()
		ts.validator.EXPECT().Validate(ctx, fields, model).Return(nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(content.Repository) error) error {
			ts.repo.EXPECT().CreateEntry(ctx, mock.Anything).Return(nil).Once()
			ts.repo.EXPECT().CreateVersion(ctx, mock.Anything).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		// Fix: Add EventBus Expectation
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()
		// Fix: Add Reindex Dispatch Expectation
		ts.dispatcher.EXPECT().Dispatch(ctx, content.JobContentReindexEntry, mock.Anything, mock.Anything).Return(nil, nil).Once()

		entry, err := ts.service.CreateEntry(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, 1, entry.Version)
	})

	t.Run("Fail_Validation", func(t *testing.T) {
		ts := setupService(t)
		valErr := errors.New("field missing")

		ts.repo.EXPECT().GetModel(ctx, modelID).Return(model, nil).Once()
		ts.validator.EXPECT().Validate(ctx, fields, model).Return(valErr).Once()

		_, err := ts.service.CreateEntry(ctx, params)
		assert.ErrorIs(t, err, valErr)
	})
}

func TestService_UpdateEntry(t *testing.T) {
	ctx, userID := contextWithUser()
	entryID := uuid.New()
	modelID := uuid.New()

	entry := &content.Entry{
		ID: entryID, ModelID: modelID, Version: 2, Status: content.StatusDraft,
		Fields: map[string]map[string]any{},
	}
	model := &content.Model{ID: modelID}

	params := content.UpdateEntryParams{
		Version: 2,
		Fields:  map[string]map[string]any{"en": {"title": "Updated"}},
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(content.Repository) error) error {
			ts.repo.EXPECT().GetEntry(ctx, entryID).Return(entry, nil).Once()
			ts.repo.EXPECT().GetModel(ctx, modelID).Return(model, nil).Once()
			ts.validator.EXPECT().Validate(ctx, params.Fields, model).Return(nil).Once()
			ts.repo.EXPECT().UpdateEntry(ctx, mock.MatchedBy(func(e *content.Entry) bool {
				return e.Version == 3 && e.UpdatedBy == userID
			})).Return(nil).Once()
			ts.repo.EXPECT().CreateVersion(ctx, mock.MatchedBy(func(v *content.Version) bool {
				return v.Version == 3
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		// Fix: Add Audit, Bus, and Dispatcher Expectations
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()
		ts.dispatcher.EXPECT().Dispatch(ctx, content.JobContentReindexEntry, mock.Anything, mock.Anything).Return(nil, nil).Once()

		updated, err := ts.service.UpdateEntry(ctx, entryID, params)
		require.NoError(t, err)
		assert.Equal(t, 3, updated.Version)
	})

	t.Run("Fail_Conflict", func(t *testing.T) {
		ts := setupService(t)
		params.Version = 1

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(content.Repository) error) error {
			ts.repo.EXPECT().GetEntry(ctx, entryID).Return(entry, nil).Once()
			return fn(ts.repo)
		}).Once()

		_, err := ts.service.UpdateEntry(ctx, entryID, params)
		assert.ErrorIs(t, err, content.ErrConflict)
	})
}

func TestService_PublishEntry(t *testing.T) {
	ctx, _ := contextWithUser()
	entryID := uuid.New()
	entry := &content.Entry{ID: entryID, Version: 5, Status: content.StatusDraft}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(content.Repository) error) error {
			ts.repo.EXPECT().GetEntry(ctx, entryID).Return(entry, nil).Once()
			ts.repo.EXPECT().UpdateEntry(ctx, mock.MatchedBy(func(e *content.Entry) bool {
				return e.Status == content.StatusPublished &&
					e.PublishedVersion != nil &&
					*e.PublishedVersion == 5
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		// Fix: Add Audit and EventBus Expectations
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()
		ts.dispatcher.EXPECT().Dispatch(ctx, content.JobContentReindexEntry, mock.Anything, mock.Anything).Return(nil, nil).Once()

		err := ts.service.PublishEntry(ctx, entryID)
		require.NoError(t, err)
	})
}

func TestService_RestoreVersion(t *testing.T) {
	ctx, _ := contextWithUser()
	entryID := uuid.New()
	versionID := uuid.New()

	currentEntry := &content.Entry{ID: entryID, Version: 5, Status: content.StatusDraft}
	targetVersion := &content.Version{
		ID: versionID, EntryID: entryID, Version: 3,
		Fields: map[string]map[string]any{"en": {"title": "Old Title"}},
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(content.Repository) error) error {
			ts.repo.EXPECT().GetVersion(ctx, versionID).Return(targetVersion, nil).Once()
			ts.repo.EXPECT().GetEntry(ctx, entryID).Return(currentEntry, nil).Once()
			ts.repo.EXPECT().UpdateEntry(ctx, mock.MatchedBy(func(e *content.Entry) bool {
				return e.Version == 6 && e.Status == content.StatusDraft
			})).Return(nil).Once()
			ts.repo.EXPECT().CreateVersion(ctx, mock.MatchedBy(func(v *content.Version) bool {
				return v.Version == 6
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		restored, err := ts.service.RestoreVersion(ctx, entryID, versionID)
		require.NoError(t, err)
		assert.Equal(t, 6, restored.Version)
	})
}
