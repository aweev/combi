package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ auth.Service = (*Service)(nil)
var _ privacy.Anonymizer = (*Service)(nil)

type Service struct {
	repo         auth.Repository
	audit        audit.EventRecorder
	passService  security.PasswordService
	tokenGen     security.TokenGenerator
	accessSigner security.TokenSigner
	mfaService   auth.MFAService
	logger       log.Logger
	cfg          config.AuthConfig
	accessTTL    time.Duration
	refreshTTL   time.Duration
	eventBus     events.Bus
	userService  user.Service
	linkConfig   *config.LinkConfig
}

type ServiceDependencies struct {
	Repo         auth.Repository
	Audit        audit.EventRecorder
	PassService  security.PasswordService
	TokenGen     security.TokenGenerator
	AccessSigner security.TokenSigner
	MfaService   auth.MFAService
	Config       config.AuthConfig
	Logger       log.Logger
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	EventBus     events.Bus
	UserService  user.Service
	LinkConfig   *config.LinkConfig
}

func NewService(deps ServiceDependencies) *Service {
	return &Service{
		repo:         deps.Repo,
		audit:        deps.Audit,
		passService:  deps.PassService,
		tokenGen:     deps.TokenGen,
		accessSigner: deps.AccessSigner,
		mfaService:   deps.MfaService,
		logger:       deps.Logger.With(log.String("component", "auth_service")),
		cfg:          deps.Config,
		accessTTL:    deps.AccessTTL,
		refreshTTL:   deps.RefreshTTL,
		eventBus:     deps.EventBus,
		userService:  deps.UserService,
		linkConfig:   deps.LinkConfig,
	}
}

// Register now handles everything, removing the dependency on AccountService
func (s *Service) Register(ctx context.Context, params auth.RegisterParams) (*auth.Account, error) {
	if err := s.passService.ValidateStrength(params.Password); err != nil {
		return nil, err
	}

	existing, _ := s.repo.GetAccountByEmail(ctx, params.Email)
	if existing != nil {
		return nil, auth.ErrAccountExists
	}

	var account *auth.Account
	var rawVerificationToken string

	err := s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		acc := &auth.Account{
			ID:     uuid.New(),
			UserID: uuid.New(),
			Email:  params.Email,
			Status: auth.StatusUnverified,
		}
		if err := txRepo.CreateAccount(ctx, acc); err != nil {
			return err
		}
		account = acc

		hashedPassword, err := s.passService.Hash(params.Password)
		if err != nil {
			return err
		}
		method := &auth.AuthMethod{
			ID:         uuid.New(),
			AccountID:  acc.ID,
			Type:       auth.AuthMethodPassword,
			Credential: hashedPassword,
			IsVerified: true,
		}
		if err := txRepo.CreateAuthMethod(ctx, method); err != nil {
			return err
		}

		rawToken, hashedToken, err := s.tokenGen.Generate()
		if err != nil {
			return err
		}
		rawVerificationToken = rawToken

		token := &auth.Token{
			ID:        uuid.New(),
			AccountID: acc.ID,
			Type:      auth.TokenEmailVerification,
			TokenHash: hashedToken,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}
		return txRepo.CreateToken(ctx, token)
	})

	if err != nil {
		return nil, err
	}

	// This logic now lives here
	verificationURL := strings.Replace(s.linkConfig.EmailVerificationURL, "{{.Token}}", rawVerificationToken, 1)
	payload := auth.UserCreatedPayload{
		UserID:           account.UserID,
		Email:            account.Email,
		VerificationLink: verificationURL,
	}

	event, err := events.NewEvent(core.EventAccountCreated, account.ID, "account", payload)
	if err != nil {
		return nil, err // Logic error
	}

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish auth.account.created event", err, log.String("account_id", account.ID.String()))
	}

	auditEvent := audit.NewEvent(account.ID, "user", core.EventAccountRegistered).
		WithSuccess().
		WithTarget(account.ID, "account", account.Email.String()).
		WithContext(ctx).
		Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record account registration audit event", err)
	}

	return account, nil
}

