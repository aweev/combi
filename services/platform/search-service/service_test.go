package search

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/domain/search"
	search_mocks "github.com/combi/pkg/domain/search/mocks"
	"github.com/combi/pkg/log"
)

func TestService_Search(t *testing.T) {
	mockEngine := new(search_mocks.MockEngine)
	mockDispatcher := mocks.NewMockDispatcher(t)

	// We don't mock RBAC service here because logic is inside resolveViewerScopes helper
	// which depends on inputs, not external calls (except if we fetched roles, which we assume are passed).
	svc := NewService(mockEngine, nil, mockDispatcher, &log.NoOpLogger{})

	ctx := context.Background()

	t.Run("resolves public scopes", func(t *testing.T) {
		req := search.SearchParams{
			Query: "test",
		}

		mockEngine.On("Search", ctx, mock.MatchedBy(func(q search.SearchQuery) bool {
			return len(q.ViewerScopes) == 1 && q.ViewerScopes[0] == "public"
		})).Return(&search.SearchResponse{}, nil).Once()

		_, err := svc.Search(ctx, req)
		require.NoError(t, err)
	})

	t.Run("resolves authenticated scopes", func(t *testing.T) {
		uid := uuid.New()
		req := search.SearchParams{
			Query:       "test",
			ViewerID:    uid,
			ViewerRoles: []string{"admin"},
		}

		mockEngine.On("Search", ctx, mock.MatchedBy(func(q search.SearchQuery) bool {
			// Expect: public, user:UUID, role:admin
			return len(q.ViewerScopes) == 3 &&
				assert.Contains(t, q.ViewerScopes, "public") &&
				assert.Contains(t, q.ViewerScopes, "role:admin")
		})).Return(&search.SearchResponse{}, nil).Once()

		_, err := svc.Search(ctx, req)
		require.NoError(t, err)
	})
}

func TestService_TriggerRebuild(t *testing.T) {
	mockDispatcher := mocks.NewMockDispatcher(t)
	svc := NewService(nil, nil, mockDispatcher, &log.NoOpLogger{})

	// Register a dummy provider
	svc.RegisterProvider(search.IndexUsers, nil)

	t.Run("dispatches job for registered provider", func(t *testing.T) {
		mockDispatcher.EXPECT().Dispatch(
			mock.Anything,
			search.JobRebuildIndex,
			mock.Anything,
			mock.Anything, // Priority opts
		).Return(nil, nil).Once()

		err := svc.TriggerRebuild(context.Background(), search.IndexUsers)
		require.NoError(t, err)
	})

	t.Run("fails for unknown index", func(t *testing.T) {
		err := svc.TriggerRebuild(context.Background(), "unknown_index")
		assert.Error(t, err)
	})
}
