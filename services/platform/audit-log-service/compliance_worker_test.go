// Ported from api/internal/services/audit/compliance_worker_test.go.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/audit"
	privacy_mocks "github.com/combi/pkg/domain/privacy/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

func TestComplianceWorker_ProcessJob(t *testing.T) {
	reqID := uuid.New()
	jobID := uuid.New()
	ctx := context.Background()

	createJob := func(payload interface{}) *queue.Job {
		p, _ := json.Marshal(payload)
		return &queue.Job{
			ID:      jobID,
			Type:    audit.JobComplianceAnonymizeUser,
			Payload: p,
		}
	}

	t.Run("should succeed when manager processes erasure", func(t *testing.T) {
		mockManager := privacy_mocks.NewMockManager(t)
		worker := NewComplianceWorker(&log.NoOpLogger{}, mockManager)

		job := createJob(map[string]string{"request_id": reqID.String()})

		mockManager.EXPECT().ProcessErasure(ctx, reqID).Return(nil).Once()

		result, err := worker.ProcessJob(ctx, job)

		require.NoError(t, err)
		var resultMap map[string]string
		_ = json.Unmarshal(result, &resultMap)
		assert.Equal(t, "completed", resultMap["status"])
	})

	t.Run("should return retryable error if manager fails", func(t *testing.T) {
		mockManager := privacy_mocks.NewMockManager(t)
		worker := NewComplianceWorker(&log.NoOpLogger{}, mockManager)

		job := createJob(map[string]string{"request_id": reqID.String()})
		failErr := errors.New("db error")

		mockManager.EXPECT().ProcessErasure(ctx, reqID).Return(failErr).Once()

		_, err := worker.ProcessJob(ctx, job)
		require.Error(t, err)
		assert.ErrorIs(t, err, failErr)
	})

	t.Run("should return non-retryable error for malformed payload", func(t *testing.T) {
		mockManager := privacy_mocks.NewMockManager(t) // No calls expected
		worker := NewComplianceWorker(&log.NoOpLogger{}, mockManager)
		job := &queue.Job{
			ID:      jobID,
			Type:    audit.JobComplianceAnonymizeUser,
			Payload: []byte(`invalid-json`),
		}

		_, err := worker.ProcessJob(ctx, job)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		require.True(t, errors.As(err, &nonRetryable))
	})

	t.Run("should return non-retryable error when manager is not configured", func(t *testing.T) {
		// The privacy module is not mounted in this host yet: the worker
		// must fail honestly (non-retryable), not panic on a nil manager.
		worker := NewComplianceWorker(&log.NoOpLogger{}, nil)

		job := createJob(map[string]string{"request_id": reqID.String()})

		_, err := worker.ProcessJob(ctx, job)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPrivacyManagerNotConfigured)
		var nonRetryable *queue.NonRetryableError
		require.True(t, errors.As(err, &nonRetryable))
	})
}
