package auth

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// Repository is the master persistence interface for the auth domain.
// It composes smaller, more focused repository interfaces to enable transaction management.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error
	AccountRepository
	AuthMethodRepository
	SessionRepository
	TokenRepository
}

// AccountRepository manages persistence for Account entities.
type AccountRepository interface {
	// Note: Method names are prefixed with the entity name to avoid conflicts.
	CreateAccount(ctx context.Context, account *Account) error
	GetAccountByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetAccountByEmail(ctx context.Context, email core.Email) (*Account, error)
	UpdateAccount(ctx context.Context, account *Account) error
	IncrementFailedLogin(ctx context.Context, accountID uuid.UUID) (newCount int, err error)
	LockAccount(ctx context.Context, accountID uuid.UUID, duration time.Duration) error
	ResetFailedLogin(ctx context.Context, accountID uuid.UUID) error
	DeleteAccountByUserID(ctx context.Context, userID uuid.UUID) error
	GetAccountByIdentifier(ctx context.Context, identifier string) (*Account, error)
}

// AuthMethodRepository manages persistence for AuthMethod entities.
type AuthMethodRepository interface {
	CreateAuthMethod(ctx context.Context, method *AuthMethod) error
	GetAuthMethod(ctx context.Context, accountID uuid.UUID, methodType AuthMethodType) (*AuthMethod, error)
	GetAuthMethodByProvider(ctx context.Context, provider, providerUserID string) (*AuthMethod, error)
	ListAuthMethodsForAccount(ctx context.Context, accountID uuid.UUID) ([]*AuthMethod, error)
	UpdateAuthMethod(ctx context.Context, method *AuthMethod) error
	DeleteAuthMethodsByType(ctx context.Context, accountID uuid.UUID, methodType AuthMethodType, isVerified bool) error
	DeleteAuthMethod(ctx context.Context, methodID uuid.UUID) error

	GetAccountByIDForUpdate(ctx context.Context, id uuid.UUID) (*Account, error)
}

// SessionRepository manages persistence for Session entities.
type SessionRepository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByID(ctx context.Context, id uuid.UUID) (*Session, error)
	GetSessionByRefreshTokenHash(ctx context.Context, hash string) (*Session, error)
	FindSessionByRefreshTokenHash(ctx context.Context, hash string) (*Session, error)
	ListSessionsForAccount(ctx context.Context, accountID uuid.UUID) ([]*Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeAllForAccount(ctx context.Context, accountID uuid.UUID) error
	DeleteExpiredSessions(ctx context.Context, cutoff time.Time) (int64, error)
}

// TokenRepository manages persistence for single-use Token entities.
type TokenRepository interface {
	CreateToken(ctx context.Context, token *Token) error
	GetTokenByHash(ctx context.Context, hash string, tokenType TokenType) (*Token, error)
	UseToken(ctx context.Context, id uuid.UUID) error // Marks a token as used.
	DeleteExpiredTokens(ctx context.Context, cutoff time.Time) (int64, error)
}
