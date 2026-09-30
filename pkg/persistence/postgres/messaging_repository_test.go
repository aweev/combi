package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessagingRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewMessagingRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	u1 := uuid.New()
	u2 := uuid.New()

	t.Run("Create and Find Direct Channel", func(t *testing.T) {
		// 1. Create Channel
		channelID := uuid.New()
		ch := &messaging.Channel{
			ID:   channelID,
			Type: messaging.TypeDirect,
		}
		require.NoError(t, repo.CreateChannel(ctx, ch))

		// 2. Add Members
		require.NoError(t, repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: channelID, UserID: u1, Role: "member"}))
		require.NoError(t, repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: channelID, UserID: u2, Role: "member"}))

		// 3. Find It
		found, err := repo.FindDirectChannel(ctx, u1, u2)
		require.NoError(t, err)
		assert.Equal(t, channelID, found.ID)

		// 4. Negative Test (Wrong Pair)
		u3 := uuid.New()
		_, err = repo.FindDirectChannel(ctx, u1, u3)
		assert.ErrorIs(t, err, messaging.ErrChannelNotFound)
	})

	t.Run("Message Lifecycle and Pagination", func(t *testing.T) {
		// Setup
		chID := uuid.New()
		require.NoError(t, repo.CreateChannel(ctx, &messaging.Channel{ID: chID, Type: messaging.TypeGroup, Name: Ptr("Group")}))
		require.NoError(t, repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: chID, UserID: u1}))

		// 1. Send Messages
		msg1 := &messaging.Message{ChannelID: chID, SenderID: u1, Content: "First"}
		msg2 := &messaging.Message{ChannelID: chID, SenderID: u1, Content: "Second"}
		msg3 := &messaging.Message{ChannelID: chID, SenderID: u1, Content: "Third"}

		// Sleep slightly to ensure timestamps differ for simple testing (though tuple comparison handles equality)
		require.NoError(t, repo.CreateMessage(ctx, msg1))
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, repo.CreateMessage(ctx, msg2))
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, repo.CreateMessage(ctx, msg3))

		// 2. List (Latest first)
		msgs, err := repo.ListMessages(ctx, chID, nil, 2)
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, msg3.ID, msgs[0].ID)
		assert.Equal(t, msg2.ID, msgs[1].ID)

		// 3. Pagination (Cursor)
		cursor := msgs[1].ID // msg2
		nextPage, err := repo.ListMessages(ctx, chID, &cursor, 2)
		require.NoError(t, err)
		require.Len(t, nextPage, 1)
		assert.Equal(t, msg1.ID, nextPage[0].ID)

		// 4. Soft Delete
		require.NoError(t, repo.SoftDeleteMessage(ctx, msg1.ID))
		_, err = repo.GetMessage(ctx, msg1.ID)
		assert.ErrorIs(t, err, messaging.ErrMessageNotFound)
	})

	t.Run("Channel Summaries & Unread Counts", func(t *testing.T) {
		// FIX: Use a fresh user for this test to ensure isolation from previous tests
		u3 := uuid.New()
		u4 := uuid.New() // Sender

		chID := uuid.New()
		require.NoError(t, repo.CreateChannel(ctx, &messaging.Channel{ID: chID, Type: messaging.TypeGroup}))

		// User joins
		joinTime := time.Now().Add(-1 * time.Hour)
		require.NoError(t, repo.AddMember(ctx, &messaging.ChannelMember{
			ChannelID: chID, UserID: u3, LastReadAt: joinTime, JoinedAt: joinTime,
		}))

		// Send 3 messages from OTHER user
		for i := 0; i < 3; i++ {
			require.NoError(t, repo.CreateMessage(ctx, &messaging.Message{
				ChannelID: chID, SenderID: u4, Content: "Hi",
			}))
		}
		// Send 1 message from SELF (should not count as unread)
		require.NoError(t, repo.CreateMessage(ctx, &messaging.Message{
			ChannelID: chID, SenderID: u3, Content: "Self",
		}))

		// Fetch Summary for u3
		summaries, err := repo.ListChannelsForUser(ctx, u3, 10, 0)
		require.NoError(t, err)
		require.Len(t, summaries, 1) // Now this will be true

		// Expect 3 unread (u4's messages), ignoring u3's own message
		assert.Equal(t, 3, summaries[0].UnreadCount)

		// Mark Read
		require.NoError(t, repo.UpdateLastRead(ctx, chID, u3, time.Now()))

		// Fetch again
		summaries, err = repo.ListChannelsForUser(ctx, u3, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, summaries[0].UnreadCount)
	})
}

func Ptr[T any](v T) *T { return &v }
