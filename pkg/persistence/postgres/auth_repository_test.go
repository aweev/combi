package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* ---------- Test Helpers ---------- */

func newTestAccount(t *testing.T) *auth.Account {
	t.Helper()
	email, err := core.NewEmail("test-" + uuid.NewString() + "@example.com")
	require.NoError(t, err)

	return &auth.Account{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Email:      email,
		Status:     auth.StatusUnverified,
		MfaEnabled: false,
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
}

func newTestAuthMethod(t *testing.T, accountID uuid.UUID, typ auth.AuthMethodType) *auth.AuthMethod {
	t.Helper()
	method := &auth.AuthMethod{
		ID:         uuid.New(),
		AccountID:  accountID,
		Type:       typ,
		Credential: "$2a$10$fake-hash-for-testing",
		IsVerified: true,
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
	// ADDED: Set a provider for OAuth types for more realistic tests.
	if typ == auth.AuthMethodOAuth {
		provider := "google"
		method.Provider = &provider
	}
	return method
}

func newTestSession(t *testing.T, accountID uuid.UUID) *auth.Session {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &auth.Session{
		ID:                    uuid.New(),
		AccountID:             accountID,
		RefreshTokenHash:      "refresh-hash-" + uuid.NewString(),
		DeviceFingerprintHash: "", // Add this field
		IPAddress:             "127.0.0.1",
		UserAgent:             "GoTest/1.0",
		ExpiresAt:             now.Add(7 * 24 * time.Hour),
		LastUsedAt:            now,
		CreatedAt:             now,
		RevokedAt:             nil, // Add this field explicitly
	}
}

func newTestToken(t *testing.T, accountID uuid.UUID, typ auth.TokenType) *auth.Token {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &auth.Token{
		ID:        uuid.New(),
		AccountID: accountID,
		Type:      typ,
		TokenHash: "token-hash-" + uuid.NewString(),
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
		UsedAt:    nil,
	}
}

/* ---------- Account Tests ---------- */

func TestAuthRepository_AccountCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CreateAndGet", func(t *testing.T) {
		acc := newTestAccount(t)
		err := repo.CreateAccount(ctx, acc)
		require.NoError(t, err)

		byID, err := repo.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, byID.ID)
		assert.Equal(t, acc.Email, byID.Email)
		assert.Equal(t, acc.UserID, byID.UserID)
		assert.Equal(t, acc.Status, byID.Status)
		assert.Equal(t, acc.MfaEnabled, byID.MfaEnabled)

		byEmail, err := repo.GetAccountByEmail(ctx, acc.Email)
		require.NoError(t, err)
		assert.Equal(t, acc.ID, byEmail.ID)
		assert.Equal(t, acc.Email, byEmail.Email)
	})

	t.Run("DuplicateEmail", func(t *testing.T) {
		acc1 := newTestAccount(t)
		acc2 := newTestAccount(t)
		acc2.Email = acc1.Email

		require.NoError(t, repo.CreateAccount(ctx, acc1))
		err := repo.CreateAccount(ctx, acc2)
		assert.ErrorIs(t, err, auth.ErrAccountExists)
	})

	t.Run("GetAccountByID_NotFound", func(t *testing.T) {
		_, err := repo.GetAccountByID(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})

	t.Run("GetAccountByEmail_NotFound", func(t *testing.T) {
		email, _ := core.NewEmail("nonexistent@example.com")
		_, err := repo.GetAccountByEmail(ctx, email)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		acc := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc))

		acc.Status = auth.StatusActive
		acc.MfaEnabled = true
		time.Sleep(time.Millisecond)
		require.NoError(t, repo.UpdateAccount(ctx, acc))

		updated, err := repo.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, auth.StatusActive, updated.Status)
		assert.True(t, updated.MfaEnabled)
		assert.True(t, updated.UpdatedAt.After(acc.CreatedAt))
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		ghost := newTestAccount(t)
		err := repo.UpdateAccount(ctx, ghost)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})

	t.Run("UpdatePreservesCreatedAt", func(t *testing.T) {
		acc := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc))
		originalCreatedAt := acc.CreatedAt

		acc.Status = auth.StatusActive
		require.NoError(t, repo.UpdateAccount(ctx, acc))

		updated, err := repo.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		assert.Equal(t, originalCreatedAt.Unix(), updated.CreatedAt.Unix())
	})
}

/* ---------- AuthMethod Tests ---------- */

