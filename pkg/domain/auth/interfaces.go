package auth

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// Service handles the primary user-facing authentication flows.
type Service interface {
	Register(ctx context.Context, params RegisterParams) (*Account, error)
	Login(ctx context.Context, params LoginParams) (*LoginResult, error)
	LoginMFA(ctx context.Context, params LoginMFAParams) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	RefreshToken(ctx context.Context, params RefreshTokenParams) (*TokenPair, error)
	VerifyAccessToken(ctx context.Context, accessToken string) (*Actor, error)
	CreateSessionForAccount(ctx context.Context, acc *Account, ip, ua string, impersonatorID *uuid.UUID) (*TokenPair, error)
	RequestMagicLink(ctx context.Context, email core.Email, ip, ua string) error
	LoginWithMagicLink(ctx context.Context, token string, ip, ua string) (*TokenPair, error)
	SetUserService(userService user.Service)
}

// AccountService manages the lifecycle and state of an account, such as verification and password changes.
type AccountService interface {
	RequestEmailVerification(ctx context.Context, email core.Email) error
	VerifyEmail(ctx context.Context, verificationToken string) error
	RequestPasswordReset(ctx context.Context, email core.Email) error
	CompletePasswordReset(ctx context.Context, params CompletePasswordResetParams) error
	ChangePassword(ctx context.Context, actorID uuid.UUID, params ChangePasswordParams) error
	VerifyResetToken(ctx context.Context, token string) (string, error)
}

// MFAService manages Multi-Factor Authentication for an account.
type MFAService interface {
	SetupTOTP(ctx context.Context, accountID uuid.UUID) (*SetupTOTPResult, error)
	EnableTOTP(ctx context.Context, accountID uuid.UUID, code string) ([]string, error) // Returns backup codes on success
	DisableTOTP(ctx context.Context, accountID uuid.UUID, params DisableTOTPParams) error
	VerifyLoginCode(ctx context.Context, accountID uuid.UUID, code string) error
}

// SessionService provides functionality for managing an account's active sessions.
type SessionService interface {
	List(ctx context.Context, accountID uuid.UUID) ([]*Session, error)
	Revoke(ctx context.Context, actorID, sessionToRevokeID uuid.UUID) error
}

type AdminService interface {
	UpdateAccountStatus(ctx context.Context, accountID uuid.UUID, status AccountStatus, reason *string) (*Account, error)
	AdminUpdateAccountEmail(ctx context.Context, accountID uuid.UUID, newEmail string) error
	AdminInvalidateAllSessionsForAccount(ctx context.Context, accountID uuid.UUID) error
	Impersonate(ctx context.Context, targetAccountID uuid.UUID, reason string) (*TokenPair, error)
}

type ClaimService interface {
	Search(ctx context.Context, identifier string) (found bool, user *FoundUser, err error)
	SendActivationCode(ctx context.Context, accountID uuid.UUID) error
}

// --- DTOs for Service Methods ---

type FoundUser struct {
	ID          uuid.UUID
	FirstName   string
	MaskedEmail string
}

type RegisterParams struct {
	Email           core.Email `validate:"required,email"`
	Password        string     `validate:"required,min=8"`
	Role            string     `validate:"required,oneof=volunteer donor alumni"` // Corresponds to frontend roles
	FirstName       string     `validate:"required"`
	LastName        string     `validate:"required"`
	TermsAccepted   bool       `validate:"required,eq=true"` // Must be true
	GDPRConsent     bool       `validate:"required,eq=true"` // Must be true
	NewsletterOptIn bool       // Optional
	IPAddress       string
	UserAgent       string
}

type LoginParams struct {
	Email     core.Email `validate:"required,email"`
	Password  string     `validate:"required"`
	IPAddress string
	UserAgent string
}

type LoginMFAParams struct {
	MfaChallengeToken string `validate:"required"` // The short-lived token returned from a successful password login
	MfaCode           string `validate:"required,len=6"`
	IPAddress         string
	UserAgent         string
}

type RefreshTokenParams struct {
	RefreshToken string `validate:"required"`
	IPAddress    string
	UserAgent    string
}

type CompletePasswordResetParams struct {
	ResetToken  string `validate:"required"`
	NewPassword string `validate:"required,min=8"`
}

type ChangePasswordParams struct {
	OldPassword string `validate:"required"`
	NewPassword string `validate:"required,min=8"`
}

type DisableTOTPParams struct {
	Password string `validate:"required"`
	Code     string `validate:"required,len=6"`
}

type LoginResult struct {
	TokenPair         *TokenPair // Populated on successful login without MFA
	MfaChallengeToken string     // Populated if MFA is required
	MFAIsRequired     bool
}

type TokenPair struct {
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt time.Time
	User                 *user.User
}

type SetupTOTPResult struct {
	Secret         string // Base32 encoded secret for manual entry
	QRCodeImageURL string // Data URL for QR code image
}

// --- Platform & Infrastructure Interfaces ---

type OAuthService interface {
	// Generates the redirect URL for the user to start the OAuth flow.
	Initiate(ctx context.Context, provider string) (redirectURL, state string, err error)
	// Handles the callback from the provider, exchanges the code for tokens, and logs the user in.
	HandleCallback(ctx context.Context, params HandleCallbackParams) (*LoginResult, error)
	SetUserService(userService user.Service)
}

type HandleCallbackParams struct {
	Provider  string
	Code      string
	State     string
	IPAddress string
	UserAgent string
}

type OAuthProvider interface {
	GetAuthCodeURL(state string) string
	ExchangeCodeForUserInfo(ctx context.Context, code string) (*OAuthUserInfo, error)
}

type OAuthUserInfo struct {
	ProviderUserID  string // The unique ID from the provider (e.g., Google's 'sub')
	Email           core.Email
	IsEmailVerified bool
	Name            string
	PictureURL      string
	RawData         map[string]interface{}
}
