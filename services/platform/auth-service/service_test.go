// internal/services/auth/service_test.go
package auth_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// serviceTestSuite holds all mocked dependencies for the main auth Service.
type serviceTestSuite struct {
	repo         *auth_mocks.MockRepository
	audit        *audit_mocks.MockEventRecorder
	passService  *auth_mocks.MockPasswordService
	tokenGen     *auth_mocks.MockTokenGenerator
	accessSigner *auth_mocks.MockTokenSigner
	mfaService   *auth_mocks.MockMFAService
	eventBus     *events_mocks.MockBus
	service      auth.Service
}

// setupService initializes a new test suite with fresh mocks.
func setupService(t *testing.T) serviceTestSuite {
	var ts serviceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.passService = auth_mocks.NewMockPasswordService(t)
	ts.tokenGen = auth_mocks.NewMockTokenGenerator(t)
	ts.accessSigner = auth_mocks.NewMockTokenSigner(t)
	ts.mfaService = auth_mocks.NewMockMFAService(t)
	ts.eventBus = events_mocks.NewMockBus(t)

	linkConfig := &config.LinkConfig{
		EmailVerificationURL: "http://test.com/verify?token={{.Token}}",
		PasswordResetURL:     "http://test.com/reset?token={{.Token}}",
	}

	deps := auth_service.ServiceDependencies{
		Repo:         ts.repo,
		Audit:        ts.audit,
		PassService:  ts.passService,
		TokenGen:     ts.tokenGen,
		AccessSigner: ts.accessSigner,
		MfaService:   ts.mfaService,
		Config:       config.AuthConfig{MaxFailedLogins: 5, LockoutDuration: 15 * time.Minute},
		Logger:       &log.NoOpLogger{},
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   7 * 24 * time.Hour,
		EventBus:     ts.eventBus,
		LinkConfig:   linkConfig,
	}
	ts.service = auth_service.NewService(deps)
	return ts
}

// Helper for tests that need an http.Request in the context
func contextWithRequest() context.Context {
	req := httptest.NewRequest("POST", "/", nil)
	return context.WithValue(context.Background(), "http.Request", req)
}
func TestService_Register(t *testing.T) {
	ctx := context.Background()
	email := core.MustNewEmail(t, "newuser@example.com")
	params := auth.RegisterParams{Email: email, Password: "strongPassword123"}

	t.Run("should succeed with valid parameters", func(t *testing.T) {
		ts := setupService(t)

		ts.passService.EXPECT().ValidateStrength(params.Password).Return(nil).Once()
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(nil, auth.ErrAccountNotFound).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().CreateAccount(ctx, mock.Anything).Return(nil).Once()
			ts.passService.EXPECT().Hash(params.Password).Return("hashedPassword", nil).Once()
			txRepo.EXPECT().CreateAuthMethod(ctx, mock.Anything).Return(nil).Once()
			ts.tokenGen.EXPECT().Generate().Return("raw", "hashed", nil).Once()
			txRepo.EXPECT().CreateToken(ctx, mock.Anything).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		// Expect synchronous publish
		ts.eventBus.EXPECT().Publish(ctx, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventAccountCreated
		})).Return(nil).Once()

		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.Register(ctx, params)
		require.NoError(t, err)
	})

	t.Run("should fail if email already exists", func(t *testing.T) {
		ts := setupService(t)
		existingAccount := &auth.Account{}

		ts.passService.EXPECT().ValidateStrength(params.Password).Return(nil).Once()
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(existingAccount, nil).Once()

		_, err := ts.service.Register(ctx, params)
		assert.ErrorIs(t, err, auth.ErrAccountExists)
		ts.eventBus.AssertNotCalled(t, "Publish")
	})
}

