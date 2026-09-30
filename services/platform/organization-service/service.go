package organization

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ organization.Service = (*Service)(nil)

type Service struct {
	repo          organization.Repository
	userSvc       user.Service
	notifSvc      notification.Service
	auditRecorder audit.EventRecorder
	dispatcher    queue.Dispatcher
	tokenGen      security.TokenGenerator
	logger        log.Logger
}

func NewService(
	repo organization.Repository,
	userSvc user.Service,
	notifSvc notification.Service,
	audit audit.EventRecorder,
	dispatcher queue.Dispatcher,
	tokenGen security.TokenGenerator,
	logger log.Logger,
) *Service {
	return &Service{
		repo:          repo,
		userSvc:       userSvc,
		notifSvc:      notifSvc,
		auditRecorder: audit,
		dispatcher:    dispatcher,
		tokenGen:      tokenGen,
		logger:        logger.With(log.String("component", "org_service")),
	}
}

// --- Organization Management ---

func (s *Service) CreateOrganization(ctx context.Context, params organization.CreateOrganizationParams) (*organization.Organization, error) {
	actor := auth.ActorFromContext(ctx)

	// 1. Prepare Data
	orgID := uuid.New()
	ownerRoleID := uuid.New()
	now := time.Now().UTC()

	status := params.Status
	if status == "" {
		status = organization.StatusActive
	}

	org := &organization.Organization{
		ID:        orgID,
		OwnerID:   actor.UserID,
		Name:      params.Name,
		Slug:      params.Slug,
		Type:      params.Type,
		Status:    status,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ownerRole := &organization.Role{
		ID:             ownerRoleID,
		OrganizationID: orgID,
		Name:           "Owner",
		Description:    "Full access to all organization resources",
		IsSystemRole:   true,
		// Grant "Admin" permission which implies all others in our RBAC logic
		Permissions: []string{string(organization.PermOrganizationAdmin)},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	member := &organization.Member{
		OrganizationID: orgID,
		UserID:         actor.UserID,
		RoleID:         ownerRoleID,
		JoinedAt:       now,
	}

	// 2. Execute Transaction (Org + Role + Member)
	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		if err := txRepo.CreateOrganization(ctx, org); err != nil {
			return err
		}
		if err := txRepo.CreateRole(ctx, ownerRole); err != nil {
			return err
		}
		if err := txRepo.CreateMember(ctx, member); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, s.wrapError(err)
	}

	// 3. Audit
	s.audit(ctx, core.EventOrganizationCreated, orgID, orgID, "organization", nil)

	return org, nil
}

func (s *Service) GetOrganization(ctx context.Context, orgID uuid.UUID) (*organization.Organization, error) {
	// Authorization: User must be a member
	if err := s.checkMembership(ctx, orgID); err != nil {
		return nil, err
	}

	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, s.wrapError(err)
	}
	return org, nil
}

func (s *Service) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]*organization.Organization, error) {
	// Only allow listing own organizations unless Admin (omitted for brevity)
	actor := auth.ActorFromContext(ctx)
	if actor.UserID != userID {
		return nil, organization.ErrPermissionDenied
	}

	return s.repo.ListOrganizationsForUser(ctx, userID)
}

func (s *Service) UpdateOrganization(ctx context.Context, orgID uuid.UUID, params organization.UpdateOrganizationParams) (*organization.Organization, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return nil, err
	}

	var updated *organization.Organization

	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		org, err := txRepo.GetOrganizationByID(ctx, orgID)
		if err != nil {
			return err
		}

		if params.Name != nil {
			org.Name = *params.Name
		}
		if params.Slug != nil {
			org.Slug = *params.Slug
		}
		if params.Status != nil {
			org.Status = *params.Status
		}
		if params.AvatarID != nil {
			org.AvatarID = params.AvatarID
		}

		if err := txRepo.UpdateOrganization(ctx, org); err != nil {
			return err
		}
		updated = org
		return nil
	})

	if err != nil {
		return nil, s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, orgID, "organization", nil)
	return updated, nil
}

