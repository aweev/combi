package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
)

// Statically verify that it implements the domain interface.
var _ auth.SessionService = (*SessionService)(nil)

// SessionService provides functionality for managing an account's active sessions.
type SessionService struct {
	repo   auth.Repository
	audit  audit.EventRecorder
	authz  rbac.AuthorizationService
	logger log.Logger
}

// NewSessionService creates a new session management service.
func NewSessionService(
	repo auth.Repository,
	auditRecorder audit.EventRecorder,
	authz rbac.AuthorizationService,
	logger log.Logger,
) *SessionService {
	return &SessionService{
		repo:   repo,
		audit:  auditRecorder,
		authz:  authz,
		logger: logger.With(log.String("service", "session_service")),
	}
}

// List retrieves all active sessions for a given account.
func (s *SessionService) List(ctx context.Context, accountID uuid.UUID) ([]*auth.Session, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}
	isOwner := actor.AccountID == accountID

	check := rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: auth.PermAuthSessionRead,
		ResourceContext: map[string]any{
			"target_account_id": accountID.String(),
		},
	}
	isAdmin, _ := s.authz.Authorize(ctx, check)

	if !isOwner && !isAdmin {
		return nil, rbac.ErrAccessDenied
	}

	return s.repo.ListSessionsForAccount(ctx, accountID)
}

// Revoke revokes a specific session, logging the user out of that device.
func (s *SessionService) Revoke(ctx context.Context, actorID, sessionToRevokeID uuid.UUID) error {
	session, err := s.repo.GetSessionByID(ctx, sessionToRevokeID)
	if err != nil {
		return err // Let the handler map ErrSessionNotFound
	}

	isOwner := actorID == session.AccountID

	check := rbac.PermissionCheck{
		ActorID:        actorID,
		PermissionName: auth.PermAuthSessionRevoke,
		ResourceContext: map[string]any{
			"target_account_id": session.AccountID.String(),
		},
	}
	isAdmin, _ := s.authz.Authorize(ctx, check)

	if !isOwner && !isAdmin {
		return rbac.ErrAccessDenied
	}

	if err := s.repo.DeleteSession(ctx, sessionToRevokeID); err != nil {
		return err
	}

	// Fire audit event
	auditEvent := audit.NewEvent(actorID, "user", core.EventSessionRevoked).
		WithSuccess().
		WithTarget(session.AccountID, "account", ""). // Target is the account whose session was revoked
		WithMetadata(map[string]interface{}{"revoked_session_id": sessionToRevokeID}).
		WithContext(ctx).
		Build()

	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record session revocation audit event", err)
	}

	return nil
}
