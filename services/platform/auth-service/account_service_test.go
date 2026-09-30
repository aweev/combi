// internal/services/auth/account_service_test.go
package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	notif_mocks "github.com/combi/pkg/domain/notification/mocks"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type accountServiceTestSuite struct {
	repo         *auth_mocks.MockRepository
	audit        *audit_mocks.MockEventRecorder
	passService  *auth_mocks.MockPasswordService
	tokenGen     *auth_mocks.MockTokenGenerator
	notifService *notif_mocks.MockService
	eventBus     *events_mocks.MockBus
	service      auth.AccountService
}

func setupAccountService(t *testing.T) accountServiceTestSuite {
	t.Helper()
	var ts accountServiceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.passService = auth_mocks.NewMockPasswordService(t)
	ts.tokenGen = auth_mocks.NewMockTokenGenerator(t)
	ts.notifService = notif_mocks.NewMockService(t)
	ts.eventBus = events_mocks.NewMockBus(t)

	linkConfig := &config.LinkConfig{
		EmailVerificationURL: "http://test.com/verify?token={{.Token}}",
		PasswordResetURL:     "http://test.com/reset?token={{.Token}}",
	}

	ts.service = auth_service.NewAccountService(
		ts.repo,
		ts.audit,
		ts.passService,
		ts.tokenGen,
		ts.notifService,
		ts.eventBus,
		linkConfig,
		&log.NoOpLogger{},
	)
	return ts
}

func TestAccountService_RequestEmailVerification(t *testing.T) {
	ctx := context.Background()
	email := core.MustNewEmail(t, "test@example.com")
	acc := &auth.Account{ID: uuid.New(), UserID: uuid.New(), Email: email, Status: auth.StatusUnverified}

	t.Run("should succeed for unverified account and publish event", func(t *testing.T) {
		ts := setupAccountService(t)

		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(acc, nil).Once()
		ts.tokenGen.EXPECT().Generate().Return("raw", "hashed", nil).Once()
		ts.repo.EXPECT().CreateToken(ctx, mock.AnythingOfType("*auth.Token")).Return(nil).Once()

		// Expect synchronous publish
		ts.eventBus.EXPECT().Publish(ctx, mock.MatchedBy(func(e events.DomainEvent) bool {
			if e.Action != core.EventAccountCreated || e.ActorID != acc.ID {
				return false
			}
			// Verify payload contains verification link
			var p auth.UserCreatedPayload
			_ = json.Unmarshal(e.Payload, &p)
			return p.Email == email
		})).Return(nil).Once()

		err := ts.service.RequestEmailVerification(ctx, email)
		require.NoError(t, err)
	})

	t.Run("should do nothing if account is not found", func(t *testing.T) {
		ts := setupAccountService(t)
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(nil, auth.ErrAccountNotFound).Once()

		err := ts.service.RequestEmailVerification(ctx, email)
		assert.NoError(t, err)
		ts.eventBus.AssertNotCalled(t, "Publish")
	})

	t.Run("should do nothing if account is already verified", func(t *testing.T) {
		ts := setupAccountService(t)
		verifiedAcc := &auth.Account{ID: uuid.New(), Email: email, Status: auth.StatusActive}
		ts.repo.EXPECT().GetAccountByEmail(ctx, email).Return(verifiedAcc, nil).Once()

		err := ts.service.RequestEmailVerification(ctx, email)
		assert.NoError(t, err)
		ts.eventBus.AssertNotCalled(t, "Publish")
	})
}

