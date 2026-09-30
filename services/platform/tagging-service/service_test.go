package tagging_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/domain/tagging"
	tagging_mocks "github.com/combi/pkg/domain/tagging/mocks"
	event_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	tagging_service "github.com/combi/tagging"
)

type taggingServiceTestSuite struct {
	repo    *tagging_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	bus     *event_mocks.MockBus
	authz   *rbac_mocks.MockAuthorizationService
	service *tagging_service.Service
}

func setupTaggingService(t *testing.T) taggingServiceTestSuite {
	var ts taggingServiceTestSuite
	ts.repo = tagging_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.bus = event_mocks.NewMockBus(t)
	ts.authz = rbac_mocks.NewMockAuthorizationService(t)

	ts.service = tagging_service.NewService(
		ts.repo,
		ts.audit,
		ts.bus,
		ts.authz,
		&log.NoOpLogger{},
	)
	return ts
}

func contextWithActor(actorID uuid.UUID) context.Context {
	actor := &auth.Actor{AccountID: actorID, UserID: actorID}
	return auth.NewContextWithActor(context.Background(), actor)
}

func TestService_CreateTag(t *testing.T) {
	actorID := uuid.New()
	ctx := contextWithActor(actorID)

	t.Run("success", func(t *testing.T) {
		ts := setupTaggingService(t)
		tagName := "Go Language"
		tagSlug := "go-language"
		tagType := tagging.TypeSkill

		// Use WaitGroup for async audit
		var wg sync.WaitGroup
		wg.Add(1)

		ts.authz.EXPECT().Authorize(ctx, mock.MatchedBy(func(c rbac.PermissionCheck) bool {
			return c.PermissionName == tagging.PermTagManage
		})).Return(true, nil).Once()

		ts.repo.EXPECT().GetTagBySlug(ctx, tagSlug, tagType).Return(nil, tagging.ErrTagNotFound).Once()

		ts.repo.EXPECT().CreateTag(ctx, mock.MatchedBy(func(tag *tagging.Tag) bool {
			return tag.Name == tagName && tag.Slug == tagSlug
		})).Return(nil).Once()

		// Expect Audit and signal WaitGroup
		ts.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventTagCreated
		})).Run(func(ctx context.Context, e audit.Event) {
			wg.Done()
		}).Return(nil).Once()

		tag, err := ts.service.CreateTag(ctx, tagName, tagType)
		require.NoError(t, err)
		assert.Equal(t, tagName, tag.Name)

		wg.Wait() // Wait for async audit
	})

	t.Run("idempotent return if exists", func(t *testing.T) {
		ts := setupTaggingService(t)
		tagName := "Existing"
		tagSlug := "existing"
		existingTag := &tagging.Tag{ID: uuid.New(), Name: tagName}

		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetTagBySlug(ctx, tagSlug, tagging.TypeSkill).Return(existingTag, nil).Once()

		tag, err := ts.service.CreateTag(ctx, tagName, tagging.TypeSkill)
		require.NoError(t, err)
		assert.Equal(t, existingTag.ID, tag.ID)
		ts.repo.AssertNotCalled(t, "CreateTag")
	})
}

func TestService_MergeTags(t *testing.T) {
	actorID := uuid.New()
	ctx := contextWithActor(actorID)
	sourceID := uuid.New()
	targetID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ts := setupTaggingService(t)
		var wg sync.WaitGroup
		wg.Add(1) // For Audit

		sourceTag := &tagging.Tag{ID: sourceID, Name: "Old", Count: 5}
		targetTag := &tagging.Tag{ID: targetID, Name: "New", Count: 10}

		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetTagByID(ctx, sourceID).Return(sourceTag, nil).Once()
		ts.repo.EXPECT().GetTagByID(ctx, targetID).Return(targetTag, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(tagging.Repository) error) {
			txRepo := tagging_mocks.NewMockRepository(t)
			txRepo.EXPECT().ReassignTaggings(ctx, sourceID, targetID).Return(nil).Once()
			txRepo.EXPECT().UpdateTagCount(ctx, targetID, 5).Return(nil).Once()
			txRepo.EXPECT().DeleteTag(ctx, sourceID).Return(nil).Once()
			fn(txRepo)
		}).Return(nil).Once()

		// Async Audit
		ts.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventTagsMerged
		})).Run(func(ctx context.Context, e audit.Event) {
			wg.Done()
		}).Return(nil).Once()

		// Async EventBus (We can relax this or mock it similarly if needed)
		ts.bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()

		err := ts.service.MergeTags(ctx, sourceID, targetID)
		require.NoError(t, err)

		// Wait with timeout to prevent deadlock if callback never runs
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for audit record")
		}
	})
}