func TestService_Login(t *testing.T) {
	ts := setupService(t)
	ctx := contextWithRequest() // Use helper for request context
	email := core.MustNewEmail(t, "user@example.com")
	params := auth.LoginParams{Email: email, Password: "password"}
	account := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: email, Status: auth.StatusActive}
	passMethod := &auth.AuthMethod{Credential: "hashedPassword"}

	t.Run("should succeed for active user without MFA", func(t *testing.T) {
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(account, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, account.ID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(nil).Once()
		ts.repo.EXPECT().ResetFailedLogin(ctx, account.ID).Return(nil).Once()

		// Mocks for createSessionAndTokenPair
		ts.tokenGen.EXPECT().Generate().Return("rawRefresh", "hashedRefresh", nil).Once()
		ts.tokenGen.EXPECT().Hash("rawRefresh").Return("hashedRefresh").Once()
		ts.repo.EXPECT().CreateSession(ctx, mock.Anything).Return(nil).Once()
		ts.accessSigner.EXPECT().Sign(mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasImp := claims["imp"]
			return !hasImp
		}), mock.Anything).Return("accessToken", nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		result, err := ts.service.Login(ctx, params)
		require.NoError(t, err)
		assert.False(t, result.MFAIsRequired)
		assert.NotNil(t, result.TokenPair)
		assert.Equal(t, "accessToken", result.TokenPair.AccessToken)
	})

	t.Run("should return MFA challenge for user with MFA enabled", func(t *testing.T) {
		mfaAccount := &auth.Account{ID: uuid.New(), Email: email, Status: auth.StatusActive, MfaEnabled: true}
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(mfaAccount, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, mfaAccount.ID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(nil).Once()
		ts.repo.EXPECT().ResetFailedLogin(ctx, mfaAccount.ID).Return(nil).Once()
		ts.accessSigner.EXPECT().Sign(mock.Anything, mock.Anything).Return("mfaChallengeToken", nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		result, err := ts.service.Login(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.MFAIsRequired)
		assert.Nil(t, result.TokenPair)
		assert.Equal(t, "mfaChallengeToken", result.MfaChallengeToken)
	})

	t.Run("should fail for incorrect password and increment fail count", func(t *testing.T) {
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(account, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, account.ID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(errors.New("mismatch")).Once()
		// FIXED: The mock now returns two values: the new count and an error.
		ts.repo.EXPECT().IncrementFailedLogin(ctx, account.ID).Return(1, nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.Login(ctx, params)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})

	t.Run("should lock account after too many failed attempts", func(t *testing.T) {
		lockedAccount := &auth.Account{ID: uuid.New(), Email: email, Status: auth.StatusActive, FailedLoginCount: 4}
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(lockedAccount, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, lockedAccount.ID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(errors.New("mismatch")).Once()
		// FIXED: The mock now returns the new count which triggers the lock.
		ts.repo.EXPECT().IncrementFailedLogin(ctx, lockedAccount.ID).Return(5, nil).Once()
		ts.repo.EXPECT().LockAccount(ctx, lockedAccount.ID, 15*time.Minute).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.Login(ctx, params)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	})
}

func TestService_LoginMFA(t *testing.T) {
	ts := setupService(t)
	ctx := contextWithRequest()
	accountID := uuid.New()
	account := &auth.Account{ID: accountID, UserID: uuid.New(), Email: core.MustNewEmail(t, "mfa@user.com")}
	params := auth.LoginMFAParams{MfaChallengeToken: "mfaChallengeToken", MfaCode: "123456"}
	claims := map[string]interface{}{"sub": accountID.String(), "purpose": "mfa_challenge"}

	t.Run("should succeed with a valid MFA code", func(t *testing.T) {
		ts.accessSigner.EXPECT().Verify(params.MfaChallengeToken).Return(claims, nil).Once()
		ts.mfaService.EXPECT().VerifyLoginCode(ctx, accountID, params.MfaCode).Return(nil).Once()
		ts.repo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()

		// Mocks for createSessionAndTokenPair
		ts.tokenGen.EXPECT().Generate().Return("rawRefresh", "hashedRefresh", nil).Once()
		ts.tokenGen.EXPECT().Hash("rawRefresh").Return("hashedRefresh").Once()
		ts.repo.EXPECT().CreateSession(ctx, mock.Anything).Return(nil).Once()
		ts.accessSigner.EXPECT().Sign(mock.Anything, mock.Anything).Return("accessToken", nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		pair, err := ts.service.LoginMFA(ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, pair)
		assert.Equal(t, "accessToken", pair.AccessToken)
	})

	t.Run("should fail if mfa code is invalid", func(t *testing.T) {
		ts.accessSigner.EXPECT().Verify(params.MfaChallengeToken).Return(claims, nil).Once()
		ts.mfaService.EXPECT().VerifyLoginCode(ctx, accountID, params.MfaCode).Return(auth.ErrInvalidMFACode).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		_, err := ts.service.LoginMFA(ctx, params)
		assert.ErrorIs(t, err, auth.ErrInvalidMFACode)
	})
}

func TestService_RefreshToken(t *testing.T) {
	ts := setupService(t)
	ctx := contextWithRequest()
	params := auth.RefreshTokenParams{RefreshToken: "rawRefreshToken"}
	account := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: core.MustNewEmail(t, "user@example.com")}
	session := &auth.Session{ID: uuid.New(), AccountID: account.ID, ExpiresAt: time.Now().Add(1 * time.Hour)}

	t.Run("should succeed with a valid refresh token", func(t *testing.T) {
		ts.tokenGen.EXPECT().Hash(params.RefreshToken).Return("hashedRefreshToken").Once()
		// FIXED: Mock the new FindSessionByRefreshTokenHash method.
		ts.repo.EXPECT().FindSessionByRefreshTokenHash(ctx, "hashedRefreshToken").Return(session, nil).Once()
		ts.repo.EXPECT().GetAccountByID(ctx, account.ID).Return(account, nil).Once()

		// Transaction for rotation
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			// FIXED: Mock the new RevokeSession method instead of DeleteSession.
			txRepo.EXPECT().RevokeSession(ctx, session.ID).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		// Mocks for creating the NEW session and pair
		ts.tokenGen.EXPECT().Generate().Return("newRawRefresh", "", nil).Once()
		ts.tokenGen.EXPECT().Hash("newRawRefresh").Return("newHashedRefresh").Once()
		ts.repo.EXPECT().CreateSession(ctx, mock.Anything).Return(nil).Once()
		ts.accessSigner.EXPECT().Sign(mock.Anything, mock.Anything).Return("newAccessToken", nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		pair, err := ts.service.RefreshToken(ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, pair)
		assert.Equal(t, "newAccessToken", pair.AccessToken)
	})

	// ADDED: New test case for token reuse detection.
	t.Run("should fail and revoke all sessions on token reuse", func(t *testing.T) {
		revokedTime := time.Now().Add(-5 * time.Minute)
		revokedSession := &auth.Session{
			ID:        uuid.New(),
			AccountID: account.ID,
			ExpiresAt: time.Now().Add(1 * time.Hour),
			RevokedAt: &revokedTime,
		}

		ts.tokenGen.EXPECT().Hash(params.RefreshToken).Return("hashedRefreshToken").Once()
		// The service finds a session, but it's already been revoked.
		ts.repo.EXPECT().FindSessionByRefreshTokenHash(ctx, "hashedRefreshToken").Return(revokedSession, nil).Once()
		// CRITICAL: Expect the service to revoke all other sessions for this user as a security response.
		ts.repo.EXPECT().RevokeAllForAccount(ctx, revokedSession.AccountID).Return(nil).Once()

		_, err := ts.service.RefreshToken(ctx, params)
		assert.ErrorIs(t, err, auth.ErrInvalidRefreshToken)
	})
}

func TestService_VerifyAccessToken(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()
	accountID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	claims := map[string]interface{}{
		"sub": accountID.String(),
		"uid": userID.String(),
		"sid": sessionID.String(),
		"eml": "user@example.com",
	}

	t.Run("should succeed with valid token and claims", func(t *testing.T) {
		ts.accessSigner.EXPECT().Verify("validToken").Return(claims, nil).Once()
		actor, err := ts.service.VerifyAccessToken(ctx, "validToken")
		require.NoError(t, err)
		assert.Equal(t, accountID, actor.AccountID)
		assert.Equal(t, userID, actor.UserID)
		assert.Equal(t, sessionID, actor.SessionID)
	})

	t.Run("should fail if token verification fails", func(t *testing.T) {
		ts.accessSigner.EXPECT().Verify("invalidToken").Return(nil, auth.ErrTokenSignature).Once()
		_, err := ts.service.VerifyAccessToken(ctx, "invalidToken")
		assert.ErrorIs(t, err, auth.ErrTokenSignature)
	})
}

func TestService_MagicLink(t *testing.T) {
	ctx := contextWithRequest()
	email := core.MustNewEmail(t, "user@example.com")
	ip, ua := "127.0.0.1", "test-agent"
	account := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: email}

	t.Run("RequestMagicLink should succeed for existing user and publish event", func(t *testing.T) {
		ts := setupService(t) // Fresh mocks
		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(account, nil).Once()
		ts.tokenGen.EXPECT().Generate().Return("raw", "hashed", nil).Once()
		ts.repo.EXPECT().CreateToken(ctx, mock.AnythingOfType("*auth.Token")).Return(nil).Once()

		ts.eventBus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventMagicLinkRequested
		})).
			Run(func(ctx context.Context, event events.DomainEvent) {
				wg.Done()
			}).
			Return(nil).
			Once()

		err := ts.service.RequestMagicLink(ctx, email, ip, ua)
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("RequestMagicLink should silently succeed for non-existent user", func(t *testing.T) {
		ts := setupService(t)
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(nil, auth.ErrAccountNotFound).Once()

		err := ts.service.RequestMagicLink(ctx, email, ip, ua)
		require.NoError(t, err)
		// Ensure no other calls were made.
		ts.tokenGen.AssertNotCalled(t, "Generate")
	})

	t.Run("LoginWithMagicLink should succeed with valid token", func(t *testing.T) {
		ts := setupService(t)
		rawToken, hashedToken := "raw-token", "hashed-token"
		mlToken := &auth.Token{ID: uuid.New(), AccountID: account.ID}

		ts.tokenGen.EXPECT().Hash(rawToken).Return(hashedToken).Once()
		ts.repo.EXPECT().GetTokenByHash(ctx, hashedToken, auth.TokenMagicLinkLogin).Return(mlToken, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().UseToken(ctx, mlToken.ID).Return(nil).Once()
			txRepo.EXPECT().GetAccountByID(ctx, mlToken.AccountID).Return(account, nil).Once()

			// Mock calls from CreateSessionForAccount
			ts.tokenGen.EXPECT().Generate().Return("rawRefresh", "", nil).Once()
			ts.tokenGen.EXPECT().Hash("rawRefresh").Return("hashedRefresh").Once()
			ts.repo.EXPECT().CreateSession(ctx, mock.Anything).Return(nil).Once()
			ts.accessSigner.EXPECT().Sign(mock.Anything, mock.Anything).Return("access-token", nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		pair, err := ts.service.LoginWithMagicLink(ctx, rawToken, ip, ua)
		require.NoError(t, err)
		require.NotNil(t, pair)
		assert.Equal(t, "access-token", pair.AccessToken)
	})

	t.Run("LoginWithMagicLink should fail with invalid token", func(t *testing.T) {
		ts := setupService(t)
		rawToken, hashedToken := "raw-token", "hashed-token"
		ts.tokenGen.EXPECT().Hash(rawToken).Return(hashedToken).Once()
		ts.repo.EXPECT().GetTokenByHash(ctx, hashedToken, auth.TokenMagicLinkLogin).Return(nil, auth.ErrTokenNotFound).Once()

		_, err := ts.service.LoginWithMagicLink(ctx, rawToken, ip, ua)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)
	})
}
func TestService_RequestMagicLink(t *testing.T) {
	ctx := contextWithRequest()
	email := core.MustNewEmail(t, "user@example.com")
	ip, ua := "127.0.0.1", "test-agent"
	account := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: email}

	t.Run("RequestMagicLink should succeed for existing user and publish event", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(account, nil).Once()
		ts.tokenGen.EXPECT().Generate().Return("raw", "hashed", nil).Once()
		ts.repo.EXPECT().CreateToken(ctx, mock.AnythingOfType("*auth.Token")).Return(nil).Once()

		// Expect synchronous publish
		ts.eventBus.EXPECT().Publish(ctx, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventMagicLinkRequested
		})).Return(nil).Once()

		err := ts.service.RequestMagicLink(ctx, email, ip, ua)
		require.NoError(t, err)
	})

	// Renamed test case to match logic: even if bus fails, we log error but don't fail user flow
	t.Run("should succeed but log error if event bus publish fails", func(t *testing.T) {
		ts := setupService(t)

		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(account, nil).Once()
		ts.tokenGen.EXPECT().Generate().Return("raw", "hashed", nil).Once()
		ts.repo.EXPECT().CreateToken(ctx, mock.AnythingOfType("*auth.Token")).Return(nil).Once()

		// Simulate bus failure
		ts.eventBus.EXPECT().Publish(ctx, mock.Anything).Return(errors.New("event bus is down")).Once()

		err := ts.service.RequestMagicLink(ctx, email, ip, ua)
		// We expect NO error because the service swallows bus errors to avoid blocking
		assert.NoError(t, err)
	})
}