func TestAuthRepository_AuthMethod(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	acc := newTestAccount(t)
	require.NoError(t, repo.CreateAccount(ctx, acc))

	t.Run("CreateGetList", func(t *testing.T) {
		m1 := newTestAuthMethod(t, acc.ID, auth.AuthMethodPassword)
		require.NoError(t, repo.CreateAuthMethod(ctx, m1))

		got, err := repo.GetAuthMethod(ctx, acc.ID, auth.AuthMethodPassword)
		require.NoError(t, err)
		assert.Equal(t, m1.ID, got.ID)
		assert.Equal(t, m1.Type, got.Type)
		assert.Equal(t, m1.Credential, got.Credential)
		assert.Equal(t, m1.IsVerified, got.IsVerified)

		list, err := repo.ListAuthMethodsForAccount(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, m1.ID, list[0].ID)
	})

	t.Run("CreateMultipleMethods", func(t *testing.T) {
		acc2 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc2))

		m1 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodPassword)
		m2 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodTOTP)
		m3 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodBackupCode)

		m4 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodOAuth) // Defaults to 'google'
		m5 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodOAuth)
		providerGithub := "github"
		m5.Provider = &providerGithub

		require.NoError(t, repo.CreateAuthMethod(ctx, m1))
		require.NoError(t, repo.CreateAuthMethod(ctx, m2))
		require.NoError(t, repo.CreateAuthMethod(ctx, m3))
		require.NoError(t, repo.CreateAuthMethod(ctx, m4))
		require.NoError(t, repo.CreateAuthMethod(ctx, m5))

		m6 := newTestAuthMethod(t, acc2.ID, auth.AuthMethodOAuth) // Another 'google' one
		err := repo.CreateAuthMethod(ctx, m6)
		assert.ErrorIs(t, err, auth.ErrAuthMethodExists)

		list, err := repo.ListAuthMethodsForAccount(ctx, acc2.ID)
		require.NoError(t, err)
		assert.Len(t, list, 5)
	})

	t.Run("GetAuthMethod_NotFound", func(t *testing.T) {
		_, err := repo.GetAuthMethod(ctx, uuid.New(), auth.AuthMethodPassword)
		assert.ErrorIs(t, err, auth.ErrAuthMethodNotFound)
	})

	t.Run("ListAuthMethodsForAccount_Empty", func(t *testing.T) {
		acc3 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc3))

		list, err := repo.ListAuthMethodsForAccount(ctx, acc3.ID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("Update", func(t *testing.T) {
		m := newTestAuthMethod(t, acc.ID, auth.AuthMethodOAuth)
		require.NoError(t, repo.CreateAuthMethod(ctx, m))

		originalID := m.ID
		newProvider := "github"
		m.Credential = "new-oauth-id"
		m.IsVerified = false
		m.Provider = &newProvider
		time.Sleep(time.Millisecond)
		require.NoError(t, repo.UpdateAuthMethod(ctx, m))

		list, err := repo.ListAuthMethodsForAccount(ctx, acc.ID)
		require.NoError(t, err)

		var updated *auth.AuthMethod
		for _, method := range list {
			if method.ID == originalID {
				updated = method
				break
			}
		}
		require.NotNil(t, updated)

		assert.Equal(t, "new-oauth-id", updated.Credential)
		assert.False(t, updated.IsVerified)
		assert.NotNil(t, updated.Provider)
		assert.Equal(t, "github", *updated.Provider)
		assert.True(t, updated.UpdatedAt.After(m.CreatedAt))
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		ghost := newTestAuthMethod(t, uuid.New(), auth.AuthMethodPassword)
		err := repo.UpdateAuthMethod(ctx, ghost)
		assert.ErrorIs(t, err, auth.ErrAuthMethodNotFound)
	})

	t.Run("DeleteByType", func(t *testing.T) {
		acc4 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc4))

		m := newTestAuthMethod(t, acc4.ID, auth.AuthMethodBackupCode)
		require.NoError(t, repo.CreateAuthMethod(ctx, m))

		require.NoError(t, repo.DeleteAuthMethodsByType(ctx, acc4.ID, auth.AuthMethodBackupCode, true))
		_, err := repo.GetAuthMethod(ctx, acc4.ID, auth.AuthMethodBackupCode)
		assert.ErrorIs(t, err, auth.ErrAuthMethodNotFound)
	})

	t.Run("DeleteByType_MatchesIsVerified", func(t *testing.T) {
		acc5 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc5))

		// FIX: Use two *different* auth method types to avoid violating the unique constraint.
		m1 := newTestAuthMethod(t, acc5.ID, auth.AuthMethodTOTP)
		m1.IsVerified = true
		m2 := newTestAuthMethod(t, acc5.ID, auth.AuthMethodBackupCode)
		m2.IsVerified = false

		require.NoError(t, repo.CreateAuthMethod(ctx, m1))
		require.NoError(t, repo.CreateAuthMethod(ctx, m2))

		// Delete only verified TOTP methods
		require.NoError(t, repo.DeleteAuthMethodsByType(ctx, acc5.ID, auth.AuthMethodTOTP, true))

		// Check that the unverified backup code method remains.
		list, err := repo.ListAuthMethodsForAccount(ctx, acc5.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, auth.AuthMethodBackupCode, list[0].Type)
		assert.False(t, list[0].IsVerified)
	})

	t.Run("DeleteSingle", func(t *testing.T) {
		m := newTestAuthMethod(t, acc.ID, auth.AuthMethodOAuth)
		require.NoError(t, repo.CreateAuthMethod(ctx, m))

		require.NoError(t, repo.DeleteAuthMethod(ctx, m.ID))

		// Verify it's gone
		list, err := repo.ListAuthMethodsForAccount(ctx, acc.ID)
		require.NoError(t, err)
		found := false
		for _, method := range list {
			if method.ID == m.ID {
				found = true
				break
			}
		}
		assert.False(t, found, "method should have been deleted")
	})

	t.Run("DeleteSingle_NotFound", func(t *testing.T) {
		err := repo.DeleteAuthMethod(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrAuthMethodNotFound)
	})
}

