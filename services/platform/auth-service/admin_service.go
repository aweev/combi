package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/middleware"
)

// Statically verify implementation.
var _ auth.AdminService = (*AdminService)(nil)

// AdminService provides the implementation for administrative authentication functions.
type AdminService struct {
	repo           auth.Repository
	auditRecorder  audit.EventRecorder
	accountService auth.AccountService
	mainService    auth.Service
	authzService   rbac.AuthorizationService
	logger         log.Logger
}

// NewAdminService creates a new auth admin service.
func NewAdminService(
	repo auth.Repository,
	auditRecorder audit.EventRecorder,
	accountService auth.AccountService,
	mainService auth.Service,
	authzService rbac.AuthorizationService,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:           repo,
		auditRecorder:  auditRecorder,
		accountService: accountService,
		mainService:    mainService,
		authzService:   authzService,
		logger:         logger.With(log.String("component", "auth_admin_service")),
	}
}

// UpdateAccountStatus changes an account's status (e.g., active, suspended).
func (s *AdminService) UpdateAccountStatus(ctx context.Context, accountID uuid.UUID, status auth.AccountStatus, reason *string) (*auth.Account, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		// Allow for internal system calls, but default to a system actor for auditing.
		actor = &auth.Actor{UserID: audit.SystemActorID}
	}

	var updatedAccount *auth.Account
	var oldStatus auth.AccountStatus

	err := s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		acc, err := txRepo.GetAccountByID(ctx, accountID)
		if err != nil {
			return err
		}
		oldStatus = acc.Status

		if acc.Status == status {
			updatedAccount = acc // Operation is idempotent, no change needed.
			return nil
		}

		acc.Status = status
		acc.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateAccount(ctx, acc); err != nil {
			return fmt.Errorf("failed to update account status in repository: %w", err)
		}

		// CRITICAL SIDE EFFECT: If an account is suspended, invalidate all of its sessions.
		if status == auth.StatusSuspended {
			s.logger.Warn("account suspended, invalidating all sessions", log.String("account_id", accountID.String()))
			if err := txRepo.RevokeAllForAccount(ctx, accountID); err != nil {
				// We MUST roll back the transaction if session invalidation fails.
				return fmt.Errorf("failed to invalidate sessions during suspension: %w", err)
			}
		}

		updatedAccount = acc
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit the event only if a change actually happened.
	if oldStatus != updatedAccount.Status {
		oldValues, _ := json.Marshal(map[string]interface{}{"status": oldStatus})
		newValues, _ := json.Marshal(map[string]interface{}{"status": updatedAccount.Status, "reason": reason})

		event := audit.NewEvent(actor.UserID, "admin", core.EventAdminAccountStatusUpdated).
			WithSuccess().
			WithTarget(accountID, "account", updatedAccount.Email.String()).
			WithSensitiveData(audit.SensitiveData{OldValues: oldValues, NewValues: newValues}).
			WithContext(ctx).
			Build()
		// Recording audit events should not fail the primary operation.
		_ = s.auditRecorder.Record(context.Background(), event)
	}

	return updatedAccount, nil
}

