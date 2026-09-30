package realtime_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/log"
	rt_service "github.com/combi/realtime"
)

func TestRBACAuthorizer_CanSubscribe(t *testing.T) {
	mockRBAC := rbac_mocks.NewMockAuthorizationService(t)
	logger := &log.NoOpLogger{}
	authz := rt_service.NewRBACAuthorizer(mockRBAC, logger)
	ctx := context.Background()

	userID := uuid.New()

	t.Run("Public channel allows anonymous", func(t *testing.T) {
		// No UserID provided
		allowed, err := authz.CanSubscribe(ctx, nil, "public:stats")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("Private channel denies anonymous", func(t *testing.T) {
		allowed, err := authz.CanSubscribe(ctx, nil, "private:chat")
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("User channel allows matching owner", func(t *testing.T) {
		channel := realtime.ChannelID("user:" + userID.String())
		allowed, err := authz.CanSubscribe(ctx, &userID, channel)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("User channel denies mismatching owner", func(t *testing.T) {
		otherUser := uuid.New()
		channel := realtime.ChannelID("user:" + otherUser.String())
		allowed, err := authz.CanSubscribe(ctx, &userID, channel)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("Gated channel delegates to RBAC service", func(t *testing.T) {
		channel := realtime.ChannelID("admin:dashboard")
		expectedPerm := core.PermissionName("realtime:subscribe:admin:dashboard")

		// Expect RBAC call
		mockRBAC.EXPECT().Authorize(ctx, mock.MatchedBy(func(c rbac.PermissionCheck) bool {
			return c.ActorID == userID && c.PermissionName == expectedPerm
		})).Return(true, nil).Once()

		allowed, err := authz.CanSubscribe(ctx, &userID, channel)
		require.NoError(t, err)
		assert.True(t, allowed)
	})
}