func TestAccountService_VerifyEmail(t *testing.T) {
	ts := setupAccountService(t)
	ctx := context.Background()
	rawToken := "raw_token"
	hashedToken := "hashed_token"
	accID := uuid.New()
	token := &auth.Token{ID: uuid.New(), AccountID: accID, ExpiresAt: time.Now().Add(1 * time.Hour)}
	acc := &auth.Account{ID: accID, Status: auth.StatusUnverified}

	t.Run("should succeed with a valid token", func(t *testing.T) {
		ts.tokenGen.EXPECT().Hash(rawToken).Return(hashedToken).Once()
		ts.repo.EXPECT().GetTokenByHash(ctx, hashedToken, auth.TokenEmailVerification).Return(token, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByID(ctx, accID).Return(acc, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.Anything).Return(nil).Once()
			txRepo.EXPECT().UseToken(ctx, token.ID).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.audit.EXPECT().Record(mock.Anything, mock.AnythingOfType("audit.Event")).Return(nil).Once()

		err := ts.service.VerifyEmail(ctx, rawToken)
		require.NoError(t, err)
	})
}

func TestAccountService_CompletePasswordReset(t *testing.T) {
	ts := setupAccountService(t)
	ctx := context.Background()
	params := auth.CompletePasswordResetParams{ResetToken: "raw_token", NewPassword: "newStrongPassword123!"}
	hashedToken := "hashed_token"
	accID := uuid.New()
	token := &auth.Token{ID: uuid.New(), AccountID: accID, ExpiresAt: time.Now().Add(1 * time.Hour)}
	method := &auth.AuthMethod{AccountID: accID, Type: auth.AuthMethodPassword, Credential: "old_hash"}

	t.Run("should succeed with a valid token and strong password", func(t *testing.T) {
		ts.passService.EXPECT().ValidateStrength(params.NewPassword).Return(nil).Once()
		ts.tokenGen.EXPECT().Hash(params.ResetToken).Return(hashedToken).Once()
		ts.repo.EXPECT().GetTokenByHash(ctx, hashedToken, auth.TokenPasswordReset).Return(token, nil).Once()
		ts.passService.EXPECT().Hash(params.NewPassword).Return("new_hash", nil).Once()
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAuthMethod(ctx, accID, auth.AuthMethodPassword).Return(method, nil).Once()
			txRepo.EXPECT().UpdateAuthMethod(ctx, mock.Anything).Return(nil).Once()
			txRepo.EXPECT().UseToken(ctx, token.ID).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.CompletePasswordReset(ctx, params)
		require.NoError(t, err)
	})

	t.Run("should fail if password is too weak", func(t *testing.T) {
		ts.passService.EXPECT().ValidateStrength(params.NewPassword).Return(auth.ErrPasswordTooWeak).Once()

		err := ts.service.CompletePasswordReset(ctx, params)
		assert.ErrorIs(t, err, auth.ErrPasswordTooWeak)
	})
}

func TestAccountService_ChangePassword(t *testing.T) {
	ts := setupAccountService(t)
	ctx := context.Background()
	actorID := uuid.New()
	params := auth.ChangePasswordParams{OldPassword: "oldPassword", NewPassword: "newStrongPassword123!"}
	method := &auth.AuthMethod{AccountID: actorID, Type: auth.AuthMethodPassword, Credential: "hashed_old_password"}

	t.Run("should succeed with correct old password and strong new password", func(t *testing.T) {
		ts.passService.EXPECT().ValidateStrength(params.NewPassword).Return(nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, actorID, auth.AuthMethodPassword).Return(method, nil).Once()
		ts.passService.EXPECT().Compare(method.Credential, params.OldPassword).Return(nil).Once()
		ts.passService.EXPECT().Hash(params.NewPassword).Return("new_hash", nil).Once()
		ts.repo.EXPECT().UpdateAuthMethod(ctx, mock.Anything).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.ChangePassword(ctx, actorID, params)
		require.NoError(t, err)
	})

	t.Run("should fail if new password is same as old", func(t *testing.T) {
		samePassParams := auth.ChangePasswordParams{OldPassword: "same", NewPassword: "same"}
		err := ts.service.ChangePassword(ctx, actorID, samePassParams)
		assert.ErrorIs(t, err, auth.ErrSamePassword)
	})

	t.Run("should fail if old password is incorrect", func(t *testing.T) {
		ts.passService.EXPECT().ValidateStrength(params.NewPassword).Return(nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, actorID, auth.AuthMethodPassword).Return(method, nil).Once()
		ts.passService.EXPECT().Compare(method.Credential, params.OldPassword).Return(errors.New("mismatch")).Once()

		err := ts.service.ChangePassword(ctx, actorID, params)
		assert.ErrorIs(t, err, auth.ErrIncorrectPassword)
	})
}
