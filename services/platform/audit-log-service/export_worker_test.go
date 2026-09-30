// Ported from api/internal/services/audit/export_worker_test.go.
package auditlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/notification"
	notification_mocks "github.com/combi/pkg/domain/notification/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	security_mocks "github.com/combi/pkg/security/mocks"
)

// exportWorkerTestSuite encapsulates all mocks for the ExportWorker tests.
type exportWorkerTestSuite struct {
	repo       *audit_mocks.MockEventRepository
	cryptoSvc  *security_mocks.MockCryptoService
	storageSvc *storage_mocks.MockService
	notifier   *notification_mocks.MockService
	worker     queue.JobProcessor
}

// setupExportWorker creates a new test suite with fresh mocks.
func setupExportWorker(t *testing.T) exportWorkerTestSuite {
	var ts exportWorkerTestSuite
	ts.repo = audit_mocks.NewMockEventRepository(t)
	ts.cryptoSvc = security_mocks.NewMockCryptoService(t)
	ts.storageSvc = storage_mocks.NewMockService(t)
	ts.notifier = notification_mocks.NewMockService(t)
	ts.worker = NewExportWorker(
		ts.repo,
		ts.cryptoSvc,
		ts.storageSvc,
		ts.notifier,
		&log.NoOpLogger{},
	)
	return ts
}

// createMockEvent is a helper to create complete mock events to avoid panics.
func createMockEvent(id, actorID uuid.UUID) *audit.Event {
	reason := "success"
	targetID := uuid.New()
	targetType := "document"
	return &audit.Event{
		ID:                id,
		ActorID:           actorID,
		OutcomeReason:     &reason,
		TargetID:          &targetID,
		TargetType:        &targetType,
		EncryptedData:     []byte("encrypted"),
		EncryptionContext: &security.EncryptionContext{},
	}
}

func TestExportWorker_ProcessJob_HappyPath(t *testing.T) {
	ts := setupExportWorker(t)
	ctx := context.Background()
	requesterID := uuid.New()
	eventID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"filters":      audit.EventFilters{},
		"format":       "csv",
		"requester_id": requesterID.String(),
	})
	job := &queue.Job{ID: uuid.New(), Payload: payload}

	sensitiveData := audit.SensitiveData{ActorName: "test@user.com"}
	sensitivePayload, _ := json.Marshal(sensitiveData)
	event := createMockEvent(eventID, requesterID)

	// mock.Anything handles changes in the filters struct across calls.
	ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return([]*audit.Event{event}, 1, nil).Once()
	ts.cryptoSvc.EXPECT().
		Decrypt(ctx, event.EncryptedData, event.EncryptionContext).
		Return(sensitivePayload, nil).Once()

	ts.storageSvc.EXPECT().
		UploadFile(ctx, mock.AnythingOfType("storage.UploadFileRequest"), mock.AnythingOfType("*bytes.Buffer")).
		Return(&storage.File{ID: uuid.New()}, nil).Once()

	ts.storageSvc.EXPECT().
		GetDownloadURL(ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("time.Duration")).
		Return(&storage.SignedURL{URL: "http://example.com/download"}, nil).Once()

	ts.notifier.EXPECT().
		Dispatch(ctx, mock.AnythingOfType("notification.DispatchRequest")).
		Return(&notification.Notification{}, nil).Once()

	result, err := ts.worker.ProcessJob(ctx, job)

	require.NoError(t, err)
	var resultMap map[string]string
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/download", resultMap["file_url"])
}

