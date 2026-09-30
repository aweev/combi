package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"
)

// mfaServiceTestSuite holds all mocked dependencies for the MFAService.
type mfaServiceTestSuite struct {
	repo        *auth_mocks.MockRepository
	audit       *audit_mocks.MockEventRecorder
	totpManager *auth_mocks.MockTOTPManager
	passService *auth_mocks.MockPasswordService
	tokenGen    *auth_mocks.MockTokenGenerator
	service     auth.MFAService
}

// setupMFAService initializes a new test suite with fresh mocks.
func setupMFAService(t *testing.T) mfaServiceTestSuite {
	var ts mfaServiceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.totpManager = auth_mocks.NewMockTOTPManager(t)
	ts.passService = auth_mocks.NewMockPasswordService(t)
	ts.tokenGen = auth_mocks.NewMockTokenGenerator(t)

	ts.service = auth_service.NewMFAService(
		ts.repo,
		ts.audit,
		ts.totpManager,
		ts.passService,
		ts.tokenGen,
		&log.NoOpLogger{},
	)
	return ts
}

func TestMFAService_SetupTOTP(t *testing.T) {
	ts := setupMFAService(t)
	ctx := context.Background()
	accountID := uuid.New()
	email := core.MustNewEmail(t, "user@example.com")
	account := &auth.Account{ID: accountID, Email: email, MfaEnabled: false}

	t.Run("should succeed for an account with MFA disabled", func(t *testing.T) {
		ts.repo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()
		ts.totpManager.EXPECT().GenerateSecretAndQRCode("NGO Platform", email.String()).
			Return("rawSecret", "encryptedSecret", "qrCodeURL", nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodTOTP, false).Return(nil).Once()
			txRepo.EXPECT().CreateAuthMethod(ctx, mock.MatchedBy(func(m *auth.AuthMethod) bool {
				return !m.IsVerified && m.Credential == "encryptedSecret"
			})).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		result, err := ts.service.SetupTOTP(ctx, accountID)
		require.NoError(t, err)
		assert.Equal(t, "rawSecret", result.Secret)
		assert.Equal(t, "qrCodeURL", result.QRCodeImageURL)
	})

	t.Run("should fail if MFA is already enabled", func(t *testing.T) {
		mfaEnabledAccount := &auth.Account{ID: accountID, MfaEnabled: true}
		ts.repo.EXPECT().GetAccountByID(ctx, accountID).Return(mfaEnabledAccount, nil).Once()

		_, err := ts.service.SetupTOTP(ctx, accountID)
		assert.ErrorIs(t, err, auth.ErrMfaAlreadyEnabled)
	})
}
func TestMFAService_EnableTOTP(t *testing.T) {
	ts := setupMFAService(t)
	ctx := context.Background()
	accountID := uuid.New()
	validCode := "123456"

	t.Run("should succeed with a valid code", func(t *testing.T) {
		// ADD state variables inside the sub-test
		account := &auth.Account{ID: accountID, MfaEnabled: false}
		unverifiedMethod := &auth.AuthMethod{Credential: "encryptedSecret", IsVerified: false}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			// FIX: Use GetAccountByIDForUpdate
			txRepo.EXPECT().GetAccountByIDForUpdate(ctx, accountID).Return(account, nil).Once()
			txRepo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(unverifiedMethod, nil).Once()
			ts.totpManager.EXPECT().VerifyCode("encryptedSecret", validCode).Return(true, nil).Once()
			txRepo.EXPECT().UpdateAuthMethod(ctx, mock.MatchedBy(func(m *auth.AuthMethod) bool {
				return m.IsVerified
			})).Return(nil).Once()
			txRepo.EXPECT().DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodBackupCode, true).Return(nil).Once()
			ts.tokenGen.EXPECT().Generate().Return("rawBackup", "hashedBackup", nil).Times(10)
			txRepo.EXPECT().CreateAuthMethod(ctx, mock.MatchedBy(func(m *auth.AuthMethod) bool {
				return m.Type == auth.AuthMethodBackupCode && m.Credential == "hashedBackup"
			})).Return(nil).Times(10)
			txRepo.EXPECT().UpdateAccount(ctx, mock.MatchedBy(func(acc *auth.Account) bool {
				return acc.MfaEnabled
			})).Return(nil).Once()

			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		backupCodes, err := ts.service.EnableTOTP(ctx, accountID, validCode)
		require.NoError(t, err)
		assert.Len(t, backupCodes, 10)
	})

	t.Run("should fail with an invalid code", func(t *testing.T) {
		// ADD state variables inside the sub-test
		account := &auth.Account{ID: accountID, MfaEnabled: false}
		unverifiedMethod := &auth.AuthMethod{Credential: "encryptedSecret", IsVerified: false}
		invalidCode := "654321"

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			// FIX: Use GetAccountByIDForUpdate
			txRepo.EXPECT().GetAccountByIDForUpdate(ctx, accountID).Return(account, nil).Once()
			txRepo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(unverifiedMethod, nil).Once()
			ts.totpManager.EXPECT().VerifyCode("encryptedSecret", invalidCode).Return(false, nil).Once()

			err := fn(txRepo)
			assert.ErrorIs(t, err, auth.ErrInvalidMFACode)
		}).Return(auth.ErrInvalidMFACode).Once()

		_, err := ts.service.EnableTOTP(ctx, accountID, invalidCode)
		assert.ErrorIs(t, err, auth.ErrInvalidMFACode)
	})
}