/* ---------- Session Tests ---------- */

func TestAuthRepository_Session(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	acc := newTestAccount(t)
	require.NoError(t, repo.CreateAccount(ctx, acc))

	t.Run("CreateGetListDelete", func(t *testing.T) {
		s1 := newTestSession(t, acc.ID)
		require.NoError(t, repo.CreateSession(ctx, s1))

		byID, err := repo.GetSessionByID(ctx, s1.ID)
		require.NoError(t, err)
		assert.Equal(t, s1.ID, byID.ID)
		assert.Equal(t, s1.AccountID, byID.AccountID)
		assert.Equal(t, s1.RefreshTokenHash, byID.RefreshTokenHash)
		assert.Equal(t, s1.IPAddress, byID.IPAddress)
		assert.Equal(t, s1.UserAgent, byID.UserAgent)

		byHash, err := repo.GetSessionByRefreshTokenHash(ctx, s1.RefreshTokenHash)
		require.NoError(t, err)
		assert.Equal(t, s1.ID, byHash.ID)

		list, err := repo.ListSessionsForAccount(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)

		require.NoError(t, repo.DeleteSession(ctx, s1.ID))
		_, err = repo.GetSessionByID(ctx, s1.ID)
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		_, err := repo.GetSessionByID(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})

	t.Run("GetSessionByRefreshTokenHash_NotFound", func(t *testing.T) {
		_, err := repo.GetSessionByRefreshTokenHash(ctx, "nonexistent-hash")
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})

	t.Run("DeleteSession_NotFound", func(t *testing.T) {
		err := repo.DeleteSession(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})

	t.Run("ExpiredSession_NotRetrieved", func(t *testing.T) {
		s := newTestSession(t, acc.ID)
		s.ExpiresAt = time.Now().UTC().Add(-time.Hour)
		require.NoError(t, repo.CreateSession(ctx, s))

		_, err := repo.GetSessionByID(ctx, s.ID)
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})

	t.Run("ListSessionsForAccount_Empty", func(t *testing.T) {
		acc2 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc2))

		list, err := repo.ListSessionsForAccount(ctx, acc2.ID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("ListSessionsForAccount_MultipleActive", func(t *testing.T) {
		acc3 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc3))

		s1 := newTestSession(t, acc3.ID)
		s2 := newTestSession(t, acc3.ID)
		s3 := newTestSession(t, acc3.ID)

		require.NoError(t, repo.CreateSession(ctx, s1))
		require.NoError(t, repo.CreateSession(ctx, s2))
		require.NoError(t, repo.CreateSession(ctx, s3))

		list, err := repo.ListSessionsForAccount(ctx, acc3.ID)
		require.NoError(t, err)
		assert.Len(t, list, 3)
	})

	t.Run("RevokeAllForAccount", func(t *testing.T) {
		acc4 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc4))

		s2 := newTestSession(t, acc4.ID)
		s3 := newTestSession(t, acc4.ID)
		require.NoError(t, repo.CreateSession(ctx, s2))
		require.NoError(t, repo.CreateSession(ctx, s3))

		require.NoError(t, repo.RevokeAllForAccount(ctx, acc4.ID))

		list, err := repo.ListSessionsForAccount(ctx, acc4.ID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("RevokeAllForAccount_NoSessions", func(t *testing.T) {
		acc5 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc5))

		err := repo.RevokeAllForAccount(ctx, acc5.ID)
		assert.NoError(t, err)
	})
}