func (s *Service) DeleteOrganization(ctx context.Context, orgID uuid.UUID) error {
	// Only Owner can delete
	if err := s.requireOwner(ctx, orgID); err != nil {
		return err
	}

	if err := s.repo.DeleteOrganization(ctx, orgID); err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationDeleted, orgID, orgID, "organization", nil)
	return nil
}

func (s *Service) TransferOwnership(ctx context.Context, orgID, newOwnerUserID uuid.UUID) error {
	// Logic:
	// 1. Caller must be current Owner.
	// 2. New Owner must be a Member.
	// 3. Swap "Owner" role:
	//    - Demote current owner to "Admin" (or create Admin role if missing? For simplicity, we demote to Member or keep as Owner role but update Org.OwnerID).
	//    Actually, "Ownership" is strictly defined by `organizations.owner_id`. The "Owner" role is just for permissions.
	//    Best Practice: Both users keep "Owner" role, but database ownership column changes.

	if err := s.requireOwner(ctx, orgID); err != nil {
		return err
	}

	// Verify target is a member
	if _, err := s.repo.GetMember(ctx, orgID, newOwnerUserID); err != nil {
		return organization.ErrNotOrganizationMember
	}

	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		org, err := txRepo.GetOrganizationByID(ctx, orgID)
		if err != nil {
			return err
		}
		org.OwnerID = newOwnerUserID

		// Ensure new owner has the System Owner Role
		ownerRole, err := txRepo.GetSystemRole(ctx, orgID, "Owner")
		if err != nil {
			return err
		}

		// Update new owner's role
		targetMember, _ := txRepo.GetMember(ctx, orgID, newOwnerUserID)
		if targetMember.RoleID != ownerRole.ID {
			targetMember.RoleID = ownerRole.ID
			if err := txRepo.UpdateMember(ctx, targetMember); err != nil {
				return err
			}
		}

		return txRepo.UpdateOrganization(ctx, org)
	})

	if err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationOwnerChanged, orgID, newOwnerUserID, "user", nil)
	return nil
}

// --- Member Management ---

func (s *Service) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*organization.MemberDetails, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersList); err != nil {
		return nil, err
	}
	return s.repo.ListMemberWithDetails(ctx, orgID)
}

func (s *Service) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organization.MemberDetails, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersList); err != nil {
		return nil, err
	}
	return s.repo.GetMemberWithDetails(ctx, orgID, userID)
}

func (s *Service) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersRemove); err != nil {
		return err
	}

	// Cannot remove the Owner
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return s.wrapError(err)
	}
	if org.OwnerID == userID {
		return organization.ErrCannotRemoveLastOwner
	}

	if err := s.repo.DeleteMember(ctx, orgID, userID); err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventMemberRemoved, orgID, userID, "member", nil)
	return nil
}

func (s *Service) UpdateMemberRole(ctx context.Context, orgID, userID, roleID uuid.UUID) (*organization.Member, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersUpdateRole); err != nil {
		return nil, err
	}

	var updated *organization.Member

	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		// Prevent changing role of the Org Owner (safety check)
		org, err := txRepo.GetOrganizationByID(ctx, orgID)
		if err != nil {
			return err
		}
		if org.OwnerID == userID {
			return organization.ErrCannotChangeOwnerRole
		}

		// Verify Role exists
		if _, err := txRepo.GetRoleByID(ctx, roleID); err != nil {
			return organization.ErrRoleNotFound
		}

		m, err := txRepo.GetMember(ctx, orgID, userID)
		if err != nil {
			return err
		}
		m.RoleID = roleID

		if err := txRepo.UpdateMember(ctx, m); err != nil {
			return err
		}
		updated = m
		return nil
	})

	if err != nil {
		return nil, s.wrapError(err)
	}

	s.audit(ctx, core.EventMemberRoleChanged, orgID, userID, "member", map[string]interface{}{"role_id": roleID})
	return updated, nil
}

