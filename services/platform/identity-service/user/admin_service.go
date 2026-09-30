// internal/services/user/admin_service.go
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
)

var _ user.AdminService = (*AdminService)(nil)

// AdminService provides administrative functions for user management.
// It enforces strict authorization checks for all its methods.
type AdminService struct {
	repo          user.Repository
	auditRecorder audit.EventRecorder
	authAdminSvc  auth.AdminService
	authz         rbac.AuthorizationService
	storageSvc    storage.Service
	logger        log.Logger
}

// NewAdminService creates a new user admin service instance.
func NewAdminService(
	repo user.Repository,
	auditRecorder audit.EventRecorder,
	authAdminSvc auth.AdminService,
	authz rbac.AuthorizationService,
	storageSvc storage.Service,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:          repo,
		auditRecorder: auditRecorder,
		authAdminSvc:  authAdminSvc,
		authz:         authz,
		storageSvc:    storageSvc,
		logger:        logger.With(log.String("component", "user_admin_service")),
	}
}

// SetAuthAdminService late-binds the auth admin service (account email,
// status, session invalidation during user admin flows). The host calls
// this after both modules are mounted; until then those admin paths have
// no account backend. Neither direction can be an fx edge: auth consumes
// user.Service and user consumes auth.AdminService (mirrors api wiring,
// where both were injected post-construction).
func (s *AdminService) SetAuthAdminService(svc auth.AdminService) {
	s.authAdminSvc = svc
}

// ListUsers retrieves a paginated list of users based on a filter.
// Requires `user:read:any` permission.
func (s *AdminService) ListUsers(ctx context.Context, filter user.ListUsersFilter) ([]*user.User, int, error) {
	// --- Authorization ---
	if err := s.authorize(ctx, user.PermUserRead, uuid.Nil); err != nil {
		return nil, 0, err
	}
	return s.repo.ListUsers(ctx, filter)
}

// GetUserDetails retrieves the full user and profile models for a given user.
// Requires `user:read:any` permission.
func (s *AdminService) GetUserDetails(ctx context.Context, userID uuid.UUID) (*user.UserWithProfile, error) {
	if err := s.authorize(ctx, user.PermUserRead, userID); err != nil {
		return nil, err
	}

	usr, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrProfileNotFound) {
			return &user.UserWithProfile{User: usr, Profile: nil}, nil
		}
		return nil, err
	}

	// ENHANCEMENT: If profile exists and has an avatar, sign the URL so Admin can see it.
	// Note: We are modifying the PublicProfile model logic here slightly by modifying
	// the return struct or we assume the handler handles DTO mapping.
	// The UserWithProfile struct holds the raw Profile model.
	// We don't mutate the Profile model with the URL (it stores FileID).
	// The URL resolution happens in the Handler via DTO mapping helper or here if we extend the model.
	// For cleanliness, let's stick to returning the model and let the Handler resolve the URL
	// OR add a helper method to AdminService to get signed URL.

	return &user.UserWithProfile{User: usr, Profile: profile}, nil
}

// UpdateUser updates a user's core information (name, email).
// Requires `user:manage:any` permission.
func (s *AdminService) UpdateUser(ctx context.Context, userID uuid.UUID, params user.UpdateUserParams) (*user.User, error) {
	// --- Authorization ---
	if err := s.authorize(ctx, user.PermUserManage, userID); err != nil {
		return nil, err
	}
	actor := auth.ActorFromContext(ctx) // Actor is guaranteed to exist after authorize.

	var updatedUser *user.User
	var oldUser user.User

	err := s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		usr, err := txRepo.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		oldUser = *usr // Make a copy for auditing.

		// Coordinate with Auth service if email is being changed.
		if params.Email != nil && usr.Email.String() != params.Email.String() {
			s.logger.Info("Admin updating user email, coordinating with auth service",
				log.String("user_id", userID.String()),
				log.String("new_email", params.Email.String()),
			)
			if err := s.authAdminSvc.AdminUpdateAccountEmail(ctx, userID, params.Email.String()); err != nil {
				// If this fails, the whole transaction should roll back.
				return fmt.Errorf("failed to update email in auth service: %w", err)
			}
			usr.Email = *params.Email
		}

		if params.FirstName != nil {
			usr.FirstName = *params.FirstName
		}
		if params.LastName != nil {
			usr.LastName = *params.LastName
		}

		if err := txRepo.UpdateUser(ctx, usr); err != nil {
			return fmt.Errorf("failed to update user in repository: %w", err)
		}
		updatedUser = usr
		return nil
	})

	if err != nil {
		return nil, err // Errors from repo or auth service will be AppErrors already.
	}

	// Audit the change.
	if oldUser.ID != uuid.Nil { // Ensure we have the old user data to compare
		oldValues, _ := json.Marshal(oldUser)
		newValues, _ := json.Marshal(updatedUser)
		event := audit.NewEvent(actor.UserID, "admin", core.EventUserProfileUpdated).
			WithSuccess().
			WithTarget(userID, "user", updatedUser.FullName()).
			WithSensitiveData(audit.SensitiveData{OldValues: oldValues, NewValues: newValues}).
			WithContext(ctx).Build()

		if err := s.auditRecorder.Record(context.Background(), event); err != nil {
			s.logger.Error("failed to record user update audit event", err)
		}
	}

	return updatedUser, nil
}

