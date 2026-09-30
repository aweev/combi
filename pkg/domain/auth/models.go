package auth

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// --- Enums ---

type AccountStatus string

const (
	StatusUnverified AccountStatus = "UNVERIFIED" // User has registered but not confirmed their email.
	StatusActive     AccountStatus = "ACTIVE"     // User is fully active and can log in.
	StatusSuspended  AccountStatus = "SUSPENDED"  // User is blocked by an admin.
	StatusPending    AccountStatus = "PENDING"
)

type AuthMethodType string

const (
	AuthMethodPassword   AuthMethodType = "PASSWORD"
	AuthMethodTOTP       AuthMethodType = "TOTP" // Time-based One-Time Password
	AuthMethodBackupCode AuthMethodType = "BACKUP_CODE"
	AuthMethodOAuth      AuthMethodType = "OAUTH"
)

type AuthMethod struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	Type       AuthMethodType
	Provider   *string // ADDED: e.g., "google", "github"
	Credential string  // For OAUTH: this will be the provider's unique user ID (the 'sub' claim)
	IsVerified bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
type TokenType string

const (
	TokenEmailVerification TokenType = "EMAIL_VERIFICATION"
	TokenPasswordReset     TokenType = "PASSWORD_RESET"
	TokenMfaChallenge      TokenType = "MFA_CHALLENGE"
	TokenMagicLinkLogin    TokenType = "MAGIC_LINK_LOGIN"
)

// --- Core Models ---

type Account struct {
	ID               uuid.UUID     `db:"id"`
	UserID           uuid.UUID     `db:"user_id"` // The critical link to the user domain
	Email            core.Email    `db:"email"`
	Status           AccountStatus `db:"status"`
	MfaEnabled       bool          `db:"mfa_enabled"`
	FailedLoginCount int           `db:"failed_login_count"`
	LockedUntil      *time.Time    `db:"locked_until"`
	CreatedAt        time.Time     `db:"created_at"`
	UpdatedAt        time.Time     `db:"updated_at"`
}

// Session represents a user's authenticated session, managed via refresh tokens.
type Session struct {
	ID                    uuid.UUID
	AccountID             uuid.UUID
	RefreshTokenHash      string
	DeviceFingerprintHash string
	IPAddress             string
	UserAgent             string
	ExpiresAt             time.Time
	LastUsedAt            time.Time
	CreatedAt             time.Time
	RevokedAt             *time.Time
}

// Token is a single-use, short-lived token for actions like password reset or email verification.
type Token struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Type      TokenType
	TokenHash string // Stored in the database, never sent to the client.
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// --- Actor Context ---

// Actor is the verified identity of the entity performing an action. It is placed
// in the context by authentication middleware and used by all other services.
type Actor struct {
	AccountID      uuid.UUID
	UserID         uuid.UUID
	Email          string
	SessionID      uuid.UUID
	ImpersonatorID *uuid.UUID
}

type contextKey string

const actorKey = contextKey("actor")

// NewContextWithActor returns a new context carrying the verified Actor.
func NewContextWithActor(ctx context.Context, actor *Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFromContext retrieves the Actor from the context. Returns nil if not found.
func ActorFromContext(ctx context.Context) *Actor {
	actor, _ := ctx.Value(actorKey).(*Actor)
	return actor
}
