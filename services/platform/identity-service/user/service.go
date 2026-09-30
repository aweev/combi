package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
)

var _ user.Service = (*Service)(nil)
var _ privacy.Anonymizer = (*Service)(nil)
var _ reporting.DataFetcher = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo          user.Repository
	storageSvc    storage.Service
	auditRecorder audit.EventRecorder
	authz         rbac.AuthorizationService
	dispatcher    queue.Dispatcher
	logger        log.Logger
}

func NewService(
	repo user.Repository,
	storageSvc storage.Service,
	audit audit.EventRecorder,
	authz rbac.AuthorizationService,
	dispatcher queue.Dispatcher,
	logger log.Logger,
) *Service {
	return &Service{
		repo:          repo,
		storageSvc:    storageSvc,
		auditRecorder: audit,
		authz:         authz,
		dispatcher:    dispatcher,
		logger:        logger.With(log.String("component", "user_service")),
	}
}

func (s *Service) FetchData(ctx context.Context, params map[string]any) (any, error) {

	users, _, err := s.repo.ListUsers(ctx, user.ListUsersFilter{
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}

	type UserRow struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Status string `json:"status"`
	}

	rows := make([]UserRow, len(users))
	for i, u := range users {
		rows[i] = UserRow{
			ID:     u.ID.String(),
			Name:   u.FullName(),
			Email:  u.Email.String(),
			Status: string(u.Status),
		}
	}

	return rows, nil
}

// --- Identity Management ---

func (s *Service) CreateUser(ctx context.Context, params user.CreateUserParams) (*user.User, error) {
	// No Auth check here - this is an internal method called by AuthService during registration.

	now := time.Now().UTC()
	newUser := &user.User{
		ID:        params.UserID,
		FirstName: params.FirstName,
		LastName:  params.LastName,
		Email:     params.Email,
		Status:    user.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	profile := &user.Profile{
		ID:          params.UserID,
		DisplayName: newUser.FullName(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		if err := txRepo.CreateUser(ctx, newUser); err != nil {
			return err
		}
		return txRepo.CreateProfile(ctx, profile)
	}); err != nil {
		return nil, apperror.New(500, "USER_CREATION_FAILED", "Failed to persist user records", err)
	}

	// Audit
	event := audit.NewEvent(params.UserID, "user", core.EventUserCreated).
		WithSuccess().
		WithTarget(params.UserID, "user", newUser.FullName()).
		Build()

	// We use a background context for recording to ensure it doesn't block or fail the request
	// if the client disconnects immediately after creation.
	go func() {
		if err := s.auditRecorder.Record(context.Background(), event); err != nil {
			s.logger.Error("failed to record user creation audit event", err)
		}
	}()

	s.logger.Info("User created", log.String("user_id", newUser.ID.String()))
	return newUser, nil
}

func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, s.wrapRepoError(err, "GetUser")
	}
	return u, nil
}

// --- Profile Management ---

func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*user.Profile, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// Authorization Strategy:
	// 1. Check if requesting own profile (Fast path)
	// 2. If not, check if admin (Slow path via RBAC service)

	isSelf := actor.UserID == userID
	if !isSelf {
		allowed, err := s.authz.Authorize(ctx, rbac.PermissionCheck{
			ActorID:        actor.AccountID,
			PermissionName: user.PermUserRead,
		})
		if err != nil {
			return nil, apperror.New(500, "AUTHZ_ERROR", "Authorization check failed", err)
		}
		if !allowed {
			return nil, user.ErrForbidden
		}
	}

	p, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, s.wrapRepoError(err, "GetProfile")
	}
	return p, nil
}

