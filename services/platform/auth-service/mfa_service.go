package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Statically verify that it implements the domain interface.
var _ auth.MFAService = (*MFAService)(nil)

// MFAService manages Multi-Factor Authentication for an account.
type MFAService struct {
	repo        auth.Repository
	audit       audit.EventRecorder
	totpManager security.TOTPManager
	passService security.PasswordService
	tokenGen    security.TokenGenerator
	logger      log.Logger
}

// NewMFAService creates a new MFA management service.
func NewMFAService(
	repo auth.Repository,
	auditRecorder audit.EventRecorder,
	totpManager security.TOTPManager,
	passService security.PasswordService,
	tokenGen security.TokenGenerator,
	logger log.Logger,
) *MFAService {
	return &MFAService{
		repo:        repo,
		audit:       auditRecorder,
		totpManager: totpManager,
		passService: passService,
		tokenGen:    tokenGen,
		logger:      logger.With(log.String("service", "auth_mfa")),
	}
}

// SetupTOTP begins the process of enabling TOTP.
func (s *MFAService) SetupTOTP(ctx context.Context, accountID uuid.UUID) (*auth.SetupTOTPResult, error) {
	acc, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc.MfaEnabled {
		return nil, auth.ErrMfaAlreadyEnabled
	}

	rawSecret, encryptedSecret, qrCodeURL, err := s.totpManager.GenerateSecretAndQRCode("NGO Platform", acc.Email.String())
	if err != nil {
		return nil, fmt.Errorf("could not generate TOTP data: %w", err)
	}

	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		// Clean up any previous, unverified setup attempt.
		_ = txRepo.DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodTOTP, false) // `false` for not verified

		tempMethod := &auth.AuthMethod{
			ID:         uuid.New(),
			AccountID:  accountID,
			Type:       auth.AuthMethodTOTP,
			Credential: encryptedSecret,
			IsVerified: false,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		return txRepo.CreateAuthMethod(ctx, tempMethod)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store temporary TOTP method: %w", err)
	}

	return &auth.SetupTOTPResult{
		Secret:         rawSecret,
		QRCodeImageURL: qrCodeURL,
	}, nil
}

// EnableTOTP verifies the code and fully enables MFA for the account.
func (s *MFAService) EnableTOTP(ctx context.Context, accountID uuid.UUID, code string) ([]string, error) {
	var backupCodes []string

	err := s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		acc, err := txRepo.GetAccountByIDForUpdate(ctx, accountID)
		if err != nil {
			return err
		}
		if acc.MfaEnabled {
			return auth.ErrMfaAlreadyEnabled
		}

		method, err := txRepo.GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP)
		if err != nil {
			return fmt.Errorf("no TOTP setup process found, please start setup first: %w", err)
		}
		if method.IsVerified {
			return auth.ErrMfaAlreadyEnabled
		}

		valid, err := s.totpManager.VerifyCode(method.Credential, code)
		if err != nil {
			return fmt.Errorf("error verifying TOTP code: %w", err)
		}
		if !valid {
			return auth.ErrInvalidMFACode
		}

		// --- Success, finalize setup ---
		method.IsVerified = true
		method.UpdatedAt = time.Now().UTC()
		if err := txRepo.UpdateAuthMethod(ctx, method); err != nil {
			return fmt.Errorf("failed to mark TOTP method as verified: %w", err)
		}

		// Clean up old backup codes before creating new ones.
		_ = txRepo.DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodBackupCode, true) // `true` for verified

		rawBackupCodes := make([]string, 10)
		for i := 0; i < 10; i++ {
			raw, hashed, err := s.tokenGen.Generate()
			if err != nil {
				return fmt.Errorf("failed to generate backup code: %w", err)
			}
			rawBackupCodes[i] = raw
			backupMethod := &auth.AuthMethod{
				ID:         uuid.New(),
				AccountID:  accountID,
				Type:       auth.AuthMethodBackupCode,
				Credential: hashed,
				IsVerified: true,
			}
			if err := txRepo.CreateAuthMethod(ctx, backupMethod); err != nil {
				return fmt.Errorf("failed to store backup code: %w", err)
			}
		}
		backupCodes = rawBackupCodes

		acc.MfaEnabled = true
		acc.UpdatedAt = time.Now().UTC()
		return txRepo.UpdateAccount(ctx, acc)
	})

	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(accountID, "user", core.EventMfaEnabled).
		WithSuccess().
		WithTarget(accountID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return backupCodes, nil
}

