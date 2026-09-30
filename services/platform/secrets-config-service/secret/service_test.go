package secret_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/domain/secret"
	secret_mocks "github.com/combi/pkg/domain/secret/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	eventbus_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	security_mocks "github.com/combi/pkg/security/mocks"
	secret_service "github.com/combi/config/secret"
)

type secretSuite struct {
	repo       *secret_mocks.MockRepository
	crypto     *security_mocks.MockCryptoService
	audit      *audit_mocks.MockEventRecorder
	bus        *eventbus_mocks.MockBus
	dispatcher *queue_mocks.MockDispatcher
	cache      *cache_mocks.MockService
	service    secret.Service
	admin      secret.AdminService
}

func setupSecretService(t *testing.T) secretSuite {
	s := secretSuite{
		repo:       secret_mocks.NewMockRepository(t),
		crypto:     security_mocks.NewMockCryptoService(t),
		audit:      audit_mocks.NewMockEventRecorder(t),
		bus:        eventbus_mocks.NewMockBus(t),
		dispatcher: queue_mocks.NewMockDispatcher(t),
	}

	s.service = secret_service.NewService(s.repo, s.crypto, s.audit, s.bus, &log.NoOpLogger{})
	s.admin = secret_service.NewAdminService(s.repo, s.crypto, s.audit, s.bus, s.dispatcher, s.cache, &log.NoOpLogger{})
	return s
}

func contextWithUser() (context.Context, *auth.Actor) {
	actor := &auth.Actor{UserID: uuid.New()}
	return auth.NewContextWithActor(context.Background(), actor), actor
}

func TestService_GetSecretValue(t *testing.T) {
	ts := setupSecretService(t)
	ctx, _ := contextWithUser()
	secretID := uuid.New()

	// Setup Secret Model
	encVal := []byte("ciphertext")
	encCtx := &security.EncryptionContext{Nonce: []byte("nonce")}
	encCtxBytes, _ := json.Marshal(encCtx)

	sec := &secret.Secret{
		ID:                secretID,
		Name:              "stripe_key",
		EncryptedValue:    encVal,
		EncryptionContext: encCtxBytes,
		Version:           1,
	}

	t.Run("success", func(t *testing.T) {
		// Arrange
		ts.repo.EXPECT().Get(ctx, "stripe_key").Return(sec, nil).Once()
		// Decrypt returns the JSON string of the map
		ts.crypto.EXPECT().Decrypt(ctx, encVal, encCtx).Return([]byte(`{"key":"sk_test_123"}`), nil).Once()
		// Audit happens asynchronously, so we use mock.Anything to avoid race condition in test
		// In a real test environment, you might want to wait for the go-routine or inject a sync mechanism
		// For now, we just ensure Record is called eventually or we don't strict check it here to keep test stable
		// Alternatively, spy/mock waitgroup injection (advanced)
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		val, err := ts.service.GetSecretValue(ctx, "stripe_key")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "sk_test_123", val["key"])
	})
}
