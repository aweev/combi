package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Statically verify that it implements the domain interface.
var _ auth.AccountService = (*AccountService)(nil)

// AccountService manages account lifecycle actions like verification and password changes.
type AccountService struct {
	repo         auth.Repository
	audit        audit.EventRecorder
	passService  security.PasswordService
	tokenGen     security.TokenGenerator
	notifService notification.Service
	eventBus     events.Bus
	linkConfig   *config.LinkConfig
	logger       log.Logger
}

// NewAccountService creates a new account management service.
func NewAccountService(
	repo auth.Repository,
	auditRecorder audit.EventRecorder,
	passService security.PasswordService,
	tokenGen security.TokenGenerator,
	notifService notification.Service,
	eventBus events.Bus,
	linkConfig *config.LinkConfig,
	logger log.Logger,
) *AccountService {
	return &AccountService{
		repo:         repo,
		audit:        auditRecorder,
		passService:  passService,
		tokenGen:     tokenGen,
		notifService: notifService,
		eventBus:     eventBus,
		linkConfig:   linkConfig,
		logger:       logger.With(log.String("component", "auth_account_service")),
	}
}

// RequestEmailVerification generates and sends a verification token.
func (s *AccountService) RequestEmailVerification(ctx context.Context, email core.Email) error {
	acc, err := s.repo.GetAccountByEmail(ctx, email)
	if err != nil || acc == nil || acc.Status != auth.StatusUnverified {
		return nil
	}

	rawToken, hashedToken, err := s.tokenGen.Generate()
	if err != nil {
		return err
	}

	token := &auth.Token{
		ID:        uuid.New(),
		AccountID: acc.ID,
		Type:      auth.TokenEmailVerification,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := s.repo.CreateToken(ctx, token); err != nil {
		return err
	}

	verificationURL := strings.Replace(s.linkConfig.EmailVerificationURL, "{{.Token}}", rawToken, 1)

	// Use Domain Event Structure
	payload := auth.UserCreatedPayload{
		UserID:           acc.UserID,
		Email:            acc.Email,
		Name:             "", // Not available on Account model
		VerificationLink: verificationURL,
	}

	event, err := events.NewEvent(core.EventAccountCreated, acc.ID, "account", payload)
	if err != nil {
		// Log but allow process to continue? No, event creation failure is code error.
		return err
	}

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish auth.account.created event (re-request)",
			err,
			log.String("account_id", acc.ID.String()))
	}

	return nil
}

// VerifyEmail consumes a verification token, and if valid, activates the user's account.
func (s *AccountService) VerifyEmail(ctx context.Context, verificationToken string) error {
	hashedToken := s.tokenGen.Hash(verificationToken)

	token, err := s.repo.GetTokenByHash(ctx, hashedToken, auth.TokenEmailVerification)
	if err != nil {
		return err // Maps to ErrTokenNotFound
	}
	if token.UsedAt != nil {
		return auth.ErrTokenAlreadyUsed
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return auth.ErrTokenExpired
	}

	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		acc, err := txRepo.GetAccountByID(ctx, token.AccountID)
		if err != nil {
			return err
		}
		if acc.Status != auth.StatusUnverified {
			return auth.ErrEmailAlreadyVerified
		}

		acc.Status = auth.StatusActive
		acc.UpdatedAt = time.Now().UTC()
		if err := txRepo.UpdateAccount(ctx, acc); err != nil {
			return err
		}

		return txRepo.UseToken(ctx, token.ID)
	})

	if err != nil {
		return err
	}

	auditEvent := audit.NewEvent(token.AccountID, "user", core.EventEmailVerified).
		WithSuccess().
		WithTarget(token.AccountID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record email verification audit event", err)
	}

	return nil
}

