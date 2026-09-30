// internal/services/auth/admin_service_test.go
package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"
)

// adminServiceTestSuite holds all mocked dependencies for the AdminService.
type adminServiceTestSuite struct {
	repo           *auth_mocks.MockRepository
	auditRecorder  *audit_mocks.MockEventRecorder
	accountService *auth_mocks.MockAccountService
	mainService    *auth_mocks.MockService
	authzService   *rbac_mocks.MockAuthorizationService
	service        auth.AdminService
}

// setupAdminService initializes a new test suite with fresh mocks.
func setupAdminService(t *testing.T) adminServiceTestSuite {
	t.Helper()
	var ts adminServiceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.auditRecorder = audit_mocks.NewMockEventRecorder(t)
	ts.accountService = auth_mocks.NewMockAccountService(t)
	ts.mainService = auth_mocks.NewMockService(t)
	ts.authzService = rbac_mocks.NewMockAuthorizationService(t)
	ts.service = auth_service.NewAdminService(
		ts.repo,
		ts.auditRecorder,
		ts.accountService,
		ts.mainService,
		ts.authzService,
		&log.NoOpLogger{},
	)
	return ts
}

// Helper to create a context with a test admin actor.
func createAdminContext() context.Context {
	actor := &auth.Actor{AccountID: uuid.New(), UserID: uuid.New()}
	return auth.NewContextWithActor(context.Background(), actor)
}

func TestAdminService_AdminUpdateAccountEmail(t *testing.T) {
	ctx := createAdminContext()
	accountID := uuid.New()
	oldEmail := core.MustNewEmail(t, "old@example.com")
	newEmailStr := "new@example.com"
	newEmail := core.MustNewEmail(t, newEmailStr)
	account := &auth.Account{ID: accountID, Email: oldEmail, Status: auth.StatusActive}

	t.Run("should successfully update email and trigger verification", func(t *testing.T) {
		ts := setupAdminService(t)
		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByEmail(ctx, newEmail).Return(nil, auth.ErrAccountNotFound).Once()
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.MatchedBy(func(acc *auth.Account) bool {
				return acc.Email.String() == newEmailStr && acc.Status == auth.StatusUnverified
			})).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// CORRECTED: Expect the call and use the correct Run signature.
		ts.accountService.EXPECT().RequestEmailVerification(mock.Anything, newEmail).
			Run(func(ctx context.Context, email core.Email) {
				wg.Done()
			}).
			Return(nil).
			Once()

		err := ts.service.AdminUpdateAccountEmail(ctx, accountID, newEmailStr)
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("should fail if new email is already taken by another account", func(t *testing.T) {
		ts := setupAdminService(t)
		existingAccount := &auth.Account{ID: uuid.New()}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByEmail(ctx, newEmail).Return(existingAccount, nil).Once()
			err := fn(txRepo)
			assert.ErrorIs(t, err, auth.ErrAccountExists)
		}).Return(auth.ErrAccountExists).Once()

		err := ts.service.AdminUpdateAccountEmail(ctx, accountID, newEmailStr)
		assert.ErrorIs(t, err, auth.ErrAccountExists)
		ts.accountService.AssertNotCalled(t, "RequestEmailVerification", mock.Anything, mock.Anything)
	})

	t.Run("should succeed if new email is a case-change of the existing email", func(t *testing.T) {
		ts := setupAdminService(t)
		var wg sync.WaitGroup
		wg.Add(1)

		newCaseEmailStr := "Old@example.com"
		newCaseEmail := core.MustNewEmail(t, newCaseEmailStr)

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByEmail(ctx, newCaseEmail).Return(account, nil).Once()
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(account, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.Anything).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()

		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// CORRECTED: Expect the call and use the correct Run signature.
		ts.accountService.EXPECT().RequestEmailVerification(mock.Anything, newCaseEmail).
			Run(func(ctx context.Context, email core.Email) {
				wg.Done()
			}).
			Return(nil).
			Once()

		err := ts.service.AdminUpdateAccountEmail(ctx, accountID, newCaseEmailStr)
		require.NoError(t, err)

		wg.Wait()
	})
}

func TestAdminService_AdminInvalidateAllSessionsForAccount(t *testing.T) {
	ctx := createAdminContext()
	accountID := uuid.New()

	t.Run("should call repository to revoke all sessions", func(t *testing.T) {
		ts := setupAdminService(t)
		ts.repo.EXPECT().RevokeAllForAccount(ctx, accountID).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.AdminInvalidateAllSessionsForAccount(ctx, accountID)
		require.NoError(t, err)
	})

	t.Run("should return an error if repository fails", func(t *testing.T) {
		ts := setupAdminService(t)
		repoErr := errors.New("database error")
		ts.repo.EXPECT().RevokeAllForAccount(ctx, accountID).Return(repoErr).Once()

		err := ts.service.AdminInvalidateAllSessionsForAccount(ctx, accountID)
		assert.ErrorIs(t, err, repoErr)
		ts.auditRecorder.AssertNotCalled(t, "Record", mock.Anything, mock.Anything)
	})

	// CORRECTED: This test is now valid and asserts the correct, secure behavior.
	t.Run("should fail if no actor in context", func(t *testing.T) {
		ts := setupAdminService(t)
		noActorCtx := context.Background()

		err := ts.service.AdminInvalidateAllSessionsForAccount(noActorCtx, accountID)

		assert.ErrorIs(t, err, auth.ErrUnauthorized)
		ts.repo.AssertNotCalled(t, "RevokeAllForAccount")
	})
}

