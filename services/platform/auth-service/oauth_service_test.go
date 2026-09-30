// internal/services/auth/oauth_service_test.go
package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"
)

type oauthServiceTestSuite struct {
	repo          *auth_mocks.MockRepository
	audit         *mocks.MockEventRecorder
	mainService   *auth_mocks.MockService
	userService   *user_mocks.MockService
	provider      *auth_mocks.MockOAuthProvider
	providerFct   auth_service.OAuthProviderFactory
	cache         *cache_mocks.MockService
	randomService *auth_mocks.MockTokenGenerator
	service       auth.OAuthService
}

func setupOAuthService(t *testing.T) oauthServiceTestSuite {
	var ts oauthServiceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.audit = mocks.NewMockEventRecorder(t)
	ts.mainService = auth_mocks.NewMockService(t)
	ts.userService = user_mocks.NewMockService(t)
	ts.provider = auth_mocks.NewMockOAuthProvider(t)
	ts.cache = cache_mocks.NewMockService(t)
	ts.randomService = auth_mocks.NewMockTokenGenerator(t)

	ts.providerFct = func(provider string) (auth.OAuthProvider, error) {
		if provider == "google" {
			return ts.provider, nil
		}
		return nil, errors.New("unsupported provider")
	}

	ts.service = auth_service.NewOAuthService(
		ts.repo,
		ts.audit,
		ts.mainService,
		ts.userService,
		ts.providerFct,
		ts.cache,
		ts.randomService,
		&log.NoOpLogger{},
	)
	return ts
}

func TestOAuthService_Initiate(t *testing.T) {
	ts := setupOAuthService(t)
	ctx := context.Background()

	t.Run("should succeed for supported provider", func(t *testing.T) {
		state := "random-state-string"
		ts.randomService.EXPECT().Generate().Return(state, "hashed", nil).Once()
		ts.cache.EXPECT().Set(ctx, "oauth:state:"+state, []byte("1"), 10*time.Minute).Return(nil).Once()
		ts.provider.EXPECT().GetAuthCodeURL(state).Return("https://google.com/auth?state=" + state).Once()

		url, st, err := ts.service.Initiate(ctx, "google")
		require.NoError(t, err)
		assert.Equal(t, state, st)
		assert.Equal(t, "https://google.com/auth?state="+state, url)
	})

	t.Run("should fail for unsupported provider", func(t *testing.T) {
		_, _, err := ts.service.Initiate(ctx, "facebook")
		assert.ErrorIs(t, err, auth.ErrOAuthProviderNotSupported)
	})
}

func TestOAuthService_HandleCallback(t *testing.T) {
	setupOAuthService(t)
	ctx := context.Background()
	provider := "google"
	state := "valid-state"
	code := "auth-code"
	email := core.MustNewEmail(t, "new.user@google.com")
	params := auth.HandleCallbackParams{Provider: provider, State: state, Code: code, IPAddress: "127.0.0.1", UserAgent: "test"}

	userInfo := &auth.OAuthUserInfo{
		ProviderUserID:  "google-user-123",
		Email:           email,
		IsEmailVerified: true,
		Name:            "Test",
	}

	t.Run("should register a new user", func(t *testing.T) {
		ts := setupOAuthService(t) // fresh mocks for each sub-test

		// Common setup for successful state verification
		ts.cache.EXPECT().Get(ctx, "oauth:state:"+state).Return([]byte("1"), nil).Once()
		ts.cache.EXPECT().Delete(ctx, "oauth:state:"+state).Return(nil).Once()
		ts.provider.EXPECT().ExchangeCodeForUserInfo(ctx, code).Return(userInfo, nil).Once()

		// Mock the transaction
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			// Expect it to fail finding an existing auth method and account
			txRepo.EXPECT().GetAuthMethodByProvider(ctx, provider, userInfo.ProviderUserID).Return(nil, auth.ErrAuthMethodNotFound).Once()
			txRepo.EXPECT().GetAccountByEmail(ctx, email).Return(nil, auth.ErrAccountNotFound).Once()
			// Expect it to create a new account and auth method
			txRepo.EXPECT().CreateAccount(ctx, mock.AnythingOfType("*auth.Account")).Return(nil).Once()
			txRepo.EXPECT().CreateAuthMethod(ctx, mock.AnythingOfType("*auth.AuthMethod")).Return(nil).Once()
			// Expect user profile creation
			ts.userService.EXPECT().CreateUser(ctx, mock.AnythingOfType("user.CreateUserParams")).Return(&user.User{}, nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		// Expect audit record
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// Expect session creation to be called
		mockTokenPair := &auth.TokenPair{AccessToken: "access-token"}
		ts.mainService.EXPECT().CreateSessionForAccount(
			ctx,
			mock.AnythingOfType("*auth.Account"),
			params.IPAddress,
			params.UserAgent,
			mock.Anything, // This handles both nil and non-nil pointers gracefully.
		).Return(mockTokenPair, nil).Once()

		result, err := ts.service.HandleCallback(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result.TokenPair)
		assert.Equal(t, "access-token", result.TokenPair.AccessToken)
	})

	t.Run("should fail on state mismatch", func(t *testing.T) {
		ts := setupOAuthService(t) // fresh mocks
		ts.cache.EXPECT().Get(ctx, "oauth:state:"+state).Return(nil, errors.New("not found")).Once()

		_, err := ts.service.HandleCallback(ctx, params)
		assert.ErrorIs(t, err, auth.ErrOAuthStateMismatch)
	})

	// ADDED: Test for linking an existing account
	t.Run("should link to an existing user by email", func(t *testing.T) {
		ts := setupOAuthService(t)
		existingAccount := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: email, Status: auth.StatusActive}

		ts.cache.EXPECT().Get(ctx, "oauth:state:"+state).Return([]byte("1"), nil).Once()
		ts.cache.EXPECT().Delete(ctx, "oauth:state:"+state).Return(nil).Once()
		ts.provider.EXPECT().ExchangeCodeForUserInfo(ctx, code).Return(userInfo, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAuthMethodByProvider(ctx, provider, userInfo.ProviderUserID).Return(nil, auth.ErrAuthMethodNotFound).Once()
			// This time, it finds an account by email
			txRepo.EXPECT().GetAccountByEmail(ctx, email).Return(existingAccount, nil).Once()
			// It should then create just the new auth method
			txRepo.EXPECT().CreateAuthMethod(ctx, mock.MatchedBy(func(m *auth.AuthMethod) bool {
				return m.AccountID == existingAccount.ID && m.Credential == userInfo.ProviderUserID
			})).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		mockTokenPair := &auth.TokenPair{AccessToken: "access-token"}
		ts.mainService.EXPECT().CreateSessionForAccount(
			ctx,
			existingAccount,
			params.IPAddress,
			params.UserAgent,
			mock.Anything,
		).Return(mockTokenPair, nil).Once()

		result, err := ts.service.HandleCallback(ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, result.TokenPair)
	})
}
