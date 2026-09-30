package secret_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/secret"
	eventbus_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	secret_service "github.com/combi/config/secret"

	// Mocks
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	secret_mocks "github.com/combi/pkg/domain/secret/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	security_mocks "github.com/combi/pkg/security/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type adminSuite struct {
	repo       *secret_mocks.MockRepository
	crypto     *security_mocks.MockCryptoService
	audit      *audit_mocks.MockEventRecorder
	eventBus   *eventbus_mocks.MockBus
	dispatcher *queue_mocks.MockDispatcher
	cache      *cache_mocks.MockService
	service    secret.AdminService
}

func setupAdmin(t *testing.T) adminSuite {
	s := adminSuite{
		repo:       secret_mocks.NewMockRepository(t),
		crypto:     security_mocks.NewMockCryptoService(t),
		audit:      audit_mocks.NewMockEventRecorder(t),
		eventBus:   eventbus_mocks.NewMockBus(t),
		dispatcher: queue_mocks.NewMockDispatcher(t),
		// Fix: Initialize the cache mock
		cache: cache_mocks.NewMockService(t),
	}
	s.service = secret_service.NewAdminService(
		s.repo, s.crypto, s.audit, s.eventBus, s.dispatcher, s.cache, &log.NoOpLogger{},
	)
	return s
}

func createAuthContext() (context.Context, *auth.Actor) {
	actor := &auth.Actor{UserID: uuid.New()}
	return auth.NewContextWithActor(context.Background(), actor), actor
}

func TestAdminService_CreateSecret(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ts := setupAdmin(t)
		ctx, actor := createAuthContext()

		params := secret.CreateSecretParams{
			Name:  "stripe_api_key",
			Value: map[string]interface{}{"key": "sk_test_123"},
		}

		// 1. Expect Encryption
		ts.crypto.EXPECT().Encrypt(ctx, mock.Anything).Return([]byte("encrypted"), &security.EncryptionContext{}, nil).Once()

		// 2. Expect Repository Save
		ts.repo.EXPECT().Set(ctx, mock.MatchedBy(func(s *secret.Secret) bool {
			return s.Name == params.Name && s.Version == 1 && string(s.EncryptedValue) == "encrypted"
		})).Return(nil).Once()

		// 3. Expect Audit & Event (Async)
		ts.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == secret.EventSecretCreated && e.ActorID == actor.UserID
		})).Return(nil).Maybe()

		ts.eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()

		res, err := ts.service.CreateSecret(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, params.Name, res.Name)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		ts := setupAdmin(t)
		_, err := ts.service.CreateSecret(context.Background(), secret.CreateSecretParams{})
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})
}

func TestAdminService_UpdateSecret(t *testing.T) {
	t.Run("Success with Value Change", func(t *testing.T) {
		ts := setupAdmin(t)
		ctx, _ := createAuthContext()
		key := "db_pass"
		cacheKey := "secret:" + key

		existing := &secret.Secret{ID: uuid.New(), Name: key, Version: 1}

		ts.repo.EXPECT().Get(ctx, key).Return(existing, nil).Once()
		ts.crypto.EXPECT().Encrypt(ctx, mock.Anything).Return([]byte("new_enc"), &security.EncryptionContext{}, nil).Once()
		ts.repo.EXPECT().Set(ctx, mock.MatchedBy(func(s *secret.Secret) bool {
			return s.Version == 2 && string(s.EncryptedValue) == "new_enc"
		})).Return(nil).Once()

		// Fix: Expect Cache Invalidation
		ts.cache.EXPECT().Delete(ctx, cacheKey).Return(nil).Once()

		// Async calls
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()
		ts.eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()

		params := secret.UpdateSecretParams{
			Value: map[string]interface{}{"pass": "new123"},
		}
		res, err := ts.service.UpdateSecret(ctx, key, params)
		require.NoError(t, err)
		assert.Equal(t, 2, res.Version)
	})

	t.Run("Success Metadata Only (No Encryption)", func(t *testing.T) {
		ts := setupAdmin(t)
		ctx, _ := createAuthContext()
		key := "db_pass"
		cacheKey := "secret:" + key
		existing := &secret.Secret{ID: uuid.New(), Name: key, Version: 1}

		ts.repo.EXPECT().Get(ctx, key).Return(existing, nil).Once()
		// No Crypto call expected
		ts.repo.EXPECT().Set(ctx, mock.MatchedBy(func(s *secret.Secret) bool {
			return s.Version == 1 && s.Metadata["owner"] == "team-a"
		})).Return(nil).Once()

		// Fix: Expect Cache Invalidation
		ts.cache.EXPECT().Delete(ctx, cacheKey).Return(nil).Once()

		// Async calls
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()
		ts.eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()

		params := secret.UpdateSecretParams{
			Metadata: map[string]string{"owner": "team-a"},
		}
		res, err := ts.service.UpdateSecret(ctx, key, params)
		require.NoError(t, err)
		assert.Equal(t, 1, res.Version)
	})
}

func TestAdminService_RotateSecret(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ts := setupAdmin(t)
		ctx, _ := createAuthContext()
		key := "api_key"
		secID := uuid.New()
		existing := &secret.Secret{ID: secID, Name: key}

		ts.repo.EXPECT().Get(ctx, key).Return(existing, nil).Once()

		ts.dispatcher.EXPECT().Dispatch(
			ctx,
			secret.JobSecretRotate,
			mock.MatchedBy(func(payload json.RawMessage) bool {
				var p map[string]string
				if err := json.Unmarshal(payload, &p); err != nil {
					return false
				}
				return p["secret_name"] == key
			}),
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(&queue.Job{ID: uuid.New()}, nil).Once()

		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()

		res, err := ts.service.RotateSecret(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, key, res.Name)
	})
}
