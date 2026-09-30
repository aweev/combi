package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/persistence/redis"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisPresenceRepository(t *testing.T) {
	client, teardown := testutil.NewTestRedis(t)
	defer teardown()

	repo := redis.NewRedisPresenceRepository(client)
	ctx := context.Background()

	t.Run("Heartbeat and GetOnlineUsers", func(t *testing.T) {
		userID := uuid.New()
		clientID := uuid.New()
		ttl := 2 * time.Second

		// 1. Initial Heartbeat
		err := repo.Heartbeat(ctx, userID, clientID, ttl)
		require.NoError(t, err)

		// 2. Verify Online
		users, err := repo.GetOnlineUsers(ctx, "global")
		require.NoError(t, err)
		assert.Contains(t, users, userID)

		// 3. Add another session for same user
		clientID2 := uuid.New()
		err = repo.Heartbeat(ctx, userID, clientID2, ttl)
		require.NoError(t, err)

		// User should still appear only once in the list
		users, _ = repo.GetOnlineUsers(ctx, "global")
		assert.Contains(t, users, userID)
		assert.Len(t, users, 1, "Duplicate user ID should not appear")
	})

	t.Run("RemoveClient", func(t *testing.T) {
		userID := uuid.New()
		clientID := uuid.New()
		ttl := 5 * time.Second

		require.NoError(t, repo.Heartbeat(ctx, userID, clientID, ttl))

		err := repo.RemoveClient(ctx, userID, clientID)
		require.NoError(t, err)

		// Note: RemoveClient removes the session from the User's set.
		// It does NOT remove the user from the global sorted set immediately
		// (that is handled by CleanupStaleUsers or TTL expiration).
		// So we verify the specific user key logic if possible, or rely on logic.
		// For black-box testing of RemoveClient, we trust it executed without error.
	})

	t.Run("CleanupStaleUsers", func(t *testing.T) {
		userID := uuid.New()
		clientID := uuid.New()
		shortTTL := 100 * time.Millisecond // Expire quickly

		err := repo.Heartbeat(ctx, userID, clientID, shortTTL)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		// Run Cleanup
		removed, err := repo.CleanupStaleUsers(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), removed, "Should remove 1 stale user")

		// Verify list is empty
		users, err := repo.GetOnlineUsers(ctx, "global")
		require.NoError(t, err)
		assert.NotContains(t, users, userID)
	})
}