// UpdateUserStatus updates a user's status and coordinates with the auth service.
// Requires `user:manage:any` permission.
func (s *AdminService) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status user.UserStatus, reason *string) (*user.User, error) {
	// --- Authorization ---
	if err := s.authorize(ctx, user.PermUserManage, userID); err != nil {
		return nil, err
	}
	actor := auth.ActorFromContext(ctx)

	var updatedUser *user.User
	var oldUser user.User

	err := s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		usr, err := txRepo.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		oldUser = *usr

		if usr.Status == status {
			updatedUser = usr
			return nil // No change, no error.
		}

		s.logger.Info("Admin updating user status",
			log.String("user_id", userID.String()),
			log.String("old_status", string(usr.Status)),
			log.String("new_status", string(status)),
		)
		usr.Status = status

		if err := txRepo.UpdateUser(ctx, usr); err != nil {
			return fmt.Errorf("failed to update user status in repository: %w", err)
		}

		// Map user status to auth status and coordinate with the auth service.
		authStatus, shouldInvalidateSessions := mapUserStatusToAuthAction(status)
		if authStatus != "" {
			if _, err := s.authAdminSvc.UpdateAccountStatus(ctx, usr.ID, authStatus, reason); err != nil {
				return fmt.Errorf("failed to update auth account status: %w", err)
			}
		}

		if shouldInvalidateSessions {
			s.logger.Warn("User status changed to deactivated/suspended, invalidating all sessions", log.String("user_id", userID.String()))
			if err := s.authAdminSvc.AdminInvalidateAllSessionsForAccount(ctx, userID); err != nil {
				// For now, we make this transactional. You could decide to make it a non-fatal error.
				return fmt.Errorf("failed to invalidate user sessions: %w", err)
			}
		}

		updatedUser = usr
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit the change.
	if oldUser.ID != uuid.Nil && oldUser.Status != updatedUser.Status {
		oldValues, _ := json.Marshal(map[string]interface{}{"status": oldUser.Status})
		newValues, _ := json.Marshal(map[string]interface{}{"status": updatedUser.Status, "reason": reason})
		event := audit.NewEvent(actor.UserID, "admin", core.EventUserStatusUpdated).
			WithSuccess().
			WithTarget(userID, "user", updatedUser.FullName()).
			WithSensitiveData(audit.SensitiveData{OldValues: oldValues, NewValues: newValues}).
			WithContext(ctx).Build()

		if err := s.auditRecorder.Record(context.Background(), event); err != nil {
			s.logger.Error("failed to record user status update audit event", err)
		}
	}

	return updatedUser, nil
}

// authorize is a private helper to centralize authorization checks.
func (s *AdminService) authorize(ctx context.Context, permission core.PermissionName, targetID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	check := rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: permission,
	}
	if targetID != uuid.Nil {
		// Add resource context if applicable.
		check.ResourceContext = map[string]any{"user_id": targetID.String()}
	}

	allowed, err := s.authz.Authorize(ctx, check)
	if err != nil {
		return apperror.New(500, "AUTHORIZATION_CHECK_FAILED", "Failed to perform authorization check.", err)
	}
	if !allowed {
		return rbac.ErrAccessDenied
	}
	return nil
}

// mapUserStatusToAuthAction is a helper to decouple user domain statuses from auth domain statuses.
func mapUserStatusToAuthAction(status user.UserStatus) (auth.AccountStatus, bool) {
	switch status {
	case user.StatusActive:
		return auth.StatusActive, false
	case user.StatusDeactivated:
		return auth.StatusSuspended, true
	case user.StatusPending:
		return auth.StatusUnverified, false
	default:
		// For 'Anonymized', we don't change the auth status, just prevent login via user status check.
		return "", false
	}
}