func (s *Service) UpdateProfile(ctx context.Context, actorID, targetUserID uuid.UUID, params user.UpdateProfileParams) (*user.Profile, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// Authorization
	perm := user.PermUserManage
	if actor.UserID == targetUserID {
		perm = user.PermUserProfileUpdateSelf
	}

	allowed, err := s.authz.Authorize(ctx, rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: perm,
	})
	if err != nil {
		return nil, apperror.New(500, "AUTHZ_ERROR", "Authorization check failed", err)
	}
	if !allowed {
		return nil, user.ErrForbidden
	}

	if params.DisplayName != nil {
		if len(*params.DisplayName) < 2 || len(*params.DisplayName) > 50 {
			return nil, apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Display name must be between 2 and 50 chars", nil)
		}
	}
	if params.Bio != nil && len(*params.Bio) > 500 {
		return nil, apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Bio too long", nil)
	}

	// Fetch -> Update -> Save (Transactional)
	var updatedProfile *user.Profile
	var oldProfileJSON []byte

	err = s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		p, err := txRepo.GetProfile(ctx, targetUserID)
		if err != nil {
			return err
		}

		oldProfileJSON, _ = json.Marshal(p) // Snapshot for audit

		if params.DisplayName != nil {
			p.DisplayName = *params.DisplayName
		}
		if params.Bio != nil {
			p.Bio = params.Bio
		}
		if params.CustomFields != nil {
			// Merge logic could go here, currently doing replacement
			p.CustomFields = params.CustomFields
		}
		p.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateProfile(ctx, p); err != nil {
			return err
		}
		updatedProfile = p
		return nil
	})

	if err != nil {
		return nil, s.wrapRepoError(err, "UpdateProfile")
	}

	// Audit
	newProfileJSON, _ := json.Marshal(updatedProfile)
	event := audit.NewEvent(actor.UserID, "user", core.EventUserProfileUpdated).
		WithSuccess().
		WithTarget(targetUserID, "profile", updatedProfile.DisplayName).
		WithSensitiveData(audit.SensitiveData{
			OldValues: oldProfileJSON,
			NewValues: newProfileJSON,
		}).
		WithContext(ctx).
		Build()

	if err := s.auditRecorder.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record profile update audit event", err)
	}

	return updatedProfile, nil
}

func (s *Service) SetAvatar(ctx context.Context, targetUserID, fileID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	// Authz
	perm := user.PermUserManage
	if actor.UserID == targetUserID {
		perm = user.PermUserProfileUpdateSelf
	}
	allowed, err := s.authz.Authorize(ctx, rbac.PermissionCheck{ActorID: actor.AccountID, PermissionName: perm})
	if err != nil || !allowed {
		return user.ErrForbidden
	}

	// Validate File
	file, err := s.storageSvc.GetFile(ctx, fileID)
	if err != nil {
		return err // Storage service returns AppErrors
	}
	if file.Category != storage.CategoryImage {
		return user.ErrInvalidAvatarType
	}
	if file.OwnerID != targetUserID {
		// Enforce that user owns the file they are setting as avatar
		return user.ErrForbidden
	}

	// Update DB
	err = s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		p, err := txRepo.GetProfile(ctx, targetUserID)
		if err != nil {
			return err
		}
		p.AvatarFileID = &fileID
		p.UpdatedAt = time.Now().UTC()
		return txRepo.UpdateProfile(ctx, p)
	})

	if err != nil {
		return s.wrapRepoError(err, "SetAvatar")
	}

	// Audit
	event := audit.NewEvent(actor.UserID, "user", core.EventUserAvatarUpdated).
		WithSuccess().
		WithTarget(targetUserID, "profile", "").
		WithMetadata(map[string]interface{}{"file_id": fileID}).
		WithContext(ctx).
		Build()

	if err := s.auditRecorder.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record avatar update audit event", err)
	}

	return nil
}

// --- Public Profile & Connections ---

func (s *Service) GetPublicProfile(ctx context.Context, userID uuid.UUID) (*user.PublicProfile, error) {
	profiles, err := s.GetPublicProfiles(ctx, []uuid.UUID{userID})
	if err != nil {
		return nil, err
	}
	if p, ok := profiles[userID]; ok {
		return &p, nil
	}
	return nil, user.ErrUserNotFound
}

func (s *Service) GetPublicProfileByEmail(ctx context.Context, email string) (*user.PublicProfile, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, s.wrapRepoError(err, "GetUserByEmail")
	}
	return s.GetPublicProfile(ctx, u.ID)
}

func (s *Service) GetPublicProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]user.PublicProfile, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]user.PublicProfile{}, nil
	}

	profiles, err := s.repo.GetPublicProfiles(ctx, userIDs)
	if err != nil {
		return nil, s.wrapRepoError(err, "GetPublicProfiles")
	}

	for id, p := range profiles {
		if p.AvatarFileID != nil {
			// Short timeout
			signCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			url, err := s.storageSvc.GetDownloadURL(signCtx, *p.AvatarFileID, 1*time.Hour)
			cancel()

			if err == nil {
				p.AvatarURL = &url.URL
				profiles[id] = p // Update map value
			}
		}
	}
	return profiles, nil
}