/* ---------- Token Tests ---------- */

func TestAuthRepository_Token(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	acc := newTestAccount(t)
	require.NoError(t, repo.CreateAccount(ctx, acc))

	t.Run("CreateGetUse", func(t *testing.T) {
		tok := newTestToken(t, acc.ID, auth.TokenEmailVerification)
		require.NoError(t, repo.CreateToken(ctx, tok))

		got, err := repo.GetTokenByHash(ctx, tok.TokenHash, auth.TokenEmailVerification)
		require.NoError(t, err)
		assert.Equal(t, tok.ID, got.ID)
		assert.Equal(t, tok.AccountID, got.AccountID)
		assert.Equal(t, tok.Type, got.Type)
		assert.Equal(t, tok.TokenHash, got.TokenHash)
		assert.Nil(t, got.UsedAt)

		require.NoError(t, repo.UseToken(ctx, tok.ID))

		// Should not be able to get used token
		_, err = repo.GetTokenByHash(ctx, tok.TokenHash, auth.TokenEmailVerification)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("UseToken_NotFound", func(t *testing.T) {
		err := repo.UseToken(ctx, uuid.New())
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("UseToken_Twice", func(t *testing.T) {
		tok := newTestToken(t, acc.ID, auth.TokenPasswordReset)
		require.NoError(t, repo.CreateToken(ctx, tok))

		require.NoError(t, repo.UseToken(ctx, tok.ID))
		err := repo.UseToken(ctx, tok.ID)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("GetTokenByHash_NotFound", func(t *testing.T) {
		_, err := repo.GetTokenByHash(ctx, "nonexistent-hash", auth.TokenEmailVerification)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		expired := newTestToken(t, acc.ID, auth.TokenPasswordReset)
		expired.ExpiresAt = time.Now().UTC().Add(-time.Minute)
		require.NoError(t, repo.CreateToken(ctx, expired))

		_, err := repo.GetTokenByHash(ctx, expired.TokenHash, auth.TokenPasswordReset)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("WrongTokenType", func(t *testing.T) {
		tok := newTestToken(t, acc.ID, auth.TokenEmailVerification)
		require.NoError(t, repo.CreateToken(ctx, tok))

		_, err := repo.GetTokenByHash(ctx, tok.TokenHash, auth.TokenPasswordReset)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
	})

	t.Run("MultipleTokensSameAccount", func(t *testing.T) {
		acc2 := newTestAccount(t)
		require.NoError(t, repo.CreateAccount(ctx, acc2))

		tok1 := newTestToken(t, acc2.ID, auth.TokenEmailVerification)
		tok2 := newTestToken(t, acc2.ID, auth.TokenPasswordReset)
		tok3 := newTestToken(t, acc2.ID, auth.TokenMfaChallenge)

		require.NoError(t, repo.CreateToken(ctx, tok1))
		require.NoError(t, repo.CreateToken(ctx, tok2))
		require.NoError(t, repo.CreateToken(ctx, tok3))

		got1, err := repo.GetTokenByHash(ctx, tok1.TokenHash, auth.TokenEmailVerification)
		require.NoError(t, err)
		assert.Equal(t, tok1.ID, got1.ID)

		got2, err := repo.GetTokenByHash(ctx, tok2.TokenHash, auth.TokenPasswordReset)
		require.NoError(t, err)
		assert.Equal(t, tok2.ID, got2.ID)

		got3, err := repo.GetTokenByHash(ctx, tok3.TokenHash, auth.TokenMfaChallenge)
		require.NoError(t, err)
		assert.Equal(t, tok3.ID, got3.ID)
	})
}

/* ---------- Transaction Tests ---------- */

func TestAuthRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("commit on success", func(t *testing.T) {
		acc := newTestAccount(t)
		err := repo.WithTx(ctx, func(txRepo auth.Repository) error {
			return txRepo.CreateAccount(ctx, acc)
		})
		require.NoError(t, err)

		_, err = repo.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
	})

	t.Run("rollback on error", func(t *testing.T) {
		acc := newTestAccount(t)
		custom := errors.New("oops")
		err := repo.WithTx(ctx, func(txRepo auth.Repository) error {
			_ = txRepo.CreateAccount(ctx, acc)
			return custom
		})
		assert.ErrorIs(t, err, custom)

		_, err = repo.GetAccountByID(ctx, acc.ID)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})

	t.Run("panic also rolls back", func(t *testing.T) {
		acc := newTestAccount(t)
		assert.Panics(t, func() {
			_ = repo.WithTx(ctx, func(txRepo auth.Repository) error {
				_ = txRepo.CreateAccount(ctx, acc)
				panic("terrible accident")
			})
		})

		_, err := repo.GetAccountByID(ctx, acc.ID)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})

	t.Run("nested transaction error", func(t *testing.T) {
		err := repo.WithTx(ctx, func(txRepo auth.Repository) error {
			return txRepo.WithTx(ctx, func(nestedRepo auth.Repository) error {
				return nil
			})
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot begin transaction from within another transaction")
	})

	t.Run("multiple operations in transaction", func(t *testing.T) {
		acc := newTestAccount(t)
		method := newTestAuthMethod(t, acc.ID, auth.AuthMethodPassword)
		session := newTestSession(t, acc.ID)

		err := repo.WithTx(ctx, func(txRepo auth.Repository) error {
			if err := txRepo.CreateAccount(ctx, acc); err != nil {
				return err
			}
			if err := txRepo.CreateAuthMethod(ctx, method); err != nil {
				return err
			}
			if err := txRepo.CreateSession(ctx, session); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// Verify all operations persisted
		_, err = repo.GetAccountByID(ctx, acc.ID)
		require.NoError(t, err)
		_, err = repo.GetAuthMethod(ctx, acc.ID, auth.AuthMethodPassword)
		require.NoError(t, err)
		_, err = repo.GetSessionByID(ctx, session.ID)
		require.NoError(t, err)
	})

	t.Run("partial rollback on error", func(t *testing.T) {
		acc := newTestAccount(t)
		method := newTestAuthMethod(t, acc.ID, auth.AuthMethodPassword)

		err := repo.WithTx(ctx, func(txRepo auth.Repository) error {
			if err := txRepo.CreateAccount(ctx, acc); err != nil {
				return err
			}
			if err := txRepo.CreateAuthMethod(ctx, method); err != nil {
				return err
			}
			return errors.New("intentional failure")
		})
		require.Error(t, err)

		// Verify nothing persisted
		_, err = repo.GetAccountByID(ctx, acc.ID)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})
}

func TestAuthRepository_Cleanup(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuthRepository(db, &log.NoOpLogger{})
	ctx := context.Background()
	acc := newTestAccount(t)
	require.NoError(t, repo.CreateAccount(ctx, acc))

	t.Run("DeleteExpiredTokens", func(t *testing.T) {
		// 1. Create a token that expired 2 hours ago
		expired := newTestToken(t, acc.ID, auth.TokenPasswordReset)
		expired.ExpiresAt = time.Now().UTC().Add(-2 * time.Hour)
		require.NoError(t, repo.CreateToken(ctx, expired))

		// 2. Create a token that expires in 1 hour (Active)
		active := newTestToken(t, acc.ID, auth.TokenEmailVerification)
		active.ExpiresAt = time.Now().UTC().Add(1 * time.Hour)
		require.NoError(t, repo.CreateToken(ctx, active))

		// 3. Cleanup everything older than 1 hour ago
		cutoff := time.Now().UTC().Add(-1 * time.Hour)
		deleted, err := repo.DeleteExpiredTokens(ctx, cutoff)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted, "Only the expired token should be deleted")

		// Verify state
		_, err = repo.GetTokenByHash(ctx, expired.TokenHash, auth.TokenPasswordReset)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)

		found, err := repo.GetTokenByHash(ctx, active.TokenHash, auth.TokenEmailVerification)
		require.NoError(t, err)
		assert.Equal(t, active.ID, found.ID)
	})
}