func (s *Service) LeaveOrganization(ctx context.Context, orgID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)

	// Cannot leave if Owner
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return s.wrapError(err)
	}
	if org.OwnerID == actor.UserID {
		return organization.ErrCannotRemoveLastOwner
	}

	if err := s.repo.DeleteMember(ctx, orgID, actor.UserID); err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventMemberRemoved, orgID, actor.UserID, "member", map[string]interface{}{"reason": "left"})
	return nil
}

// --- Invitation Management ---

func (s *Service) InviteMember(ctx context.Context, orgID uuid.UUID, params organization.InviteMemberParams) (*organization.Invitation, error) {
	actor := auth.ActorFromContext(ctx)

	if err := s.requirePermission(ctx, orgID, organization.PermMembersInvite); err != nil {
		return nil, err
	}

	// 1. Check if user is already a member
	targetUser, err := s.userSvc.GetPublicProfileByEmail(ctx, params.Email.String())
	if err == nil && targetUser != nil {
		if actor.UserID == targetUser.UserID {
			return nil, organization.ErrCannotInviteSelf
		}
		_, err := s.repo.GetMember(ctx, orgID, targetUser.UserID)
		if err == nil {
			return nil, organization.ErrUserIsAlreadyMember
		}
	}

	// 2. Check for existing pending invite
	_, err = s.repo.GetPendingInvitationsByEmail(ctx, orgID, params.Email.String())
	if err == nil {
		return nil, organization.ErrInvitationAlreadySent
	}

	// 3. Create Invitation (FIXED: Using TokenGenerator correctly)
	// Generate returns (rawToken, hashedToken, error)
	rawToken, hashedToken, err := s.tokenGen.Generate()
	if err != nil {
		return nil, apperror.New(500, "TOKEN_GEN_FAILED", "Failed to generate invitation token", err)
	}

	inv := &organization.Invitation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          params.Email.String(),
		RoleID:         params.RoleID,
		InvitedBy:      actor.UserID,
		Status:         organization.InvitationStatusPending,
		TokenHash:      hashedToken, // Store the hash
		ExpiresAt:      time.Now().UTC().Add(7 * 24 * time.Hour),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, s.wrapError(err)
	}

	// 4. Dispatch Email Job (Pass the RAW token so it can be emailed)
	jobPayload := map[string]interface{}{
		"email":   inv.Email,
		"org_id":  orgID,
		"token":   rawToken, // SEND RAW
		"inviter": actor.UserID,
	}
	payloadBytes, _ := json.Marshal(jobPayload)

	if _, err := s.dispatcher.Dispatch(ctx, organization.JobOrgSendInviteEmail, payloadBytes); err != nil {
		s.logger.Error("Failed to dispatch invite email job", err)
	}

	s.audit(ctx, core.EventMemberInvited, orgID, uuid.Nil, "email", map[string]interface{}{"email": inv.Email})

	return inv, nil
}

func (s *Service) ListPendingInvitations(ctx context.Context, orgID uuid.UUID) ([]*organization.InvitationDetails, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersList); err != nil {
		return nil, err
	}
	return s.repo.ListPendingInvitationsWithDetails(ctx, orgID)
}

func (s *Service) CancelInvitation(ctx context.Context, orgID, invitationID uuid.UUID) error {
	if err := s.requirePermission(ctx, orgID, organization.PermMembersInvite); err != nil {
		return err
	}

	inv, err := s.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return s.wrapError(err)
	}
	if inv.OrganizationID != orgID {
		return organization.ErrInvitationNotFound
	}

	inv.Status = organization.InvitationStatusCanceled
	if err := s.repo.UpdateInvitation(ctx, inv); err != nil {
		return s.wrapError(err)
	}
	return nil
}

