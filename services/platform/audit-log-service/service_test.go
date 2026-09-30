// Ported from api/internal/services/audit/service_test.go.
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
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	security_mocks "github.com/combi/pkg/security/mocks"
)

// serviceTestSuite encapsulates all dependencies for the Service tests.
type serviceTestSuite struct {
	repo       *audit_mocks.MockEventRepository
	cryptoSvc  *security_mocks.MockCryptoService
	dispatcher *queue_mocks.MockDispatcher
	service    audit.Service
}

// setupService creates a new test suite with fresh mocks.
func setupService(t *testing.T) serviceTestSuite {
	var ts serviceTestSuite
	ts.repo = audit_mocks.NewMockEventRepository(t)
	ts.cryptoSvc = security_mocks.NewMockCryptoService(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.service = NewService(
		ts.repo,
		ts.cryptoSvc,
		ts.dispatcher,
		&log.NoOpLogger{},
	)
	return ts
}

func TestService_GetEvent(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()
	id := uuid.New()
	event := &audit.Event{
		ID:                id,
		EncryptedData:     []byte("cipher"),
		EncryptionContext: &security.EncryptionContext{Nonce: []byte("nonce")},
	}

	t.Run("should fetch and decrypt event successfully", func(t *testing.T) {
		// Arrange
		ts.repo.EXPECT().GetEvent(ctx, id).Return(event, nil).Once()
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, event.EncryptedData, event.EncryptionContext).
			Return([]byte(`{"actor_name":"bob"}`), nil).
			Once()

		// Act
		got, err := ts.service.GetEvent(ctx, id)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "bob", got.Sensitive.ActorName)
	})

	t.Run("should return error when event not found", func(t *testing.T) {
		// Arrange
		ts.repo.EXPECT().GetEvent(ctx, id).Return(nil, audit.ErrEventNotFound).Once()

		// Act
		_, err := ts.service.GetEvent(ctx, id)

		// Assert
		assert.ErrorIs(t, err, audit.ErrEventNotFound)
	})

	t.Run("should return decryption failed error on crypto failure", func(t *testing.T) {
		// Arrange
		ts.repo.EXPECT().GetEvent(ctx, id).Return(event, nil).Once()
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, event.EncryptedData, event.EncryptionContext).
			Return(nil, errors.New("bad key")).
			Once()

		// Act
		_, err := ts.service.GetEvent(ctx, id)

		// Assert
		assert.ErrorIs(t, err, audit.ErrDecryptionFailed)
	})
}

func TestService_SearchEvents(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()
	filters := audit.EventFilters{Limit: 10}

	eventEncrypted := &audit.Event{
		ID:                uuid.New(),
		EncryptedData:     []byte("c1"),
		EncryptionContext: &security.EncryptionContext{Nonce: []byte("n1")},
	}
	eventPlain := &audit.Event{
		ID:            uuid.New(),
		EncryptedData: nil, // Not encrypted
	}

	t.Run("should search and decrypt events successfully", func(t *testing.T) {
		// Arrange
		ts.repo.EXPECT().SearchEvents(ctx, filters).Return([]*audit.Event{eventEncrypted, eventPlain}, 2, nil).Once()
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, eventEncrypted.EncryptedData, eventEncrypted.EncryptionContext).
			Return([]byte(`{"actor_name":"alice"}`), nil).
			Once()

		// Act
		list, total, err := ts.service.SearchEvents(ctx, filters)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
		assert.Equal(t, "alice", list[0].Sensitive.ActorName)
		assert.Empty(t, list[1].Sensitive.ActorName) // Plain event has no sensitive data
	})

	t.Run("should continue if one event fails decryption", func(t *testing.T) {
		// Arrange
		eventFailsDecrypt := &audit.Event{
			ID:                uuid.New(),
			EncryptedData:     []byte("bad-cipher"),
			EncryptionContext: &security.EncryptionContext{Nonce: []byte("n-bad")},
		}
		ts.repo.EXPECT().SearchEvents(ctx, filters).
			Return([]*audit.Event{eventEncrypted, eventFailsDecrypt}, 2, nil).Once()
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, eventEncrypted.EncryptedData, eventEncrypted.EncryptionContext).
			Return([]byte(`{"actor_name":"alice"}`), nil).
			Once()
		ts.cryptoSvc.EXPECT().
			Decrypt(ctx, eventFailsDecrypt.EncryptedData, eventFailsDecrypt.EncryptionContext).
			Return(nil, errors.New("decrypt failed")).
			Once()

		// Act
		list, total, err := ts.service.SearchEvents(ctx, filters)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
		assert.Equal(t, "alice", list[0].Sensitive.ActorName)
		assert.Empty(t, list[1].Sensitive.ActorName, "Sensitive data should be empty on decrypt failure")
	})

	t.Run("should return error if repository search fails", func(t *testing.T) {
		// Arrange
		dbErr := errors.New("database is down")
		ts.repo.EXPECT().SearchEvents(ctx, filters).Return(nil, 0, dbErr).Once()

		// Act
		_, _, err := ts.service.SearchEvents(ctx, filters)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}

func TestService_ExportEvents(t *testing.T) {
	ts := setupService(t)
	filters := audit.EventFilters{Limit: 100}
	actor := &auth.Actor{AccountID: uuid.New()}
	ctx := auth.NewContextWithActor(context.Background(), actor)

	t.Run("should dispatch export job successfully", func(t *testing.T) {
		// Arrange
		job := &queue.Job{ID: uuid.New()}
		ts.dispatcher.EXPECT().
			Dispatch(ctx, audit.JobAuditExport, mock.AnythingOfType("json.RawMessage"), mock.AnythingOfType("queue.DispatchOption")).
			// We can also add a matcher to inspect the payload
			Run(func(_ context.Context, _ core.JobType, p json.RawMessage, _ ...queue.DispatchOption) {
				var data map[string]interface{}
				err := json.Unmarshal(p, &data)
				require.NoError(t, err)
				assert.Equal(t, "csv", data["format"])
				assert.Equal(t, actor.AccountID.String(), data["requester_id"])
			}).
			Return(job, nil).
			Once()

		// Act
		got, err := ts.service.ExportEvents(ctx, filters, "csv")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, job.ID, got.ID)
	})

	t.Run("should return unauthorized without an actor in context", func(t *testing.T) {
		// Act
		_, err := ts.service.ExportEvents(context.Background(), filters, "csv")

		// Assert
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})

	t.Run("should return error for unsupported format", func(t *testing.T) {
		// Act
		_, err := ts.service.ExportEvents(ctx, filters, "xml")

		// Assert
		assert.ErrorIs(t, err, audit.ErrUnsupportedExportFormat)
	})

	t.Run("should return error on dispatch failure", func(t *testing.T) {
		// Arrange
		dispatchErr := errors.New("queue is full")
		ts.dispatcher.EXPECT().
			Dispatch(ctx, audit.JobAuditExport, mock.Anything, mock.Anything).
			Return(nil, dispatchErr).
			Once()

		// Act
		_, err := ts.service.ExportEvents(ctx, filters, "csv")

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, dispatchErr)
	})

	t.Run("should return not-configured error without a dispatcher", func(t *testing.T) {
		// Arrange: a service built with a nil dispatcher (no queue host
		// mounted in this fx graph yet) must fail honestly, not panic.
		svc := NewService(ts.repo, ts.cryptoSvc, nil, &log.NoOpLogger{})

		// Act
		_, err := svc.ExportEvents(ctx, filters, "csv")

		// Assert
		assert.ErrorIs(t, err, ErrDispatcherNotConfigured)
	})
}
