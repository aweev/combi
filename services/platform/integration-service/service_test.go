package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	financial_mocks "github.com/combi/pkg/domain/financial/mocks"
	"github.com/combi/pkg/domain/integration"
	int_mocks "github.com/combi/pkg/domain/integration/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/log"
	security_mocks "github.com/combi/pkg/security/mocks"
	integration_service "github.com/combi/integration"
)

// --- Special Mocks ---

// MockTestableGateway combines PaymentGateway and ConnectionTester for testing.
// We define this struct manually or ensure mockery generates a mock that implements both.
// For simplicity in this manual test code, we'll extend the existing MockPaymentGateway.
type MockTestableGateway struct {
	financial_mocks.MockPaymentGateway
}

// Implement the extra method manually on the composed struct
func (m *MockTestableGateway) TestConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// --- Test Suite Setup ---

type serviceTestSuite struct {
	repo          *int_mocks.MockRepository
	crypto        *security_mocks.MockCryptoService
	factory       *int_mocks.MockProviderFactory
	auditRecorder *audit_mocks.MockEventRecorder
	dispatcher    *queue_mocks.MockDispatcher
	service       *integration_service.Service
}

func setupService(t *testing.T) serviceTestSuite {
	var ts serviceTestSuite
	ts.repo = int_mocks.NewMockRepository(t)
	ts.crypto = security_mocks.NewMockCryptoService(t)
	ts.factory = int_mocks.NewMockProviderFactory(t)
	ts.auditRecorder = audit_mocks.NewMockEventRecorder(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)

	deps := integration_service.ServiceDependencies{
		Repo:            ts.repo,
		CryptoService:   ts.crypto,
		ProviderFactory: ts.factory,
		AuditRecorder:   ts.auditRecorder,
		Dispatcher:      ts.dispatcher,
		Logger:          &log.NoOpLogger{},
	}
	ts.service = integration_service.NewService(deps)
	return ts
}

func contextWithActor() context.Context {
	actor := &auth.Actor{UserID: uuid.New()}
	return auth.NewContextWithActor(context.Background(), actor)
}

// --- Admin Service Tests ---

func TestService_Create(t *testing.T) {
	ctx := contextWithActor()
	params := integration.CreateIntegrationParams{
		Name:        "Stripe Prod",
		Provider:    core.ProviderStripe,
		Credentials: map[string]string{"key": "sk_123"},
	}

	t.Run("should create successfully", func(t *testing.T) {
		ts := setupService(t)

		// 1. Crypto
		ts.crypto.EXPECT().Encrypt(ctx, mock.Anything).Return([]byte("encrypted"), nil, nil).Once()

		// 2. Repo Create
		ts.repo.EXPECT().CreateIntegration(ctx, mock.MatchedBy(func(i *integration.Integration) bool {
			return i.Name == params.Name && i.Provider == core.ProviderStripe && string(i.CredentialsEncrypted) == "encrypted"
		})).Return(nil).Once()

		// 3. Audit
		ts.auditRecorder.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		res, err := ts.service.Create(ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, params.Name, res.Name)
	})

	t.Run("should fail without actor", func(t *testing.T) {
		ts := setupService(t)
		_, err := ts.service.Create(context.Background(), params)
		assert.ErrorIs(t, err, integration.ErrPermissionDenied)
	})
}

func TestService_TestConnection(t *testing.T) {
	ctx := contextWithActor()
	id := uuid.New()
	integ := &integration.Integration{
		ID:                   id,
		Provider:             core.ProviderStripe,
		CredentialsEncrypted: []byte("enc"),
	}
	creds := map[string]string{"key": "val"}
	credsBytes, _ := json.Marshal(creds)

	t.Run("should return success if adapter connects", func(t *testing.T) {
		ts := setupService(t)

		// 1. Get Integration
		ts.repo.EXPECT().GetIntegration(ctx, id).Return(integ, nil).Once()

		// 2. Decrypt
		ts.crypto.EXPECT().Decrypt(ctx, integ.CredentialsEncrypted, mock.Anything).Return(credsBytes, nil).Once()

		// 3. Build Adapter
		// Use our composed mock that satisfies both PaymentGateway AND ConnectionTester
		mockGateway := new(MockTestableGateway)
		mockGateway.On("TestConnection", ctx).Return(nil).Once()

		// Factory returns the mock as a PaymentGateway interface
		ts.factory.EXPECT().BuildPaymentGateway(core.ProviderStripe, creds).Return(mockGateway, nil).Once()

		// 4. Audit
		ts.auditRecorder.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		ok, msg, err := ts.service.TestConnection(ctx, id)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Success", msg)
	})

	t.Run("should return failure if adapter test fails", func(t *testing.T) {
		ts := setupService(t)

		// Use composed mock again
		mockGateway := new(MockTestableGateway)
		mockGateway.On("TestConnection", ctx).Return(errors.New("api unreachable")).Once()

		ts.repo.EXPECT().GetIntegration(ctx, id).Return(integ, nil).Once()
		ts.crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return(credsBytes, nil).Once()
		ts.factory.EXPECT().BuildPaymentGateway(core.ProviderStripe, creds).Return(mockGateway, nil).Once()

		ok, msg, err := ts.service.TestConnection(ctx, id)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, msg, "api unreachable")
	})
}