func TestMFAService_DisableTOTP(t *testing.T) {
	ts := setupMFAService(t)
	ctx := context.Background()
	accountID := uuid.New()
	params := auth.DisableTOTPParams{Password: "password", Code: "123456"}

	t.Run("should succeed with valid password and code", func(t *testing.T) {
		// ADD state variables inside the sub-test
		account := &auth.Account{ID: accountID, MfaEnabled: true}
		passMethod := &auth.AuthMethod{Credential: "hashedPassword"}
		totpMethod := &auth.AuthMethod{Credential: "encryptedSecret", IsVerified: true}

		ts.repo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(totpMethod, nil).Once()
		ts.totpManager.EXPECT().VerifyCode("encryptedSecret", params.Code).Return(true, nil).Once()

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodTOTP, true).Return(nil).Once()
			txRepo.EXPECT().DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodBackupCode, true).Return(nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.MatchedBy(func(acc *auth.Account) bool {
				return !acc.MfaEnabled
			})).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.DisableTOTP(ctx, accountID, params)
		require.NoError(t, err)
	})

	t.Run("should fail if password is incorrect", func(t *testing.T) {
		// ADD state variables inside the sub-test
		account := &auth.Account{ID: accountID, MfaEnabled: true}
		passMethod := &auth.AuthMethod{Credential: "hashedPassword"}

		ts.repo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodPassword).Return(passMethod, nil).Once()
		ts.passService.EXPECT().Compare("hashedPassword", params.Password).Return(errors.New("mismatch")).Once()

		err := ts.service.DisableTOTP(ctx, accountID, params)
		assert.ErrorIs(t, err, auth.ErrIncorrectPassword)
	})
}

func TestMFAService_VerifyLoginCode(t *testing.T) {
	ts := setupMFAService(t)
	ctx := context.Background()
	accountID := uuid.New()
	totpMethod := &auth.AuthMethod{Type: auth.AuthMethodTOTP, Credential: "encryptedSecret", IsVerified: true}
	backupMethods := []*auth.AuthMethod{
		{ID: uuid.New(), Type: auth.AuthMethodBackupCode, Credential: "hashedBackup1", IsVerified: true},
		{ID: uuid.New(), Type: auth.AuthMethodBackupCode, Credential: "hashedBackup2", IsVerified: true},
	}

	t.Run("should succeed with a valid TOTP code", func(t *testing.T) {
		validCode := "123456"
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(totpMethod, nil).Once()
		ts.totpManager.EXPECT().VerifyCode("encryptedSecret", validCode).Return(true, nil).Once()
		// Note: ListAuthMethodsForAccount should not be called

		err := ts.service.VerifyLoginCode(ctx, accountID, validCode)
		require.NoError(t, err)
	})

	t.Run("should succeed with a valid backup code and consume it", func(t *testing.T) {
		backupCode := "rawBackup2"
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(nil, auth.ErrAuthMethodNotFound).Once()
		ts.repo.EXPECT().ListAuthMethodsForAccount(ctx, accountID).Return(backupMethods, nil).Once()
		// First backup code fails
		ts.tokenGen.EXPECT().Compare("hashedBackup1", backupCode).Return(false).Once()
		// Second backup code succeeds
		ts.tokenGen.EXPECT().Compare("hashedBackup2", backupCode).Return(true).Once()
		// Expect the successful backup method to be deleted
		ts.repo.EXPECT().DeleteAuthMethod(ctx, backupMethods[1].ID).Return(nil).Once()

		err := ts.service.VerifyLoginCode(ctx, accountID, backupCode)
		require.NoError(t, err)
	})

	t.Run("should fail if TOTP fails and no backup code matches", func(t *testing.T) {
		invalidCode := "999999"
		ts.repo.EXPECT().GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP).Return(totpMethod, nil).Once()
		ts.totpManager.EXPECT().VerifyCode("encryptedSecret", invalidCode).Return(false, nil).Once()
		ts.repo.EXPECT().ListAuthMethodsForAccount(ctx, accountID).Return(backupMethods, nil).Once()
		ts.tokenGen.EXPECT().Compare("hashedBackup1", invalidCode).Return(false).Once()
		ts.tokenGen.EXPECT().Compare("hashedBackup2", invalidCode).Return(false).Once()

		err := ts.service.VerifyLoginCode(ctx, accountID, invalidCode)
		assert.ErrorIs(t, err, auth.ErrInvalidMFACode)
	})
}