// RequestPasswordReset generates and sends a password reset token.
func (s *AccountService) RequestPasswordReset(ctx context.Context, email core.Email) error {
	acc, err := s.repo.GetAccountByEmail(ctx, email)
	if err != nil {
		return nil // Do not reveal if the email exists.
	}

	rawToken, hashedToken, err := s.tokenGen.Generate()
	if err != nil {
		return err
	}

	token := &auth.Token{
		ID:        uuid.New(),
		AccountID: acc.ID,
		Type:      auth.TokenPasswordReset,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := s.repo.CreateToken(ctx, token); err != nil {
		return err
	}

	resetURL := strings.Replace(s.linkConfig.PasswordResetURL, "{{.Token}}", rawToken, 1)

	payload := auth.PasswordResetRequestedPayload{
		UserID:    acc.UserID,
		Email:     acc.Email,
		Name:      "",
		ResetLink: resetURL,
	}

	event, err := events.NewEvent(core.EventPasswordResetReq, acc.ID, "account", payload)
	if err != nil {
		return err
	}

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish auth.password.reset_requested event",
			err,
			log.String("account_id", acc.ID.String()))
	}

	auditEvent := audit.NewEvent(acc.ID, "user", core.EventPasswordResetReq).
		WithSuccess().
		WithTarget(acc.ID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record password reset request audit event", err)
	}

	return nil
}

// CompletePasswordReset consumes a reset token and sets a new password for the account.
func (s *AccountService) CompletePasswordReset(ctx context.Context, params auth.CompletePasswordResetParams) error {
	if err := s.passService.ValidateStrength(params.NewPassword); err != nil {
		return err
	}

	hashedToken := s.tokenGen.Hash(params.ResetToken)
	token, err := s.repo.GetTokenByHash(ctx, hashedToken, auth.TokenPasswordReset)
	if err != nil {
		return err
	}
	if token.UsedAt != nil {
		return auth.ErrTokenAlreadyUsed
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return auth.ErrTokenExpired
	}

	newHash, err := s.passService.Hash(params.NewPassword)
	if err != nil {
		return err
	}

	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		method, err := txRepo.GetAuthMethod(ctx, token.AccountID, auth.AuthMethodPassword)
		if err != nil {
			return err
		}
		method.Credential = newHash
		method.UpdatedAt = time.Now().UTC()
		if err := txRepo.UpdateAuthMethod(ctx, method); err != nil {
			return err
		}
		return txRepo.UseToken(ctx, token.ID)
	})

	if err != nil {
		return err
	}

	auditEvent := audit.NewEvent(token.AccountID, "user", core.EventPasswordReset).
		WithSuccess().
		WithTarget(token.AccountID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record password reset completion audit event", err)
	}

	return nil
}

// ChangePassword allows an authenticated user to change their own password.
func (s *AccountService) ChangePassword(ctx context.Context, actorID uuid.UUID, params auth.ChangePasswordParams) error {
	if params.OldPassword == params.NewPassword {
		return auth.ErrSamePassword
	}
	if err := s.passService.ValidateStrength(params.NewPassword); err != nil {
		return err
	}

	method, err := s.repo.GetAuthMethod(ctx, actorID, auth.AuthMethodPassword)
	if err != nil {
		return err
	}

	if err := s.passService.Compare(method.Credential, params.OldPassword); err != nil {
		return auth.ErrIncorrectPassword
	}

	newHash, err := s.passService.Hash(params.NewPassword)
	if err != nil {
		return err
	}
	method.Credential = newHash
	method.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateAuthMethod(ctx, method); err != nil {
		return err
	}

	auditEvent := audit.NewEvent(actorID, "user", core.EventPasswordChanged).
		WithSuccess().
		WithTarget(actorID, "account", "").
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record password change audit event", err)
	}

	return nil
}

func (s *AccountService) VerifyResetToken(ctx context.Context, rawToken string) (string, error) {
	hashedToken := s.tokenGen.Hash(rawToken)

	token, err := s.repo.GetTokenByHash(ctx, hashedToken, auth.TokenPasswordReset)
	if err != nil {
		return "", err // Token not found or expired
	}

	if token.UsedAt != nil {
		return "", auth.ErrTokenAlreadyUsed
	}

	if time.Now().UTC().After(token.ExpiresAt) {
		return "", auth.ErrTokenExpired
	}

	// Get associated account to return email context
	acc, err := s.repo.GetAccountByID(ctx, token.AccountID)
	if err != nil {
		return "", err
	}

	return acc.Email.String(), nil
}