// --- Webhook Service Tests ---

func TestService_ProcessWebhook(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()
	provider := core.ProviderStripe

	integ := &integration.Integration{ID: uuid.New(), Provider: provider, CredentialsEncrypted: []byte("enc")}
	creds := map[string]string{"secret": "shh"}
	credsBytes, _ := json.Marshal(creds)

	// Mock HTTP Request
	req, _ := http.NewRequest("POST", "/webhook", bytes.NewBufferString(`{}`))

	t.Run("should process valid webhook", func(t *testing.T) {
		// 1. Get Active Integration
		ts.repo.EXPECT().GetActiveIntegrationByProvider(ctx, provider).Return(integ, nil).Once()

		// 2. Decrypt
		ts.crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return(credsBytes, nil).Once()

		// 3. Get Parser
		mockParser := int_mocks.NewMockWebhookParser(t)
		ts.factory.EXPECT().GetWebhookParser(integ, creds).Return(mockParser, nil).Once()

		// 4. Parse
		mockParser.EXPECT().Parse(req, integ).Return("charge.succeeded", "evt_123", []byte("{}"), nil).Once()

		// 5. Idempotency Check (Not found = good)
		ts.repo.EXPECT().GetWebhookByProviderID(ctx, provider, "evt_123").Return(nil, integration.ErrWebhookNotFound).Once()

		// 6. Create
		ts.repo.EXPECT().CreateWebhook(ctx, mock.MatchedBy(func(w *integration.Webhook) bool {
			return w.ProviderEventID == "evt_123" && w.Status == integration.WebhookPending
		})).Return(nil).Once()

		// 7. Dispatch Job
		ts.dispatcher.EXPECT().Dispatch(ctx, integration.JobIntegrationProcessWebhook, mock.Anything, mock.Anything).Return(nil, nil).Once()

		wh, err := ts.service.ProcessWebhook(ctx, provider, req)
		require.NoError(t, err)
		assert.NotNil(t, wh)
		assert.Equal(t, "evt_123", wh.ProviderEventID)
	})

	t.Run("should return duplicate error if already exists", func(t *testing.T) {
		// ... (Setup mocks same as above for 1-4) ...
		ts.repo.EXPECT().GetActiveIntegrationByProvider(ctx, provider).Return(integ, nil).Once()
		ts.crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return(credsBytes, nil).Once()
		mockParser := int_mocks.NewMockWebhookParser(t)
		ts.factory.EXPECT().GetWebhookParser(integ, creds).Return(mockParser, nil).Once()
		mockParser.EXPECT().Parse(req, integ).Return("charge.succeeded", "evt_duplicate", []byte("{}"), nil).Once()

		// 5. Idempotency Check (Found!)
		existing := &integration.Webhook{ID: uuid.New()}
		ts.repo.EXPECT().GetWebhookByProviderID(ctx, provider, "evt_duplicate").Return(existing, nil).Once()

		_, err := ts.service.ProcessWebhook(ctx, provider, req)
		assert.ErrorIs(t, err, integration.ErrDuplicateWebhook)
	})

	t.Run("should return error on signature failure", func(t *testing.T) {
		ts.repo.EXPECT().GetActiveIntegrationByProvider(ctx, provider).Return(integ, nil).Once()
		ts.crypto.EXPECT().Decrypt(ctx, mock.Anything, mock.Anything).Return(credsBytes, nil).Once()
		mockParser := int_mocks.NewMockWebhookParser(t)
		ts.factory.EXPECT().GetWebhookParser(integ, creds).Return(mockParser, nil).Once()

		// Parser fails
		mockParser.EXPECT().Parse(req, integ).Return("", "", nil, integration.ErrInvalidSignature).Once()

		_, err := ts.service.ProcessWebhook(ctx, provider, req)
		assert.ErrorIs(t, err, integration.ErrInvalidSignature)
	})
}
