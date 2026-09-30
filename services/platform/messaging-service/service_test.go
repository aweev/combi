package messaging_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/messaging"
	messaging_mocks "github.com/combi/pkg/domain/messaging/mocks"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	messaging_svc "github.com/combi/messaging"
)

type testSuite struct {
	repo     *messaging_mocks.MockRepository
	userSvc  *user_mocks.MockService
	eventBus *events_mocks.MockBus
	service  messaging.Service
}

func setup(t *testing.T) testSuite {
	repo := messaging_mocks.NewMockRepository(t)
	userSvc := user_mocks.NewMockService(t)
	eventBus := events_mocks.NewMockBus(t)
	svc := messaging_svc.NewService(repo, userSvc, eventBus, &log.NoOpLogger{})

	return testSuite{repo, userSvc, eventBus, svc}
}

func TestService_CreateDirectChannel(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	u1 := uuid.New()
	u2 := uuid.New()

	t.Run("returns existing", func(t *testing.T) {
		existing := &messaging.Channel{ID: uuid.New(), Type: messaging.TypeDirect}
		ts.repo.EXPECT().FindDirectChannel(ctx, u1, u2).Return(existing, nil).Once()

		ch, err := ts.service.CreateDirectChannel(ctx, u1, u2)
		require.NoError(t, err)
		assert.Equal(t, existing.ID, ch.ID)
	})

	t.Run("creates new", func(t *testing.T) {
		ts.repo.EXPECT().FindDirectChannel(ctx, u1, u2).Return(nil, messaging.ErrChannelNotFound).Once()

		// Expect Create and Member Adds
		ts.repo.EXPECT().CreateChannel(ctx, mock.MatchedBy(func(c *messaging.Channel) bool {
			return c.Type == messaging.TypeDirect
		})).Return(nil).Once()

		ts.repo.EXPECT().AddMember(ctx, mock.MatchedBy(func(m *messaging.ChannelMember) bool {
			return m.UserID == u1
		})).Return(nil).Once()

		ts.repo.EXPECT().AddMember(ctx, mock.MatchedBy(func(m *messaging.ChannelMember) bool {
			return m.UserID == u2
		})).Return(nil).Once()

		ch, err := ts.service.CreateDirectChannel(ctx, u1, u2)
		require.NoError(t, err)
		assert.NotNil(t, ch.ID)
	})
}

func TestService_SendMessage(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	u1 := uuid.New()
	chID := uuid.New()

	t.Run("successful send", func(t *testing.T) {
		// 1. Verify Member
		ts.repo.EXPECT().GetMember(ctx, chID, u1).Return(&messaging.ChannelMember{}, nil).Once()

		// 2. Create Message
		ts.repo.EXPECT().CreateMessage(ctx, mock.MatchedBy(func(m *messaging.Message) bool {
			return m.Content == "Hello" && m.SenderID == u1
		})).Return(nil).Once()

		// 3. Emit Event
		ts.eventBus.EXPECT().Publish(ctx, mock.MatchedBy(func(evt interface{}) bool {
			// In real code check evt.Action == messaging.EventMessageCreated
			return true
		})).Return(nil).Once()

		msg, err := ts.service.SendMessage(ctx, chID, u1, "Hello", messaging.MsgText, nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello", msg.Content)
	})

	t.Run("forbidden if not member", func(t *testing.T) {
		ts.repo.EXPECT().GetMember(ctx, chID, u1).Return(nil, messaging.ErrNotMember).Once()

		_, err := ts.service.SendMessage(ctx, chID, u1, "Hello", messaging.MsgText, nil)
		assert.ErrorIs(t, err, messaging.ErrNotMember)
	})
}
