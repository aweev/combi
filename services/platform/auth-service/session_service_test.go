// internal/services/auth/session_service_test.go
package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	auth_mocks "github.com/combi/pkg/domain/auth/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
	auth_service "github.com/combi/auth"
)

// sessionServiceTestSuite holds all mocked dependencies for the SessionService.
type sessionServiceTestSuite struct {
	repo    *auth_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	authz   *rbac_mocks.MockAuthorizationService
	service auth.SessionService
	log     log.Logger
}

// setupSessionService initializes a new test suite with fresh mocks.
func setupSessionService(t *testing.T) sessionServiceTestSuite {
	var ts sessionServiceTestSuite
	ts.repo = auth_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.authz = rbac_mocks.NewMockAuthorizationService(t)
	ts.log = &log.NoOpLogger{}
	ts.service = auth_service.NewSessionService(
		ts.repo,
		ts.audit,
		ts.authz,
		ts.log,
	)
	return ts
}

// Helper to create a context with a specific actor.
func contextWithActor(actorID uuid.UUID) context.Context {
	actor := &auth.Actor{AccountID: actorID}
	return auth.NewContextWithActor(context.Background(), actor)
}

func TestSessionService_List(t *testing.T) {
	ts := setupSessionService(t)
	actorID := uuid.New()
	targetAccountID := uuid.New()
	sessions := []*auth.Session{{ID: uuid.New()}}

	t.Run("should allow owner to list their own sessions", func(t *testing.T) {
		ctx := contextWithActor(actorID)
		// When actorID == targetAccountID, owner check passes.
		// The AuthorizationService is still called, but its result doesn't matter if owner check passes first.
		// Actually, in your implementation: if !isOwner && !isAdmin.
		// So if isOwner is true, we bypass the *need* for admin, but Authorize is still called.
		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(false, nil).Once()

		ts.repo.EXPECT().ListSessionsForAccount(ctx, actorID).Return(sessions, nil).Once()

		result, err := ts.service.List(ctx, actorID)
		require.NoError(t, err)
		assert.Equal(t, sessions, result)
	})

	t.Run("should allow admin to list another user's sessions", func(t *testing.T) {
		ctx := contextWithActor(actorID) // actor != target

		// Expect check for admin permission
		ts.authz.EXPECT().Authorize(ctx, mock.MatchedBy(func(check rbac.PermissionCheck) bool {
			return check.ActorID == actorID &&
				check.PermissionName == auth.PermAuthSessionRead &&
				check.ResourceContext["target_account_id"] == targetAccountID.String()
		})).Return(true, nil).Once() // Return true for admin

		ts.repo.EXPECT().ListSessionsForAccount(ctx, targetAccountID).Return(sessions, nil).Once()

		result, err := ts.service.List(ctx, targetAccountID)
		require.NoError(t, err)
		assert.Equal(t, sessions, result)
	})

	t.Run("should deny access to non-owner non-admin", func(t *testing.T) {
		ctx := contextWithActor(actorID)
		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(false, nil).Once()

		_, err := ts.service.List(ctx, targetAccountID)
		assert.ErrorIs(t, err, rbac.ErrAccessDenied)
	})

	t.Run("should return unauthorized if no actor in context", func(t *testing.T) {
		_, err := ts.service.List(context.Background(), targetAccountID)
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})
}

func TestSessionService_Revoke(t *testing.T) {
	ts := setupSessionService(t)
	actorID := uuid.New()
	sessionID := uuid.New()
	targetAccountID := uuid.New()
	session := &auth.Session{ID: sessionID, AccountID: targetAccountID}

	t.Run("should allow owner to revoke their own session", func(t *testing.T) {
		ctx := context.Background() // Context doesn't need actor for this method, actorID is passed explicitly

		ts.repo.EXPECT().GetSessionByID(ctx, sessionID).Return(&auth.Session{ID: sessionID, AccountID: actorID}, nil).Once()

		// Owner check passes (actorID == session.AccountID)
		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(false, nil).Once()

		ts.repo.EXPECT().DeleteSession(ctx, sessionID).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e audit.Event) bool {
			return e.Action == core.EventSessionRevoked && e.ActorID == actorID
		})).Return(nil).Once()

		err := ts.service.Revoke(ctx, actorID, sessionID)
		require.NoError(t, err)
	})

	t.Run("should allow admin to revoke another user's session", func(t *testing.T) {
		ctx := context.Background()

		ts.repo.EXPECT().GetSessionByID(ctx, sessionID).Return(session, nil).Once()

		// Admin check must pass
		ts.authz.EXPECT().Authorize(ctx, mock.MatchedBy(func(check rbac.PermissionCheck) bool {
			return check.ActorID == actorID &&
				check.PermissionName == auth.PermAuthSessionRevoke
		})).Return(true, nil).Once()

		ts.repo.EXPECT().DeleteSession(ctx, sessionID).Return(nil).Once()
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.Revoke(ctx, actorID, sessionID)
		require.NoError(t, err)
	})

	t.Run("should deny access to non-owner non-admin", func(t *testing.T) {
		ctx := context.Background()
		ts.repo.EXPECT().GetSessionByID(ctx, sessionID).Return(session, nil).Once()
		ts.authz.EXPECT().Authorize(ctx, mock.Anything).Return(false, nil).Once()

		err := ts.service.Revoke(ctx, actorID, sessionID)
		assert.ErrorIs(t, err, rbac.ErrAccessDenied)
	})

	t.Run("should return error if session not found", func(t *testing.T) {
		ctx := context.Background()
		ts.repo.EXPECT().GetSessionByID(ctx, sessionID).Return(nil, auth.ErrSessionNotFound).Once()

		err := ts.service.Revoke(ctx, actorID, sessionID)
		assert.ErrorIs(t, err, auth.ErrSessionNotFound)
	})
}
