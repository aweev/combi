// Ported from api/internal/services/audit/store_worker_test.go.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

func TestStoreWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	jobID := uuid.New()

	// Helper to create a valid audit event and its corresponding job
	createValidJob := func() (*audit.Event, *queue.Job) {
		event := &audit.Event{
			ID:      uuid.New(),
			ActorID: uuid.New(),
			Action:  "user.created",
		}
		payload, err := json.Marshal(event)
		require.NoError(t, err)

		return event, &queue.Job{
			ID:      jobID,
			Type:    audit.JobAuditStoreEvent,
			Payload: payload,
		}
	}

	t.Run("should successfully store a valid event", func(t *testing.T) {
		// Arrange
		repo := audit_mocks.NewMockEventRepository(t)
		worker := NewStoreWorker(repo, &log.NoOpLogger{})
		event, job := createValidJob()

		// Expect the repository to be called with a slice containing our event
		repo.EXPECT().
			CreateBatchEvents(ctx, mock.MatchedBy(func(events []*audit.Event) bool {
				return len(events) == 1 && events[0].ID == event.ID
			})).
			Return(nil).
			Once()

		// Act
		result, err := worker.ProcessJob(ctx, job)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, result, "result should be nil on success")
	})

	t.Run("should return non-retryable error for invalid payload", func(t *testing.T) {
		// Arrange
		repo := audit_mocks.NewMockEventRepository(t)
		worker := NewStoreWorker(repo, &log.NoOpLogger{})
		job := &queue.Job{
			ID:      jobID,
			Type:    audit.JobAuditStoreEvent,
			Payload: []byte("{not-a-valid-json"),
		}

		// Act
		_, err := worker.ProcessJob(ctx, job)

		// Assert
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable), "error should be non-retryable for malformed payload")
	})

	t.Run("should return retryable error if repository fails", func(t *testing.T) {
		// Arrange
		repo := audit_mocks.NewMockEventRepository(t)
		worker := NewStoreWorker(repo, &log.NoOpLogger{})
		_, job := createValidJob()
		repoErr := errors.New("database connection timeout")

		// Expect the repository call to fail
		repo.EXPECT().
			CreateBatchEvents(ctx, mock.AnythingOfType("[]*audit.Event")).
			Return(repoErr).
			Once()

		// Act
		_, err := worker.ProcessJob(ctx, job)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr, "the original repository error should be returned")

		// Verify it's a standard, retryable error
		var nonRetryable *queue.NonRetryableError
		assert.False(t, errors.As(err, &nonRetryable))
	})
}
