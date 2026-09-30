// internal/services/automation/ingest_test.go
package automation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
)

func TestIngestService(t *testing.T) {
	ctx := context.Background()
	event := automation.Event{
		Type: "user.created",
		Data: map[string]any{"name": "Alice"},
	}

	t.Run("should dispatch event to queue successfully", func(t *testing.T) {
		// Arrange
		mockDispatcher := queue_mocks.NewMockDispatcher(t)
		service := NewIngestService(mockDispatcher)

		// Expect Dispatch to be called with the correct job type and queue
		mockDispatcher.EXPECT().Dispatch(
			ctx,
			automation.JobAutomationProcessEvent,
			mock.AnythingOfType("json.RawMessage"),
			mock.AnythingOfType("queue.DispatchOption"), // For WithQueue
		).Run(func(_ context.Context, _ core.JobType, payload json.RawMessage, opts ...queue.DispatchOption) {
			// Optional: Inspect the payload to ensure it's correct
			var dispatchedEvent automation.Event
			err := json.Unmarshal(payload, &dispatchedEvent)
			require.NoError(t, err)
			assert.Equal(t, event.Type, dispatchedEvent.Type)
			assert.NotEqual(t, uuid.Nil, dispatchedEvent.ID) // Ensure ID was set
		}).Return(nil, nil).Once()

		// Act
		err := service.Ingest(ctx, event)

		// Assert
		require.NoError(t, err)
	})

	t.Run("should return error if dispatcher fails", func(t *testing.T) {
		// Arrange
		mockDispatcher := queue_mocks.NewMockDispatcher(t)
		service := NewIngestService(mockDispatcher)
		dispatchErr := errors.New("queue is down")

		mockDispatcher.EXPECT().Dispatch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil, dispatchErr).Once()

		// Act
		err := service.Ingest(ctx, event)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, dispatchErr)
	})
}
