// internal/persistence/postgres/auth_repository.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Statically verify implementation.
var _ auth.Repository = (*AuthRepository)(nil)

type AuthRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewAuthRepository(db *sqlx.DB, logger log.Logger) *AuthRepository {
	return &AuthRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "auth_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *AuthRepository) WithTx(ctx context.Context, fn func(txRepo auth.Repository) error) (err error) {
	_, isTx := r.runner.(*sqlx.Tx)
	if isTx {
		return errors.New("cannot begin transaction from within another transaction")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
			}
		} else {
			err = tx.Commit()
		}
	}()

	txRepo := &AuthRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	return fn(txRepo)
}

// --- AccountRepository Implementation ---

const createAccountQuery = `
	INSERT INTO accounts (id, user_id, email, status, mfa_enabled, failed_login_count, locked_until, created_at, updated_at)
	VALUES (:id, :user_id, :email, :status, :mfa_enabled, :failed_login_count, :locked_until, :created_at, :updated_at)`

func (r *AuthRepository) CreateAccount(ctx context.Context, account *auth.Account) error {
	dbModel := dbAccountFromDomain(account)
	_, err := sqlx.NamedExecContext(ctx, r.runner, createAccountQuery, dbModel)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return auth.ErrAccountExists
		}
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetAccountByID(ctx context.Context, id uuid.UUID) (*auth.Account, error) {
	return r.getAccount(ctx, "id", id)
}

func (r *AuthRepository) GetAccountByEmail(ctx context.Context, email core.Email) (*auth.Account, error) {
	query, args, err := r.sq.Select("*").From("accounts").
		Where("LOWER(email) = LOWER(?)", email.String()).ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get account by email query: %w", err)
	}
	var dba dbAccount
	if err := sqlx.GetContext(ctx, r.runner, &dba, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrAccountNotFound
		}

		return nil, fmt.Errorf("failed to get account by email: %w", err)
	}
	return dba.toDomain()
}

const updateAccountQuery = `
	UPDATE accounts SET
		email = :email,
		status = :status,
		mfa_enabled = :mfa_enabled,
		failed_login_count = :failed_login_count,
		locked_until = :locked_until,
		updated_at = :updated_at
	WHERE id = :id`

func (r *AuthRepository) UpdateAccount(ctx context.Context, account *auth.Account) error {
	account.UpdatedAt = time.Now().UTC()
	dbModel := dbAccountFromDomain(account)
	res, err := sqlx.NamedExecContext(ctx, r.runner, updateAccountQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to update account %s: %w", account.ID, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrAccountNotFound
	}
	return nil
}

func (r *AuthRepository) IncrementFailedLogin(ctx context.Context, accountID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Update("accounts").
		Set("failed_login_count", squirrel.Expr("failed_login_count + 1")).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": accountID}).
		Suffix("RETURNING failed_login_count"). // Atomically return the new value
		ToSql()

	var newCount int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&newCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, auth.ErrAccountNotFound
		}
		return 0, err
	}
	return newCount, nil
}

func (r *AuthRepository) LockAccount(ctx context.Context, accountID uuid.UUID, duration time.Duration) error {
	lockedUntil := time.Now().UTC().Add(duration)
	query, args, _ := r.sq.Update("accounts").
		Set("locked_until", lockedUntil).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": accountID}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if rows, _ := res.RowsAffected(); err == nil && rows == 0 {
		return auth.ErrAccountNotFound
	}
	return err
}

func (r *AuthRepository) ResetFailedLogin(ctx context.Context, accountID uuid.UUID) error {
	query, args, _ := r.sq.Update("accounts").
		Set("failed_login_count", 0).
		Set("locked_until", nil).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": accountID}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if rows, _ := res.RowsAffected(); err == nil && rows == 0 {
		return auth.ErrAccountNotFound
	}
	return err
}

// --- AuthMethodRepository Implementation ---

const createAuthMethodQuery = `
	INSERT INTO auth_methods (id, account_id, type, provider, credential, verified, created_at, updated_at)
	VALUES (:id, :account_id, :type, :provider, :credential, :verified, :created_at, :updated_at)`