// Login verifies a user's credentials and returns either a token pair or an MFA challenge.
func (s *Service) Login(ctx context.Context, params auth.LoginParams) (*auth.LoginResult, error) {
	acc, err := s.repo.GetAccountByEmail(ctx, params.Email)
	if err != nil {
		return nil, auth.ErrInvalidCredentials // Catches ErrAccountNotFound
	}

	if acc.LockedUntil != nil && time.Now().UTC().Before(*acc.LockedUntil) {
		return nil, auth.ErrTooManyLoginAttempts
	}

	passMethod, err := s.repo.GetAuthMethod(ctx, acc.ID, auth.AuthMethodPassword)
	if err != nil {
		return nil, auth.ErrInvalidCredentials // User might not have a password method
	}

	if acc.Status == auth.StatusSuspended {
		return nil, auth.ErrAccountSuspended
	}

	if err := s.passService.Compare(passMethod.Credential, params.Password); err != nil {
		s.logger.Warn("Password mismatch",
			log.String("email", params.Email.String()),
			log.Int("provided_len", len(params.Password)),
			log.ErrorAttr(err))

		newCount, incErr := s.repo.IncrementFailedLogin(ctx, acc.ID)
		if incErr != nil {
			s.logger.Error("failed to increment failed login", incErr)
		}
		if newCount >= s.cfg.MaxFailedLogins {
			if lockErr := s.repo.LockAccount(ctx, acc.ID, s.cfg.LockoutDuration); lockErr != nil {
				s.logger.Error("failed to lock account", lockErr)
			}
		}

		auditEvent := audit.NewEvent(acc.ID, "user", core.EventLoginFailed).
			WithFailure("Incorrect password").
			WithContext(ctx).Build()
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Error("failed to record failed login audit event", err)
		}
		return nil, auth.ErrInvalidCredentials
	}

	if err := s.repo.ResetFailedLogin(ctx, acc.ID); err != nil {
		s.logger.Error("failed to reset failed login counter", err)
	}

	if acc.Status == auth.StatusUnverified {
		return nil, auth.ErrAccountNotVerified
	}

	if acc.MfaEnabled {
		challengeToken, err := s.createMfaChallengeToken(acc.ID)
		if err != nil {
			return nil, err
		}
		auditEvent := audit.NewEvent(acc.ID, "user", core.EventLoginMfaRequired).
			WithSuccess().WithContext(ctx).Build()
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Error("failed to record MFA required audit event", err)
		}
		return &auth.LoginResult{MfaChallengeToken: challengeToken, MFAIsRequired: true}, nil
	}

	tokenPair, err := s.CreateSessionForAccount(ctx, acc, params.IPAddress, params.UserAgent, nil)
	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(acc.ID, "user", core.EventLoginSuccess).
		WithSuccess().WithContext(ctx).Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record login success audit event", err)
	}

	return &auth.LoginResult{TokenPair: tokenPair, MFAIsRequired: false}, nil
}

// LoginMFA completes an MFA login flow.
func (s *Service) LoginMFA(ctx context.Context, params auth.LoginMFAParams) (*auth.TokenPair, error) {
	claims, err := s.accessSigner.Verify(params.MfaChallengeToken)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	if purpose, _ := claims["purpose"].(string); purpose != "mfa_challenge" {
		return nil, auth.ErrInvalidTokenType
	}
	accountIDStr, _ := claims["sub"].(string)
	accountID, _ := uuid.Parse(accountIDStr)

	if err := s.mfaService.VerifyLoginCode(ctx, accountID, params.MfaCode); err != nil {
		auditEvent := audit.NewEvent(accountID, "user", core.EventLoginFailed).
			WithFailure("Invalid MFA code").
			WithContext(ctx).Build()
		if err := s.audit.Record(context.Background(), auditEvent); err != nil {
			s.logger.Error("failed to record failed MFA login audit event", err)
		}
		return nil, err
	}

	acc, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	tokenPair, err := s.CreateSessionForAccount(ctx, acc, params.IPAddress, params.UserAgent, nil)
	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(acc.ID, "user", core.EventLoginSuccess).
		WithSuccess().WithMetadata(map[string]interface{}{"mfa_used": true}).
		WithContext(ctx).Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record MFA login success audit event", err)
	}

	return tokenPair, nil
}