func (s *Service) GetInvitationDetailsByToken(ctx context.Context, token string) (*organization.InvitationDetails, error) {
	// Re-hash the incoming raw token to find it in DB
	tokenHash := s.tokenGen.Hash(token)

	inv, err := s.repo.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, organization.ErrInvitationNotFound
	}

	if inv.Status != organization.InvitationStatusPending || inv.ExpiresAt.Before(time.Now().UTC()) {
		return nil, organization.ErrInvitationExpired
	}

	return s.repo.GetInvitationWithDetails(ctx, inv.ID)
}

func (s *Service) AcceptInvitation(ctx context.Context, token string, acceptingUserID uuid.UUID) error {
	tokenHash := s.tokenGen.Hash(token)

	inv, err := s.repo.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return organization.ErrInvitationNotFound
	}
	return s.processAcceptance(ctx, inv, acceptingUserID)
}

func (s *Service) AcceptInvitationInternal(ctx context.Context, invitationID, acceptingUserID uuid.UUID) error {
	inv, err := s.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return organization.ErrInvitationNotFound
	}
	return s.processAcceptance(ctx, inv, acceptingUserID)
}

func (s *Service) processAcceptance(ctx context.Context, inv *organization.Invitation, userID uuid.UUID) error {
	if inv.Status != organization.InvitationStatusPending {
		return organization.ErrInvitationAlreadyAccepted // or expired
	}
	if inv.ExpiresAt.Before(time.Now().UTC()) {
		return organization.ErrInvitationExpired
	}

	// Check if user is already a member
	if _, err := s.repo.GetMember(ctx, inv.OrganizationID, userID); err == nil {
		// Already member, just close invite
		inv.Status = organization.InvitationStatusAccepted
		inv.AcceptedBy = &userID
		_ = s.repo.UpdateInvitation(ctx, inv)
		return nil
	}

	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		// 1. Create Member
		member := &organization.Member{
			OrganizationID: inv.OrganizationID,
			UserID:         userID,
			RoleID:         inv.RoleID,
			JoinedAt:       time.Now().UTC(),
		}
		if err := txRepo.CreateMember(ctx, member); err != nil {
			return err
		}

		// 2. Update Invitation
		inv.Status = organization.InvitationStatusAccepted
		inv.AcceptedBy = &userID
		if err := txRepo.UpdateInvitation(ctx, inv); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventInvitationAccepted, inv.OrganizationID, userID, "user", nil)
	return nil
}

func (s *Service) DeclineInvitation(ctx context.Context, token string) error {
	tokenHash := s.tokenGen.Hash(token)

	inv, err := s.repo.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return organization.ErrInvitationNotFound
	}

	if inv.Status != organization.InvitationStatusPending {
		return nil // Idempotent
	}

	inv.Status = organization.InvitationStatusDeclined
	if err := s.repo.UpdateInvitation(ctx, inv); err != nil {
		return s.wrapError(err)
	}
	return nil
}

// --- Domain Management ---

func (s *Service) AddDomain(ctx context.Context, orgID uuid.UUID, params organization.AddDomainParams) (*organization.OrganizationDomain, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	domain := &organization.OrganizationDomain{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Domain:         params.Domain,
		IsPrimary:      params.IsPrimary,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateDomain(ctx, domain); err != nil {
		return nil, s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, domain.ID, "domain", map[string]interface{}{"domain": domain.Domain})
	return domain, nil
}

func (s *Service) ListDomains(ctx context.Context, orgID uuid.UUID) ([]*organization.OrganizationDomain, error) {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return nil, err
	}
	return s.repo.ListDomains(ctx, orgID)
}