// AdminUpdateAccountEmail allows an admin to change a user's email address.
func (s *AdminService) AdminUpdateAccountEmail(ctx context.Context, accountID uuid.UUID, newEmailStr string) error {
	newEmail, err := core.NewEmail(newEmailStr)
	if err != nil {
		return err
	}
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	var oldEmail string
	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		if existing, err := txRepo.GetAccountByEmail(ctx, newEmail); err == nil && existing != nil {
			if existing.ID != accountID {
				return auth.ErrAccountExists
			}
		}

		acc, err := txRepo.GetAccountByID(ctx, accountID)
		if err != nil {
			return err
		}
		oldEmail = acc.Email.String()

		acc.Email = newEmail
		acc.Status = auth.StatusUnverified // Force the user to re-verify their new email.
		acc.UpdatedAt = time.Now().UTC()

		return txRepo.UpdateAccount(ctx, acc)
	})

	if err != nil {
		return err
	}

	event := audit.NewEvent(actor.UserID, "admin", core.EventAdminAccountEmailUpdated).
		WithSuccess().
		WithTarget(accountID, "account", newEmail.String()).
		WithMetadata(map[string]interface{}{"old_email": oldEmail}).
		WithContext(ctx).
		Build()
	_ = s.auditRecorder.Record(context.Background(), event)

	s.logger.Info("admin updated account email",
		log.String("account_id", accountID.String()),
		log.String("new_email", newEmail.String()),
	)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.accountService.RequestEmailVerification(bgCtx, newEmail); err != nil {
			s.logger.Error("Failed to send verification email after admin update", err,
				log.String("account_id", accountID.String()),
				log.String("new_email", newEmail.String()),
			)
		}
	}()

	return nil
}

// AdminInvalidateAllSessionsForAccount forcibly logs a user out of all devices.
func (s *AdminService) AdminInvalidateAllSessionsForAccount(ctx context.Context, accountID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	if err := s.repo.RevokeAllForAccount(ctx, accountID); err != nil {
		return fmt.Errorf("failed to revoke all sessions in repository: %w", err)
	}

	event := audit.NewEvent(actor.UserID, "admin", core.EventAdminAllSessionsRevoked).
		WithSuccess().
		WithTarget(accountID, "account", "").
		WithContext(ctx).
		Build()
	_ = s.auditRecorder.Record(context.Background(), event)

	s.logger.Warn("admin invalidated all sessions for account", log.String("account_id", accountID.String()))
	return nil
}

// Impersonate creates a new session for a target user on behalf of an authorized admin.
func (s *AdminService) Impersonate(ctx context.Context, targetAccountID uuid.UUID, reason string) (*auth.TokenPair, error) {
	adminActor := auth.ActorFromContext(ctx)
	if adminActor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 1. Authorization Check.
	check := rbac.PermissionCheck{
		ActorID:        adminActor.AccountID,
		PermissionName: auth.PermAuthAccountImpersonate,
		ResourceContext: map[string]any{
			"target_account_id": targetAccountID.String(),
		},
	}
	allowed, err := s.authzService.Authorize(ctx, check)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, rbac.ErrAccessDenied
	}

	// 2. Fetch the target account.
	targetAccount, err := s.repo.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		return nil, err
	}

	// Prevent an admin from impersonating another admin or themselves.
	// This requires an RBAC check on the *target* user.
	// For simplicity, we'll skip this now, but in a real system it's a good safety rail.
	if targetAccount.ID == adminActor.AccountID {
		return nil, rbac.ErrAccessDenied
	}

	// 3. Create an impersonated session.
	// We pass the admin's AccountID as the impersonatorID.
	// Extract IP and UserAgent from context if available.
	meta := middleware.GetRequestMetadata(ctx)
	ip, ua := "internal", "impersonation"
	if meta != nil {
		ip, ua = meta.IPAddress, meta.UserAgent
	}

	tokenPair, err := s.mainService.CreateSessionForAccount(ctx, targetAccount, ip, ua, &adminActor.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to create impersonation session: %w", err)
	}

	// 4. Audit the event.
	event := audit.NewEvent(adminActor.AccountID, "admin", core.EventAdminImpersonationStarted).
		WithSuccess().
		WithTarget(targetAccountID, "account", targetAccount.Email.String()).
		WithMetadata(map[string]interface{}{"reason": reason}).
		WithContext(ctx).
		Build()
	_ = s.auditRecorder.Record(context.Background(), event)

	s.logger.Warn("Admin impersonation session started",
		log.String("admin_id", adminActor.AccountID.String()),
		log.String("target_id", targetAccountID.String()),
	)

	return tokenPair, nil
}