func TestExportWorker_ProcessJob_FailureScenarios(t *testing.T) {
	ctx := context.Background()
	requesterID := uuid.New()
	payload, _ := json.Marshal(map[string]interface{}{
		"filters":      audit.EventFilters{},
		"format":       "csv",
		"requester_id": requesterID.String(),
	})
	job := &queue.Job{ID: uuid.New(), Payload: payload}

	t.Run("should fail non-retryably on invalid payload", func(t *testing.T) {
		ts := setupExportWorker(t)
		invalidJob := &queue.Job{ID: uuid.New(), Payload: []byte("invalid-json")}
		_, err := ts.worker.ProcessJob(ctx, invalidJob)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable))
	})

	t.Run("should fail non-retryably on unsupported format", func(t *testing.T) {
		ts := setupExportWorker(t)
		unsupportedPayload, _ := json.Marshal(map[string]interface{}{
			"format": "xml", "requester_id": requesterID.String(),
		})
		unsupportedJob := &queue.Job{ID: uuid.New(), Payload: unsupportedPayload}
		_, err := ts.worker.ProcessJob(ctx, unsupportedJob)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable))
	})

	t.Run("should fail retryably if repo search fails", func(t *testing.T) {
		ts := setupExportWorker(t)
		repoErr := errors.New("db connection failed")
		ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return(nil, 0, repoErr)
		_, err := ts.worker.ProcessJob(ctx, job)
		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
	})

	t.Run("should continue export but log if one event fails to decrypt", func(t *testing.T) {
		ts := setupExportWorker(t)
		decryptErr := errors.New("bad key")
		eventOK := createMockEvent(uuid.New(), requesterID)
		eventOK.EncryptedData = nil
		eventFail := createMockEvent(uuid.New(), requesterID)

		// mock.Anything here as well: the filters struct mutates across calls.
		ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return([]*audit.Event{eventOK, eventFail}, 2, nil).Once()
		ts.cryptoSvc.EXPECT().Decrypt(ctx, eventFail.EncryptedData, eventFail.EncryptionContext).Return(nil, decryptErr).Once()
		ts.storageSvc.EXPECT().UploadFile(ctx, mock.Anything, mock.Anything).Return(&storage.File{ID: uuid.New()}, nil).Once()
		ts.storageSvc.EXPECT().GetDownloadURL(ctx, mock.AnythingOfType("uuid.UUID"), mock.Anything).Return(&storage.SignedURL{URL: "ok"}, nil).Once()
		ts.notifier.EXPECT().Dispatch(ctx, mock.Anything).Return(&notification.Notification{}, nil).Once()

		_, err := ts.worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("should fail retryably if storage upload fails", func(t *testing.T) {
		ts := setupExportWorker(t)
		storageErr := errors.New("s3 bucket not found")
		ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return([]*audit.Event{}, 0, nil).Once()
		ts.storageSvc.EXPECT().UploadFile(ctx, mock.Anything, mock.Anything).Return(nil, storageErr).Once()
		_, err := ts.worker.ProcessJob(ctx, job)
		require.Error(t, err)
		assert.ErrorIs(t, err, storageErr)
	})

	t.Run("should succeed but log error if notification fails", func(t *testing.T) {
		ts := setupExportWorker(t)
		notifierErr := errors.New("smtp server down")
		ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return([]*audit.Event{}, 0, nil).Once()
		ts.storageSvc.EXPECT().UploadFile(ctx, mock.Anything, mock.Anything).Return(&storage.File{ID: uuid.New()}, nil).Once()
		ts.storageSvc.EXPECT().GetDownloadURL(ctx, mock.AnythingOfType("uuid.UUID"), mock.Anything).Return(&storage.SignedURL{URL: "ok"}, nil).Once()
		ts.notifier.EXPECT().Dispatch(ctx, mock.Anything).Return(nil, notifierErr).Once()
		_, err := ts.worker.ProcessJob(ctx, job)
		require.NoError(t, err)
	})

	t.Run("should fail non-retryably when storage or notification not configured", func(t *testing.T) {
		// The storage/notification modules are not mounted in this host
		// yet: the worker must fail honestly, not panic on nil services.
		ts := setupExportWorker(t)
		worker := NewExportWorker(ts.repo, ts.cryptoSvc, nil, nil, &log.NoOpLogger{})

		_, err := worker.ProcessJob(ctx, job)
		require.Error(t, err)
		var nonRetryable *queue.NonRetryableError
		assert.True(t, errors.As(err, &nonRetryable))
	})
}

func TestExportWorker_DataWriter_Integration(t *testing.T) {
	t.Run("should produce valid CSV output", func(t *testing.T) {
		ts := setupExportWorker(t)
		ctx := context.Background()
		payload, _ := json.Marshal(map[string]interface{}{"format": "csv", "requester_id": uuid.New()})
		job := &queue.Job{ID: uuid.New(), Payload: payload}
		event := createMockEvent(uuid.New(), uuid.New())

		// mock.Anything: the filters struct mutates across calls.
		ts.repo.EXPECT().SearchEvents(ctx, mock.Anything).Return([]*audit.Event{event}, 1, nil).Once()
		// Decrypt expectation for the mock event's encrypted payload.
		sensitiveData := audit.SensitiveData{ActorName: "integration test"}
		sensitivePayload, _ := json.Marshal(sensitiveData)
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, mock.Anything, mock.Anything).
			Return(sensitivePayload, nil).
			Once()

		var capturedBuffer bytes.Buffer
		ts.storageSvc.EXPECT().
			UploadFile(ctx, mock.Anything, mock.Anything).
			Run(func(_ context.Context, _ storage.UploadFileRequest, r io.Reader) {
				io.Copy(&capturedBuffer, r)
			}).
			Return(&storage.File{ID: uuid.New()}, nil).
			Once()

		ts.storageSvc.EXPECT().GetDownloadURL(ctx, mock.AnythingOfType("uuid.UUID"), mock.Anything).Return(&storage.SignedURL{}, nil)
		ts.notifier.EXPECT().Dispatch(ctx, mock.Anything).Return(&notification.Notification{}, nil)

		_, err := ts.worker.ProcessJob(ctx, job)
		require.NoError(t, err)

		assert.Contains(t, capturedBuffer.String(), "id,created_at,actor_id,actor_type")
		assert.Contains(t, capturedBuffer.String(), event.ID.String())
	})
}