func (s *Service) GetContactInfo(ctx context.Context, userID uuid.UUID) (*user.ContactInfo, error) {
	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, s.wrapRepoError(err, "GetContactInfo")
	}
	return &user.ContactInfo{Email: u.Email}, nil
}

// --- Connections ---

func (s *Service) CreateConnection(ctx context.Context, actorID, targetUserID uuid.UUID, connType user.ConnectionType) (*user.Connection, error) {
	if actorID == targetUserID {
		return nil, user.ErrCannotConnectToSelf
	}

	// Ensure target exists
	if _, err := s.repo.GetUser(ctx, targetUserID); err != nil {
		return nil, s.wrapRepoError(err, "CreateConnection")
	}

	conn := &user.Connection{
		ID:           uuid.New(),
		SourceUserID: actorID,
		TargetUserID: targetUserID,
		Type:         connType,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateConnection(ctx, conn); err != nil {
		return nil, s.wrapRepoError(err, "CreateConnection")
	}

	// Audit
	event := audit.NewEvent(actorID, "user", core.EventUserConnectionCreated).
		WithSuccess().
		WithTarget(targetUserID, "user", "").
		WithMetadata(map[string]interface{}{"type": connType}).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record connection creation audit event", err)
	}

	return conn, nil
}

func (s *Service) DeleteConnection(ctx context.Context, actorID, targetUserID uuid.UUID, connType user.ConnectionType) error {
	conn, err := s.repo.GetConnection(ctx, actorID, targetUserID, connType)
	if err != nil {
		return s.wrapRepoError(err, "GetConnection")
	}

	if err := s.repo.DeleteConnection(ctx, conn.ID); err != nil {
		return s.wrapRepoError(err, "DeleteConnection")
	}

	event := audit.NewEvent(actorID, "user", core.EventUserConnectionDeleted).
		WithSuccess().
		WithTarget(targetUserID, "user", "").
		WithMetadata(map[string]interface{}{"type": connType}).
		WithContext(ctx).Build()

	if err := s.auditRecorder.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record connection deletion audit event", err)
	}

	return nil
}

func (s *Service) ListConnections(ctx context.Context, userID uuid.UUID, connType user.ConnectionType) ([]user.PublicProfile, error) {
	targetIDs, err := s.repo.ListConnectionTargetIDs(ctx, userID, connType)
	if err != nil {
		return nil, s.wrapRepoError(err, "ListConnections")
	}

	// Reuse GetPublicProfiles logic which handles avatars efficiently
	profilesMap, err := s.GetPublicProfiles(ctx, targetIDs)
	if err != nil {
		return nil, err
	}

	list := make([]user.PublicProfile, 0, len(targetIDs))
	for _, id := range targetIDs {
		if p, ok := profilesMap[id]; ok {
			list = append(list, p)
		}
	}
	return list, nil
}

func (s *Service) ListFollowers(ctx context.Context, userID uuid.UUID, connType user.ConnectionType) ([]user.PublicProfile, error) {
	sourceIDs, err := s.repo.ListConnectionSourceIDs(ctx, userID, connType)
	if err != nil {
		return nil, s.wrapRepoError(err, "ListFollowers")
	}

	profilesMap, err := s.GetPublicProfiles(ctx, sourceIDs)
	if err != nil {
		return nil, err
	}

	list := make([]user.PublicProfile, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		if p, ok := profilesMap[id]; ok {
			list = append(list, p)
		}
	}
	return list, nil
}

// --- Privacy / GDPR ---

func (s *Service) AnonymizeUser(ctx context.Context, userID uuid.UUID) (*privacy.ErasureReport, error) {
	// This implements the Privacy Anonymizer interface
	return s.anonymizeUserInternal(ctx, userID)
}