func (s *Service) RequestMagicLink(ctx context.Context, email core.Email, ip, ua string) error {
	acc, err := s.repo.GetAccountByEmail(ctx, email)
	if err != nil {
		s.logger.Debug("Magic link requested for non-existent account", log.String("email", email.String()))
		return nil
	}

	rawToken, hashedToken, err := s.tokenGen.Generate()
	if err != nil {
		return err
	}

	token := &auth.Token{
		ID:        uuid.New(),
		AccountID: acc.ID,
		Type:      auth.TokenMagicLinkLogin,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	if err := s.repo.CreateToken(ctx, token); err != nil {
		return err
	}

	// DYNAMICALLY BUILD THE LINK
	loginURL := strings.Replace(s.linkConfig.PasswordResetURL, "{{.Token}}", rawToken, 1)

	payload := auth.MagicLinkRequestedPayload{
		UserID:    acc.UserID,
		Email:     acc.Email,
		LoginLink: loginURL,
		IPAddress: ip,
		UserAgent: ua,
	}

	event, err := events.NewEvent(core.EventMagicLinkRequested, acc.ID, "account", payload)
	if err != nil {
		return err
	}

	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("Failed to publish magic link requested event", err, log.String("account_id", acc.ID.String()))
	}

	return nil
}

// LoginWithMagicLink validates a magic link token and creates a new session.
func (s *Service) LoginWithMagicLink(ctx context.Context, token string, ip, ua string) (*auth.TokenPair, error) {
	hashedToken := s.tokenGen.Hash(token)

	// 1. Get the token, which implicitly checks for usage and expiry.
	mlToken, err := s.repo.GetTokenByHash(ctx, hashedToken, auth.TokenMagicLinkLogin)
	if err != nil {
		return nil, auth.ErrInvalidToken // Return a generic error
	}

	var tokenPair *auth.TokenPair
	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		// 2. Mark the token as used within the transaction.
		if err := txRepo.UseToken(ctx, mlToken.ID); err != nil {
			return err
		}

		// 3. Fetch the associated account.
		acc, err := txRepo.GetAccountByID(ctx, mlToken.AccountID)
		if err != nil {
			return err
		}

		// 4. Create the session and token pair.
		pair, err := s.CreateSessionForAccount(ctx, acc, ip, ua, nil)
		if err != nil {
			return err
		}
		tokenPair = pair
		return nil
	})

	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(mlToken.AccountID, "user", core.EventLoginSuccess).
		WithSuccess().
		WithMetadata(map[string]interface{}{"method": "magic_link"}).
		WithContext(ctx).
		Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record magic link login audit event", err)
	}

	return tokenPair, nil
}

// Logout invalidates a session by deleting it from the database.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	hashedToken := s.tokenGen.Hash(refreshToken)
	session, err := s.repo.GetSessionByRefreshTokenHash(ctx, hashedToken)
	if err != nil {
		return nil // Idempotent: if session doesn't exist, it's already logged out.
	}

	if err := s.repo.DeleteSession(ctx, session.ID); err != nil {
		return err
	}

	auditEvent := audit.NewEvent(session.AccountID, "user", core.EventLogout).
		WithSuccess().WithContext(ctx).Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record logout audit event", err)
	}

	return nil
}

// RefreshToken implements refresh token rotation for enhanced security.
func (s *Service) RefreshToken(ctx context.Context, params auth.RefreshTokenParams) (*auth.TokenPair, error) {
	hashedToken := s.tokenGen.Hash(params.RefreshToken)

	session, err := s.repo.FindSessionByRefreshTokenHash(ctx, hashedToken)
	if err != nil {
		return nil, auth.ErrInvalidRefreshToken
	}

	if session.RevokedAt != nil {
		s.logger.Error("CRITICAL: Reused refresh token detected. Revoking all sessions for user.", err,
			log.String("event", "security_alert"),
			log.String("account_id", session.AccountID.String()),
			log.String("ip_address", params.IPAddress),
		)
		_ = s.repo.RevokeAllForAccount(ctx, session.AccountID)
		return nil, auth.ErrInvalidRefreshToken
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.repo.DeleteSession(ctx, session.ID) // Clean up expired session
		return nil, auth.ErrSessionExpired
	}

	acc, err := s.repo.GetAccountByID(ctx, session.AccountID)
	if err != nil {
		return nil, auth.ErrAccountNotFound
	}

	// --- Refresh Token Rotation ---
	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		if err := txRepo.RevokeSession(ctx, session.ID); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	tokenPair, err := s.CreateSessionForAccount(ctx, acc, params.IPAddress, params.UserAgent, nil)
	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(acc.ID, "user", core.EventTokenRefreshed).
		WithSuccess().
		WithContext(ctx).
		Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record token refresh audit event", err)
	}

	return tokenPair, nil
}