func (s *Service) RemoveDomain(ctx context.Context, orgID, domainID uuid.UUID) error {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return err
	}

	domain, err := s.repo.GetDomain(ctx, domainID)
	if err != nil {
		return s.wrapError(err)
	}
	if domain.OrganizationID != orgID {
		return organization.ErrPermissionDenied
	}

	if err := s.repo.DeleteDomain(ctx, domainID); err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, domainID, "domain_removed", nil)
	return nil
}

func (s *Service) SetPrimaryDomain(ctx context.Context, orgID, domainID uuid.UUID) error {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return err
	}

	domains, err := s.repo.ListDomains(ctx, orgID)
	if err != nil {
		return s.wrapError(err)
	}

	err = s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		for _, d := range domains {
			if d.ID == domainID {
				d.IsPrimary = true
			} else {
				d.IsPrimary = false
			}
			if err := txRepo.UpdateDomain(ctx, d); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, domainID, "domain_primary_set", nil)
	return nil
}

func (s *Service) VerifyDomain(ctx context.Context, orgID, domainID uuid.UUID) error {
	if err := s.requirePermission(ctx, orgID, organization.PermOrgUpdate); err != nil {
		return err
	}

	domain, err := s.repo.GetDomain(ctx, domainID)
	if err != nil {
		return s.wrapError(err)
	}
	if domain.OrganizationID != orgID {
		return organization.ErrPermissionDenied
	}

	now := time.Now().UTC()
	domain.VerifiedAt = &now
	if err := s.repo.UpdateDomain(ctx, domain); err != nil {
		return s.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, domainID, "domain_verified", nil)
	return nil
}

func (s *Service) ResolveOrganizationByDomain(ctx context.Context, domain string) (*organization.Organization, error) {
	// Attempt resolution by Custom Domain
	org, err := s.repo.GetOrganizationByDomain(ctx, domain)
	if err == nil {
		return org, nil
	}

	// Attempt resolution by Subdomain if domain matches platform pattern
	// e.g., mapping redcross.nand-ev.org to slug redcross
	// This logic should be adapted to the actual platform domain config.
	// For now, assume if it looks like a subdomain we try the SLUG lookup.
	// In a real implementation, you'd parse the hostname.

	return s.repo.GetOrganizationBySlug(ctx, domain)
}

// --- Helpers ---

func (s *Service) checkMembership(ctx context.Context, orgID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if _, err := s.repo.GetMember(ctx, orgID, actor.UserID); err != nil {
		return organization.ErrPermissionDenied
	}
	return nil
}

func (s *Service) requireOwner(ctx context.Context, orgID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return err
	}
	if org.OwnerID != actor.UserID {
		return organization.ErrMustBeOwner
	}
	return nil
}

func (s *Service) requirePermission(ctx context.Context, orgID uuid.UUID, perm core.PermissionName) error {
	actor := auth.ActorFromContext(ctx)

	member, err := s.repo.GetMember(ctx, orgID, actor.UserID)
	if err != nil {
		return organization.ErrPermissionDenied
	}

	role, err := s.repo.GetRoleByID(ctx, member.RoleID)
	if err != nil {
		return organization.ErrPermissionDenied
	}

	for _, p := range role.Permissions {
		if core.PermissionName(p) == organization.PermOrganizationAdmin || core.PermissionName(p) == perm {
			return nil
		}
	}

	return organization.ErrPermissionDenied
}

func (s *Service) wrapError(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.New(500, "ORG_OPERATION_FAILED", "An unexpected error occurred", err)
}

func (s *Service) audit(ctx context.Context, action core.EventAction, orgID, targetID uuid.UUID, targetType string, meta map[string]interface{}) {
	actor := auth.ActorFromContext(ctx)
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["org_id"] = orgID

	event := audit.NewEvent(actor.UserID, "organization", action).
		WithTarget(targetID, targetType, "").
		WithMetadata(meta).
		WithContext(ctx).
		Build()

	// FIX: Wrapped Record call in a function to handle/log the error
	go func() {
		if err := s.auditRecorder.Record(context.Background(), event); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}()
}