func (r *AuthRepository) CreateAuthMethod(ctx context.Context, method *auth.AuthMethod) error {
	dbModel := dbAuthMethodFromDomain(method)
	_, err := sqlx.NamedExecContext(ctx, r.runner, createAuthMethodQuery, dbModel)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return auth.ErrAuthMethodExists
		}
		return fmt.Errorf("failed to create auth method: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetAuthMethod(ctx context.Context, accountID uuid.UUID, methodType auth.AuthMethodType) (*auth.AuthMethod, error) {
	if methodType == auth.AuthMethodOAuth {
		return nil, errors.New("GetAuthMethod is ambiguous for OAUTH type; use a more specific finder")
	}

	mType := strings.ToUpper(string(methodType))

	query, args, _ := r.sq.Select("*").From("auth_methods").
		Where(squirrel.Eq{"account_id": accountID, "type": mType}).ToSql()

	var dbm dbAuthMethod
	if err := sqlx.GetContext(ctx, r.runner, &dbm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Warn("Auth method not found",
				log.String("account_id", accountID.String()),
				log.String("type", mType))
			return nil, auth.ErrAuthMethodNotFound
		}
		return nil, fmt.Errorf("failed to get auth method: %w", err)
	}
	return dbm.toDomain(), nil
}

func (r *AuthRepository) GetAuthMethodByProvider(ctx context.Context, provider, providerUserID string) (*auth.AuthMethod, error) {
	query, args, _ := r.sq.Select("*").From("auth_methods").
		Where(squirrel.Eq{
			"type":       auth.AuthMethodOAuth,
			"provider":   provider,
			"credential": providerUserID,
		}).ToSql()

	var dbm dbAuthMethod
	if err := sqlx.GetContext(ctx, r.runner, &dbm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrAuthMethodNotFound
		}
		return nil, fmt.Errorf("failed to get auth method by provider: %w", err)
	}
	return dbm.toDomain(), nil
}

func (r *AuthRepository) ListAuthMethodsForAccount(ctx context.Context, accountID uuid.UUID) ([]*auth.AuthMethod, error) {
	query, args, _ := r.sq.Select("*").From("auth_methods").
		Where(squirrel.Eq{"account_id": accountID}).
		OrderBy("created_at ASC").ToSql()

	var dbMethods []dbAuthMethod
	if err := sqlx.SelectContext(ctx, r.runner, &dbMethods, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list auth methods for account %s: %w", accountID, err)
	}

	methods := make([]*auth.AuthMethod, len(dbMethods))
	for i, dbm := range dbMethods {
		methods[i] = dbm.toDomain()
	}
	return methods, nil
}

const updateAuthMethodQuery = `
	UPDATE auth_methods SET
		credential = :credential,
		verified = :verified,
		provider = :provider,
		updated_at = :updated_at
	WHERE id = :id`

func (r *AuthRepository) UpdateAuthMethod(ctx context.Context, method *auth.AuthMethod) error {
	method.UpdatedAt = time.Now().UTC()
	dbModel := dbAuthMethodFromDomain(method)
	res, err := sqlx.NamedExecContext(ctx, r.runner, updateAuthMethodQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to update auth method %s: %w", method.ID, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrAuthMethodNotFound
	}
	return nil
}

func (r *AuthRepository) DeleteAuthMethodsByType(ctx context.Context, accountID uuid.UUID, methodType auth.AuthMethodType, isVerified bool) error {
	query, args, err := r.sq.Delete("auth_methods").
		Where(squirrel.Eq{
			"account_id": accountID,
			"type":       methodType,
			"verified":   isVerified, // Fixed column name
		}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete auth methods query: %w", err)
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete auth methods: %w", err)
	}
	return nil
}