// DisableTOTP removes all MFA methods from an account.
func (s *MFAService) DisableTOTP(ctx context.Context, accountID uuid.UUID, params auth.DisableTOTPParams) error {
	acc, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	if !acc.MfaEnabled {
		return auth.ErrMfaNotEnabled
	}

	passMethod, err := s.repo.GetAuthMethod(ctx, accountID, auth.AuthMethodPassword)
	if err != nil {
		return err
	}
	if err := s.passService.Compare(passMethod.Credential, params.Password); err != nil {
		return auth.ErrIncorrectPassword
	}

	totpMethod, err := s.repo.GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP)
	if err != nil || !totpMethod.IsVerified {
		return auth.ErrMfaNotEnabled
	}
	valid, err := s.totpManager.VerifyCode(totpMethod.Credential, params.Code)
	if err != nil {
		return err
	}
	if !valid {
		return auth.ErrInvalidMFACode
	}

	// --- Verification successful, proceed with disabling ---
	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		// Delete all verified MFA-related auth methods.
		if err := txRepo.DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodTOTP, true); err != nil {
			return fmt.Errorf("failed to delete TOTP method: %w", err)
		}
		if err := txRepo.DeleteAuthMethodsByType(ctx, accountID, auth.AuthMethodBackupCode, true); err != nil {
			return fmt.Errorf("failed to delete backup code methods: %w", err)
		}

		acc.MfaEnabled = false
		acc.UpdatedAt = time.Now().UTC()
		return txRepo.UpdateAccount(ctx, acc)
	})
	if err != nil {
		return err
	}

	auditEvent := audit.NewEvent(accountID, "user", core.EventMfaDisabled).
		WithSuccess().
		WithTarget(accountID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return nil
}

// VerifyLoginCode checks a user-provided code against both their TOTP and backup codes.
// It consumes a backup code upon successful use.
func (s *MFAService) VerifyLoginCode(ctx context.Context, accountID uuid.UUID, code string) error {
	// 1. Try to validate as a TOTP code first.
	totpMethod, err := s.repo.GetAuthMethod(ctx, accountID, auth.AuthMethodTOTP)
	if err == nil && totpMethod.IsVerified {
		valid, err := s.totpManager.VerifyCode(totpMethod.Credential, code)
		if err == nil && valid {
			return nil // Success with TOTP
		}
	}

	// 2. If TOTP fails or is not set up, try to find and use a backup code.
	backupMethods, err := s.repo.ListAuthMethodsForAccount(ctx, accountID)
	if err != nil {
		return auth.ErrInvalidMFACode // Return generic error
	}

	for _, method := range backupMethods {
		if method.Type == auth.AuthMethodBackupCode && method.IsVerified {
			// Compare the provided code against the hashed backup code.
			// The password service's `Compare` is generic enough for this.
			if s.tokenGen.Compare(method.Credential, code) {
				// Found a match! Invalidate this backup code so it can't be reused.
				// Fix: internal/services/auth/mfa_service.go:243:5: SA9003: empty branch
				if err := s.repo.DeleteAuthMethod(ctx, method.ID); err != nil {
					s.logger.Error("Failed to consume backup code", err)
				}
				return nil // Success with backup code
			}
		}
	}

	// 3. If neither method worked, the code is invalid.
	return auth.ErrInvalidMFACode
}