// VerifyAccessToken is a simple wrapper around the token signer's verify method.
func (s *Service) VerifyAccessToken(ctx context.Context, accessToken string) (*auth.Actor, error) {
	claims, err := s.accessSigner.Verify(accessToken)
	if err != nil {
		return nil, err
	}

	actor := &auth.Actor{}
	if sub, ok := claims["sub"].(string); ok {
		actor.AccountID, _ = uuid.Parse(sub)
	}
	if uid, ok := claims["uid"].(string); ok {
		actor.UserID, _ = uuid.Parse(uid)
	}
	if sid, ok := claims["sid"].(string); ok {
		actor.SessionID, _ = uuid.Parse(sid)
	}
	actor.Email, _ = claims["eml"].(string)

	if actor.AccountID == uuid.Nil || actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return nil, auth.ErrInvalidToken
	}

	if imp, ok := claims["imp"].(string); ok {
		impersonatorID, _ := uuid.Parse(imp)
		if impersonatorID != uuid.Nil {
			actor.ImpersonatorID = &impersonatorID
		}
	}

	return actor, nil
}

// --- Private Helper Methods ---

func (s *Service) createMfaChallengeToken(accountID uuid.UUID) (string, error) {
	claims := map[string]interface{}{
		"sub":     accountID.String(),
		"purpose": "mfa_challenge",
	}
	return s.accessSigner.Sign(claims, 5*time.Minute)
}

func (s *Service) CreateSessionForAccount(ctx context.Context, acc *auth.Account, ip, ua string, impersonatorID *uuid.UUID) (*auth.TokenPair, error) {
	rawRefreshToken, _, err := s.tokenGen.Generate()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// Build device fingerprint from IP + User-Agent (no need for raw http.Request).
	dfp := hashDevice(ip + "|" + ua)
	session := &auth.Session{
		ID:                    uuid.New(),
		AccountID:             acc.ID,
		RefreshTokenHash:      s.tokenGen.Hash(rawRefreshToken),
		DeviceFingerprintHash: dfp,
		IPAddress:             ip,
		UserAgent:             ua,
		ExpiresAt:             now.Add(s.refreshTTL),
		LastUsedAt:            now,
		CreatedAt:             now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	accessClaims := map[string]interface{}{
		"sub": acc.ID.String(),
		"uid": acc.UserID.String(),
		"eml": acc.Email.String(),
		"sid": session.ID.String(),
	}

	if impersonatorID != nil {
		accessClaims["imp"] = impersonatorID.String()
	}

	accessToken, err := s.accessSigner.Sign(accessClaims, s.accessTTL)
	if err != nil {
		return nil, err
	}

	// Fetch user profile to include in the token response.
	var usr *user.User
	if s.userService != nil {
		usr, _ = s.userService.GetUser(ctx, acc.UserID)
	}

	return &auth.TokenPair{
		AccessToken:          accessToken,
		RefreshToken:         rawRefreshToken,
		AccessTokenExpiresAt: now.Add(s.accessTTL),
		User:                 usr,
	}, nil
}

func hashDevice(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (s *Service) SetUserService(userService user.Service) {
	s.userService = userService
}

// AnonymizeUser implements the Right to be Forgotten for Authentication credentials.
// It effectively "bricks" the account so it can never be logged into again,
// while preserving the ID for relational integrity (if needed) or deleting it.
func (s *Service) AnonymizeUser(ctx context.Context, userID uuid.UUID) (*privacy.ErasureReport, error) {
	report := &privacy.ErasureReport{
		Service: "AuthService",
	}

	// 1. Find the account associated with this user
	// Note: We need a method in repo to find by UserID, usually available.
	// Assuming s.repo.GetAccountByUserID exists, or we list methods.
	// If not, we might need to rely on the User Service passing the AccountID,
	// but the Interface only gives us UserID.
	// Ideally: repo.GetAccountByUserID(ctx, userID)

	// For this implementation, let's assume we can get it or we do a direct delete by user_id
	// depending on your schema. Based on your schema: Account has `user_id`.

	// STRATEGY: Delete the account entirely.
	// If we delete the account, cascading deletes in DB removes AuthMethods, Sessions, Tokens.

	// We need to add DeleteAccountByUserID to the Repository interface if it's missing.
	// Assuming s.repo.DeleteAccountByUserID(ctx, userID)

	err := s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		// TODO: Implement this method in your Postgres Repo
		return txRepo.DeleteAccountByUserID(ctx, userID)
	})

	if err != nil {
		return nil, fmt.Errorf("auth anonymization failed: %w", err)
	}

	report.RecordsDeleted = 1 // The account record
	report.Details = "Account and all credentials permanently deleted."
	return report, nil
}