func TestAdminService_UpdateAccountStatus(t *testing.T) {
	ts := setupAdminService(t)
	ctx := createAdminContext()
	accountID := uuid.New()
	reason := "Violation of terms"
	activeEmail := core.MustNewEmail(t, "active@example.com")

	t.Run("should suspend an active account and revoke sessions", func(t *testing.T) {
		activeAccount := &auth.Account{ID: accountID, Status: auth.StatusActive, Email: activeEmail}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(activeAccount, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.MatchedBy(func(acc *auth.Account) bool {
				return acc.Status == auth.StatusSuspended
			})).Return(nil).Once()
			// CRITICAL: Expect session revocation to be called
			txRepo.EXPECT().RevokeAllForAccount(ctx, accountID).Return(nil).Once()
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("audit.Event")).Return(nil).Once()

		updated, err := ts.service.UpdateAccountStatus(ctx, accountID, auth.StatusSuspended, &reason)
		require.NoError(t, err)
		assert.Equal(t, auth.StatusSuspended, updated.Status)
	})

	t.Run("should activate a suspended account", func(t *testing.T) {
		suspendedAccount := &auth.Account{ID: accountID, Status: auth.StatusSuspended, Email: activeEmail}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(suspendedAccount, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.MatchedBy(func(acc *auth.Account) bool {
				return acc.Status == auth.StatusActive
			})).Return(nil).Once()
			// Note: RevokeAllForAccount is NOT called when activating
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		updated, err := ts.service.UpdateAccountStatus(ctx, accountID, auth.StatusActive, nil)
		require.NoError(t, err)
		assert.Equal(t, auth.StatusActive, updated.Status)
	})

	t.Run("should be idempotent and not audit if status is unchanged", func(t *testing.T) {
		activeAccount := &auth.Account{ID: accountID, Status: auth.StatusActive, Email: activeEmail}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(activeAccount, nil).Once()
			// No Update or Revoke calls are made
			err := fn(txRepo)
			require.NoError(t, err)
		}).Return(nil).Once()
		// CRITICAL: Audit should NOT be called
		// No ts.auditRecorder.EXPECT() call

		updated, err := ts.service.UpdateAccountStatus(ctx, accountID, auth.StatusActive, nil)
		require.NoError(t, err)
		assert.Equal(t, auth.StatusActive, updated.Status)
	})

	t.Run("should fail and rollback if revoking sessions fails during suspension", func(t *testing.T) {
		activeAccount := &auth.Account{ID: accountID, Status: auth.StatusActive}
		repoErr := errors.New("db connection lost")

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Run(func(ctx context.Context, fn func(txRepo auth.Repository) error) {
			txRepo := auth_mocks.NewMockRepository(t)
			txRepo.EXPECT().GetAccountByID(ctx, accountID).Return(activeAccount, nil).Once()
			txRepo.EXPECT().UpdateAccount(ctx, mock.Anything).Return(nil).Once()
			txRepo.EXPECT().RevokeAllForAccount(ctx, accountID).Return(repoErr).Once()
			err := fn(txRepo)
			assert.ErrorIs(t, err, repoErr)
		}).Return(repoErr).Once()

		_, err := ts.service.UpdateAccountStatus(ctx, accountID, auth.StatusSuspended, &reason)
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestAdminService_Impersonate(t *testing.T) {
	ts := setupAdminService(t)
	ctx := createAdminContext()
	adminActor := auth.ActorFromContext(ctx)

	targetAccountID := uuid.New()
	targetAccount := &auth.Account{ID: targetAccountID, UserID: uuid.New(), Email: core.MustNewEmail(t, "target@example.com")}
	reason := "Support ticket #12345"

	t.Run("should succeed for authorized admin", func(t *testing.T) {
		// 1. Mock the authorization check to return true
		ts.authzService.EXPECT().Authorize(ctx, mock.MatchedBy(func(check rbac.PermissionCheck) bool {
			return check.PermissionName == auth.PermAuthAccountImpersonate && check.ActorID == adminActor.AccountID
		})).Return(true, nil).Once()

		// 2. Mock fetching the target account
		ts.repo.EXPECT().GetAccountByID(ctx, targetAccountID).Return(targetAccount, nil).Once()

		// 3. Mock the session creation call
		mockTokenPair := &auth.TokenPair{AccessToken: "impersonated-token"}
		ts.mainService.EXPECT().CreateSessionForAccount(ctx, targetAccount, mock.Anything, mock.Anything, &adminActor.AccountID).Return(mockTokenPair, nil).Once()

		// 4. Mock the audit record
		ts.auditRecorder.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		// Act
		tokenPair, err := ts.service.Impersonate(ctx, targetAccountID, reason)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, tokenPair)
		assert.Equal(t, "impersonated-token", tokenPair.AccessToken)
	})

	t.Run("should fail if admin is not authorized", func(t *testing.T) {
		// Mock the authorization check to return false
		ts.authzService.EXPECT().Authorize(ctx, mock.Anything).Return(false, nil).Once()

		_, err := ts.service.Impersonate(ctx, targetAccountID, reason)

		assert.ErrorIs(t, err, rbac.ErrAccessDenied)
	})

	t.Run("should fail if no actor in context", func(t *testing.T) {
		_, err := ts.service.Impersonate(context.Background(), targetAccountID, reason)
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})

	t.Run("should fail if target account not found", func(t *testing.T) {
		ts.authzService.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetAccountByID(ctx, targetAccountID).Return(nil, auth.ErrAccountNotFound).Once()

		_, err := ts.service.Impersonate(ctx, targetAccountID, reason)
		assert.ErrorIs(t, err, auth.ErrAccountNotFound)
	})
}
