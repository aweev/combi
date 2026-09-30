// Ported from api/internal/services/audit/recorder_test.go.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	security_mocks "github.com/combi/pkg/security/mocks"
)

type asyncRecorderTestSuite struct {
	cryptoSvc *security_mocks.MockCryptoService
	enqueuer  *queue_mocks.MockJobEnqueuer
	recorder  audit.EventRecorder
}

func setupAsyncRecorder(t *testing.T) asyncRecorderTestSuite {
	var ts asyncRecorderTestSuite
	ts.cryptoSvc = security_mocks.NewMockCryptoService(t)
	ts.enqueuer = queue_mocks.NewMockJobEnqueuer(t)
	ts.recorder = NewAsyncEventRecorder(
		ts.cryptoSvc,
		ts.enqueuer,
		&log.NoOpLogger{},
	)
	return ts
}

func TestAsyncEventRecorder_Record(t *testing.T) {
	ts := setupAsyncRecorder(t)
	ctx := context.Background()

	event := audit.Event{
		ID:      uuid.New(),
		ActorID: uuid.New(),
		Action:  core.EventAction("user.login"),
		Sensitive: audit.SensitiveData{
			ActorName: "alice@example.com",
		},
	}

	plaintext, _ := json.Marshal(event.Sensitive)
	ciphertext := []byte("encrypted")
	encContext := &security.EncryptionContext{
		Nonce: []byte("nonce123"), // 12 bytes is typical for AES-GCM
	}

	t.Run("happy path", func(t *testing.T) {
		ts.cryptoSvc.EXPECT().
			Encrypt(ctx, plaintext).
			Return(ciphertext, encContext, nil).
			Once()

		ts.enqueuer.EXPECT().
			EnqueueJob(ctx, mock.MatchedBy(func(job *queue.Job) bool {
				return job.Type == audit.JobAuditStoreEvent &&
					job.Queue == "critical" &&
					job.Priority == 0 &&
					job.MaxAttempts == 5
			})).
			Return(nil).
			Once()

		err := ts.recorder.Record(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("encryption failure", func(t *testing.T) {
		ts.cryptoSvc.EXPECT().
			Encrypt(ctx, plaintext).
			Return(nil, nil, errors.New("crypto down")).
			Once()

		// Enqueue should NOT be called
		err := ts.recorder.Record(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("marshal failure", func(t *testing.T) {
		badEvent := audit.Event{
			ID:        uuid.New(),
			Sensitive: audit.SensitiveData{Metadata: json.RawMessage(`{invalid}`)},
		}

		err := ts.recorder.Record(ctx, badEvent)
		assert.NoError(t, err)
	})

	t.Run("enqueue failure", func(t *testing.T) {
		ts.cryptoSvc.EXPECT().
			Encrypt(ctx, plaintext).
			Return(ciphertext, encContext, nil).
			Once()

		ts.enqueuer.EXPECT().
			EnqueueJob(ctx, mock.Anything).
			Return(errors.New("queue full")).
			Once()

		err := ts.recorder.Record(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("context cancelled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()

		// Encrypt should still be attempted
		ts.cryptoSvc.EXPECT().
			Encrypt(cctx, plaintext).
			Return(ciphertext, encContext, nil).
			Once()

		// Enqueue should still be attempted (context error is logged, not returned)
		ts.enqueuer.EXPECT().
			EnqueueJob(cctx, mock.Anything).
			Return(context.Canceled).
			Once()

		err := ts.recorder.Record(cctx, event)
		assert.NoError(t, err)
	})
}