func (s *Service) anonymizeUserInternal(ctx context.Context, userID uuid.UUID) (*privacy.ErasureReport, error) {
	s.logger.Info("Starting user anonymization", log.String("user_id", userID.String()))

	report := &privacy.ErasureReport{
		Service: "UserService",
	}

	// 1. Anonymize DB Data
	err := s.repo.WithTx(ctx, func(txRepo user.Repository) error {
		return txRepo.AnonymizeUser(ctx, userID)
	})
	if err != nil {
		return nil, apperror.New(500, "ANONYMIZATION_FAILED", "Could not anonymize user records in database.", err)
	}
	report.RecordsAnonymized = 1 // User record + profile data scrubbed

	// 2. Dispatch File Cleanup Job (Reliable)
	payload, err := json.Marshal(map[string]string{"user_id": userID.String()})
	if err != nil {
		s.logger.Error("Failed to marshal payload for cleanup job", err)
	} else {
		_, err = s.dispatcher.Dispatch(ctx, user.JobUserFileCleanup, payload, queue.WithPriority(10)) // Low priority
		if err != nil {
			s.logger.Error("Failed to dispatch file cleanup job", err)
			// We don't fail the request, but log error. Ideally, this should be part of the TX or Outbox.
		}
	}

	return report, nil
}

// ApplySyncChange handles offline profile updates.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	if op.EntityType != "user_profile" {
		return uuid.Nil, 0, fmt.Errorf("user service only supports syncing 'user_profile', got: %s", op.EntityType)
	}

	// 1. Inject Actor Context
	actor := &auth.Actor{UserID: actorID, AccountID: actorID} // Assuming AccountID == UserID for simple setup, or auth service resolves it
	userCtx := auth.NewContextWithActor(ctx, actor)

	switch op.Action {
	case sync_domain.ActionUpdate:
		// Target ID is usually the User's own ID for profile updates
		// But in a generic sync op, it might be in op.EntityID
		targetID := actorID
		if op.EntityID != nil {
			targetID = *op.EntityID
		}

		// Security: Ensure user is updating their own profile
		if targetID != actorID {
			return uuid.Nil, 0, fmt.Errorf("offline profile updates allowed for self only")
		}

		var req struct {
			DisplayName *string                `json:"display_name"`
			Bio         *string                `json:"bio"`
			Custom      map[string]interface{} `json:"custom_fields"`
		}
		if err := json.Unmarshal(op.Payload, &req); err != nil {
			return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
		}

		// Use existing UpdateProfile logic which handles validation & audit
		updatedProfile, err := s.UpdateProfile(userCtx, actorID, targetID, user.UpdateProfileParams{
			DisplayName:  req.DisplayName,
			Bio:          req.Bio,
			CustomFields: req.Custom,
		})
		if err != nil {
			return uuid.Nil, 0, err
		}

		return updatedProfile.ID, int(updatedProfile.UpdatedAt.Unix()), nil

	default:
		return uuid.Nil, 0, fmt.Errorf("unsupported action for user_profile: %s", op.Action)
	}
}

func (s *Service) GetUserLocale(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	// Return locale if set, else return empty string (processor handles default)
	if user.Locale != nil {
		return *user.Locale, nil
	}
	return "", nil
}

// FetchDelta returns the user's own profile if changed.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	// 1. Fetch Profile
	// Since we typically only sync the user's OWN profile, we just fetch it.
	// If you wanted to sync "Contacts" or "Friends", you'd fetch list of connections here too.

	profile, err := s.repo.GetProfile(ctx, actorID)
	if err != nil {
		if errors.Is(err, user.ErrProfileNotFound) {
			return nil, nil // No profile yet?
		}
		return nil, err
	}

	results := make([]sync_domain.EntityState, 0)

	// 2. Check Modification Time
	if profile.UpdatedAt.After(since) {
		data, err := json.Marshal(profile)
		if err != nil {
			s.logger.Error("failed to marshal profile for sync", err)
			// Skip this item if marshal fails
		} else {
			results = append(results, sync_domain.EntityState{
				ID:        profile.ID,
				Version:   int(profile.UpdatedAt.Unix()),
				Deleted:   false,
				Data:      data,
				UpdatedAt: profile.UpdatedAt,
			})
		}
	}

	return results, nil
}

// wrapRepoError ensures all DB errors are converted to AppErrors
func (s *Service) wrapRepoError(err error, op string) error {
	if errors.Is(err, user.ErrUserNotFound) || errors.Is(err, user.ErrProfileNotFound) {
		return err // Already a domain error
	}
	return apperror.New(500, "DB_ERROR", fmt.Sprintf("Repository error in %s", op), err)
}