func (r *AuthRepository) DeleteAuthMethod(ctx context.Context, methodID uuid.UUID) error {
	query, args, err := r.sq.Delete("auth_methods").Where(squirrel.Eq{"id": methodID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete auth method query: %w", err)
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete auth method %s: %w", methodID, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrAuthMethodNotFound
	}
	return nil
}

// --- SessionRepository Implementation ---

const createSessionQuery = `
	INSERT INTO sessions (id, account_id, refresh_token_hash, device_fingerprint_hash, ip_address, user_agent, expires_at, last_used_at, created_at, revoked_at)
	VALUES (:id, :account_id, :refresh_token_hash, :device_fingerprint_hash, :ip_address, :user_agent, :expires_at, :last_used_at, :created_at, :revoked_at)`

func (r *AuthRepository) CreateSession(ctx context.Context, session *auth.Session) error {
	dbModel := dbSessionFromDomain(session)
	_, err := sqlx.NamedExecContext(ctx, r.runner, createSessionQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetSessionByID(ctx context.Context, id uuid.UUID) (*auth.Session, error) {
	return r.getSession(ctx, "id", id)
}

func (r *AuthRepository) GetSessionByRefreshTokenHash(ctx context.Context, hash string) (*auth.Session, error) {
	return r.getSession(ctx, "refresh_token_hash", hash)
}

func (r *AuthRepository) ListSessionsForAccount(ctx context.Context, accountID uuid.UUID) ([]*auth.Session, error) {
	query, args, err := r.sq.Select("*").From("sessions").
		Where(squirrel.Eq{"account_id": accountID, "revoked_at": nil}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build list sessions query: %w", err)
	}

	var dbSessions []dbSession
	if err := sqlx.SelectContext(ctx, r.runner, &dbSessions, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list sessions for account %s: %w", accountID, err)
	}

	sessions := make([]*auth.Session, len(dbSessions))
	for i, dbs := range dbSessions {
		sessions[i] = dbs.toDomain()
	}
	return sessions, nil
}

func (r *AuthRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.sq.Delete("sessions").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete session query: %w", err)
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete session %s: %w", id, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrSessionNotFound
	}
	return nil
}

func (r *AuthRepository) RevokeAllForAccount(ctx context.Context, accountID uuid.UUID) error {
	query, args, err := r.sq.Update("sessions").
		Set("revoked_at", time.Now().UTC()).
		Where(squirrel.Eq{"account_id": accountID, "revoked_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build revoke all sessions query: %w", err)
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to revoke all sessions for account %s: %w", accountID, err)
	}
	return nil
}

// --- TokenRepository Implementation ---

const createTokenQuery = `
	INSERT INTO tokens (id, account_id, type, hash, expires_at, created_at, used_at)
	VALUES (:id, :account_id, :type, :hash, :expires_at, :created_at, :used_at)`

func (r *AuthRepository) CreateToken(ctx context.Context, token *auth.Token) error {
	dbModel := dbTokenFromDomain(token)
	_, err := sqlx.NamedExecContext(ctx, r.runner, createTokenQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetTokenByHash(ctx context.Context, hash string, tokenType auth.TokenType) (*auth.Token, error) {
	query, args, err := r.sq.Select("*").From("tokens").
		Where(squirrel.Eq{
			"hash":    hash,
			"type":    tokenType,
			"used_at": nil,
		}).
		Where(squirrel.Gt{"expires_at": time.Now().UTC()}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get token query: %w", err)
	}

	var dbt dbToken
	if err := sqlx.GetContext(ctx, r.runner, &dbt, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to get token by hash: %w", err)
	}
	return dbt.toDomain(), nil
}

func (r *AuthRepository) UseToken(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.sq.Update("tokens").
		Set("used_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": id, "used_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build use token query: %w", err)
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark token %s as used: %w", id, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrTokenNotFound
	}
	return nil
}

// --- Private Helpers ---

func (r *AuthRepository) getAccount(ctx context.Context, field string, value any) (*auth.Account, error) {
	query, args, err := r.sq.Select("*").From("accounts").Where(squirrel.Eq{field: value}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get account query: %w", err)
	}

	var dba dbAccount
	if err := sqlx.GetContext(ctx, r.runner, &dba, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get account by %s: %w", field, err)
	}
	return dba.toDomain()
}

func (r *AuthRepository) getSession(ctx context.Context, field string, value any) (*auth.Session, error) {
	query, args, err := r.sq.Select("*").From("sessions").
		Where(squirrel.Eq{field: value, "revoked_at": nil}).
		Where(squirrel.Gt{"expires_at": time.Now().UTC()}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get session query: %w", err)
	}

	var dbs dbSession
	if err := sqlx.GetContext(ctx, r.runner, &dbs, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session by %s: %w", field, err)
	}
	return dbs.toDomain(), nil
}

// --- DB Models and Mappers ---

// Only showing the fixed parts - replace in your auth_repository.go

// Update the dbToken struct:
type dbToken struct {
	ID        uuid.UUID    `db:"id"`
	AccountID uuid.UUID    `db:"account_id"`
	Type      string       `db:"type"`
	Hash      string       `db:"hash"` // Changed from TokenHash
	ExpiresAt time.Time    `db:"expires_at"`
	CreatedAt time.Time    `db:"created_at"`
	UsedAt    sql.NullTime `db:"used_at"`
}

func (d *dbToken) toDomain() *auth.Token {
	var usedAt *time.Time
	if d.UsedAt.Valid {
		usedAt = &d.UsedAt.Time
	}

	return &auth.Token{
		ID:        d.ID,
		AccountID: d.AccountID,
		Type:      auth.TokenType(d.Type),
		TokenHash: d.Hash, // Map hash to TokenHash
		ExpiresAt: d.ExpiresAt,
		CreatedAt: d.CreatedAt,
		UsedAt:    usedAt,
	}
}

func dbTokenFromDomain(t *auth.Token) *dbToken {
	var usedAt sql.NullTime
	if t.UsedAt != nil && !t.UsedAt.IsZero() {
		usedAt.Time = *t.UsedAt
		usedAt.Valid = true
	}
	return &dbToken{
		ID:        t.ID,
		AccountID: t.AccountID,
		Type:      string(t.Type),
		Hash:      t.TokenHash, // Map TokenHash to hash
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
		UsedAt:    usedAt,
	}
}

type dbAccount struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	Email            string     `db:"email"`
	Status           string     `db:"status"`
	MfaEnabled       bool       `db:"mfa_enabled"`
	FailedLoginCount int        `db:"failed_login_count"`
	LockedUntil      *time.Time `db:"locked_until"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

func (d *dbAccount) toDomain() (*auth.Account, error) {
	email, err := core.NewEmail(d.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email in DB: %w", err)
	}
	return &auth.Account{
		ID:               d.ID,
		UserID:           d.UserID,
		Email:            email,
		Status:           auth.AccountStatus(d.Status),
		MfaEnabled:       d.MfaEnabled,
		FailedLoginCount: d.FailedLoginCount,
		LockedUntil:      d.LockedUntil,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}, nil
}

func dbAccountFromDomain(a *auth.Account) *dbAccount {
	return &dbAccount{
		ID:               a.ID,
		UserID:           a.UserID,
		Email:            a.Email.String(),
		Status:           string(a.Status),
		MfaEnabled:       a.MfaEnabled,
		FailedLoginCount: a.FailedLoginCount,
		LockedUntil:      a.LockedUntil,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

type dbAuthMethod struct {
	ID         uuid.UUID      `db:"id"`
	AccountID  uuid.UUID      `db:"account_id"`
	Type       string         `db:"type"`
	Provider   sql.NullString `db:"provider"`
	Credential string         `db:"credential"`
	Verified   bool           `db:"verified"`
	CreatedAt  time.Time      `db:"created_at"`
	UpdatedAt  time.Time      `db:"updated_at"`
}

func (d *dbAuthMethod) toDomain() *auth.AuthMethod {
	m := &auth.AuthMethod{
		ID:         d.ID,
		AccountID:  d.AccountID,
		Type:       auth.AuthMethodType(d.Type),
		Credential: d.Credential,
		IsVerified: d.Verified,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
	if d.Provider.Valid {
		m.Provider = &d.Provider.String
	}
	return m
}

func dbAuthMethodFromDomain(m *auth.AuthMethod) *dbAuthMethod {
	dbm := &dbAuthMethod{
		ID:         m.ID,
		AccountID:  m.AccountID,
		Type:       string(m.Type),
		Credential: m.Credential,
		Verified:   m.IsVerified,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
	if m.Provider != nil {
		dbm.Provider = sql.NullString{String: *m.Provider, Valid: true}
	}
	return dbm
}

type dbSession struct {
	ID                    uuid.UUID  `db:"id"`
	AccountID             uuid.UUID  `db:"account_id"`
	RefreshTokenHash      string     `db:"refresh_token_hash"`
	DeviceFingerprintHash string     `db:"device_fingerprint_hash"`
	IPAddress             string     `db:"ip_address"`
	UserAgent             string     `db:"user_agent"`
	ExpiresAt             time.Time  `db:"expires_at"`
	LastUsedAt            time.Time  `db:"last_used_at"`
	CreatedAt             time.Time  `db:"created_at"`
	RevokedAt             *time.Time `db:"revoked_at"`
}

func (d *dbSession) toDomain() *auth.Session {
	return &auth.Session{
		ID:                    d.ID,
		AccountID:             d.AccountID,
		RefreshTokenHash:      d.RefreshTokenHash,
		DeviceFingerprintHash: d.DeviceFingerprintHash,
		UserAgent:             d.UserAgent,
		IPAddress:             d.IPAddress,
		ExpiresAt:             d.ExpiresAt,
		LastUsedAt:            d.LastUsedAt,
		CreatedAt:             d.CreatedAt,
		RevokedAt:             d.RevokedAt,
	}
}

func dbSessionFromDomain(s *auth.Session) *dbSession {
	return &dbSession{
		ID:                    s.ID,
		AccountID:             s.AccountID,
		RefreshTokenHash:      s.RefreshTokenHash,
		DeviceFingerprintHash: s.DeviceFingerprintHash,
		UserAgent:             s.UserAgent,
		IPAddress:             s.IPAddress,
		ExpiresAt:             s.ExpiresAt,
		LastUsedAt:            s.LastUsedAt,
		CreatedAt:             s.CreatedAt,
		RevokedAt:             s.RevokedAt,
	}
}

func (r *AuthRepository) FindSessionByRefreshTokenHash(ctx context.Context, hash string) (*auth.Session, error) {
	// This method finds a session regardless of its revoked or expired status.
	query, args, err := r.sq.Select("*").From("sessions").
		Where(squirrel.Eq{"refresh_token_hash": hash}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build find session query: %w", err)
	}

	var dbs dbSession
	if err := sqlx.GetContext(ctx, r.runner, &dbs, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to find session by hash: %w", err)
	}
	return dbs.toDomain(), nil
}

func (r *AuthRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	query, args, err := r.sq.Update("sessions").
		Set("revoked_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": id, "revoked_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build revoke session query: %w", err)
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to revoke session %s: %w", id, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return auth.ErrSessionNotFound // It was already revoked or didn't exist.
	}
	return nil
}

func (r *AuthRepository) DeleteExpiredTokens(ctx context.Context, cutoff time.Time) (int64, error) {
	// Parameterized query: cleaner and testable
	query := `DELETE FROM tokens WHERE expires_at < :cutoff`

	// We use NamedExec for consistency, though plain Exec with $1 works too.
	res, err := sqlx.NamedExecContext(ctx, r.runner, query, map[string]interface{}{
		"cutoff": cutoff,
	})

	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}
	return res.RowsAffected()
}

func (r *AuthRepository) DeleteExpiredSessions(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at < :cutoff`

	res, err := sqlx.NamedExecContext(ctx, r.runner, query, map[string]interface{}{
		"cutoff": cutoff,
	})

	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

func (r *AuthRepository) GetAccountByIDForUpdate(ctx context.Context, id uuid.UUID) (*auth.Account, error) {
	query, args, _ := r.sq.Select("*").From("accounts").
		Where(squirrel.Eq{"id": id}).
		Suffix("FOR UPDATE"). // Locks the row
		ToSql()

	var dba dbAccount
	if err := sqlx.GetContext(ctx, r.runner, &dba, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}
	return dba.toDomain()
}
func (r *AuthRepository) DeleteAccountByUserID(ctx context.Context, userID uuid.UUID) error {

	query, args, err := r.sq.Delete("accounts").Where(squirrel.Eq{"user_id": userID}).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete account query: %w", err)
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return auth.ErrAccountNotFound
	}
	return nil
}

func (r *AuthRepository) GetAccountByIdentifier(ctx context.Context, identifier string) (*auth.Account, error) {
	// 1. Build the base query
	builder := r.sq.Select("*").From("accounts")

	// 2. Build where clause
	whereClause := squirrel.Or{
		squirrel.Eq{"email": identifier},
	}

	if id, err := uuid.Parse(identifier); err == nil {
		whereClause = append(whereClause, squirrel.Eq{"id": id})
	}

	// FIX: Rename 'sql' variable to 'querySQL' to avoid shadowing the package
	querySQL, args, err := builder.Where(whereClause).Limit(1).ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build identifier query: %w", err)
	}

	// 3. Execute
	var dba dbAccount
	// Use 'querySQL' here
	if err := sqlx.GetContext(ctx, r.runner, &dba, querySQL, args...); err != nil {
		// Now 'sql.ErrNoRows' refers to the package, not the string variable
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get account by identifier: %w", err)
	}

	return dba.toDomain()
}
