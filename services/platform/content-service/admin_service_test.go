package content_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/content"
	content_mocks "github.com/combi/pkg/domain/content/mocks"
	"github.com/combi/pkg/log"
	content_service "github.com/combi/content"
)

type adminServiceTestSuite struct {
	repo    *content_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	service content.AdminService
}

func setupAdminService(t *testing.T) adminServiceTestSuite {
	ts := adminServiceTestSuite{
		repo:  content_mocks.NewMockRepository(t),
		audit: audit_mocks.NewMockEventRecorder(t),
	}
	ts.service = content_service.NewAdminService(
		ts.repo, ts.audit, &log.NoOpLogger{},
	)
	return ts
}

func contextWithAdmin() (context.Context, uuid.UUID) {
	id := uuid.New()
	actor := &auth.Actor{UserID: id, AccountID: id}
	return auth.NewContextWithActor(context.Background(), actor), id
}

func TestAdminService_CreateSpace(t *testing.T) {
	ctx, _ := contextWithAdmin()
	params := content.CreateSpaceParams{
		Key:           "blog",
		Name:          "Blog",
		DefaultLocale: "en-US",
		Locales:       []string{"en-US"},
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.EXPECT().CreateSpace(ctx, mock.MatchedBy(func(s *content.Space) bool {
			return s.Key == "blog" && s.Name == "Blog"
		})).Return(nil).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		space, err := ts.service.CreateSpace(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "blog", space.Key)
	})

	t.Run("Fail_Duplicate", func(t *testing.T) {
		ts := setupAdminService(t)
		ts.repo.EXPECT().CreateSpace(ctx, mock.Anything).Return(content.ErrSpaceAlreadyExists).Once()

		_, err := ts.service.CreateSpace(ctx, params)
		assert.ErrorIs(t, err, content.ErrSpaceAlreadyExists)
	})
}

func TestAdminService_CreateModel(t *testing.T) {
	ctx, _ := contextWithAdmin()
	spaceID := uuid.New()
	params := content.CreateModelParams{
		SpaceID: spaceID,
		APIKey:  "post",
		Name:    "Post",
		Fields:  []content.Field{{ID: "title", Type: content.FieldTypeText}},
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)

		ts.repo.EXPECT().CreateModel(ctx, mock.MatchedBy(func(m *content.Model) bool {
			return m.APIKey == "post" && m.SpaceID == spaceID
		})).Return(nil).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		model, err := ts.service.CreateModel(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "post", model.APIKey)
	})
}

func TestAdminService_UpdateModel(t *testing.T) {
	ctx, _ := contextWithAdmin()
	modelID := uuid.New()
	newName := "Updated Post"
	params := content.UpdateModelParams{Name: &newName}

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)
		existing := &content.Model{ID: modelID, Name: "Post", IsSystem: false}

		ts.repo.EXPECT().GetModel(ctx, modelID).Return(existing, nil).Once()
		ts.repo.EXPECT().IsModelInUse(ctx, modelID).Return(false, nil).Once()

		ts.repo.EXPECT().UpdateModel(ctx, mock.MatchedBy(func(m *content.Model) bool {
			return m.Name == "Updated Post"
		})).Return(nil).Once()

		// Fix: Add Missing Audit Expectation
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		model, err := ts.service.UpdateModel(ctx, modelID, params)
		require.NoError(t, err)
		assert.Equal(t, "Updated Post", model.Name)
	})

	t.Run("Fail_SystemModel", func(t *testing.T) {
		ts := setupAdminService(t)
		systemModel := &content.Model{ID: modelID, IsSystem: true}

		ts.repo.EXPECT().GetModel(ctx, modelID).Return(systemModel, nil).Once()

		_, err := ts.service.UpdateModel(ctx, modelID, params)
		assert.ErrorIs(t, err, content.ErrSystemModelImmutable)
	})

	t.Run("Fail_BreakingChangeInUse", func(t *testing.T) {
		ts := setupAdminService(t)
		existing := &content.Model{ID: modelID, IsSystem: false}
		// Attempting to change fields while in use
		newFields := []content.Field{{ID: "new"}}
		paramsFields := content.UpdateModelParams{Fields: newFields}

		ts.repo.EXPECT().GetModel(ctx, modelID).Return(existing, nil).Once()
		ts.repo.EXPECT().IsModelInUse(ctx, modelID).Return(true, nil).Once()

		_, err := ts.service.UpdateModel(ctx, modelID, paramsFields)
		assert.ErrorIs(t, err, content.ErrModelInUse)
	})
}

func TestAdminService_UpdateEntryStatus(t *testing.T) {
	ctx, _ := contextWithAdmin()
	entryID := uuid.New()
	params := content.UpdateEntryStatusParams{Status: content.StatusRejected}

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)
		entry := &content.Entry{ID: entryID, Status: content.StatusPublished}

		ts.repo.EXPECT().GetEntry(ctx, entryID).Return(entry, nil).Once()

		ts.repo.EXPECT().UpdateEntry(ctx, mock.MatchedBy(func(e *content.Entry) bool {
			return e.Status == content.StatusRejected
		})).Return(nil).Once()

		// Fix: Audit mock was missing
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		updated, err := ts.service.UpdateEntryStatus(ctx, entryID, params)
		require.NoError(t, err)
		assert.Equal(t, content.StatusRejected, updated.Status)
	})
}
