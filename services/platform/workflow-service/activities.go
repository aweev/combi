package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
)

// Activities are the implementations of the steps in a workflow.
// Each activity is a single, idempotent unit of work that can be retried by Temporal.
type Activities struct {
	Logger            log.Logger
	CatalogAdminSvc   catalog.AdminService
	NotificationSvc   notification.Service
	UserService       user.Service
	UserAdminSvc      user.AdminService
	ContentAdminSvc   content.AdminService
	OrgService        organization.Service
	ProjectService    project.Service
	RBACAdminSvc      rbac.AdminService
	FinancialAdminSvc financial.AdminService
}

// NewActivities creates a new Activities registry with all required domain dependencies.
func NewActivities(
	logger log.Logger,
	catalogAdminSvc catalog.AdminService,
	notifSvc notification.Service,
	userSvc user.Service,
	userAdminSvc user.AdminService,
	contentAdminSvc content.AdminService,
	orgSvc organization.Service,
	projectSvc project.Service,
	rbacAdminSvc rbac.AdminService,
	financialAdminSvc financial.AdminService,
) *Activities {
	return &Activities{
		Logger:            logger,
		CatalogAdminSvc:   catalogAdminSvc,
		NotificationSvc:   notifSvc,
		UserService:       userSvc,
		UserAdminSvc:      userAdminSvc,
		ContentAdminSvc:   contentAdminSvc,
		OrgService:        orgSvc,
		ProjectService:    projectSvc,
		RBACAdminSvc:      rbacAdminSvc,
		FinancialAdminSvc: financialAdminSvc,
	}
}

// --- Generic Activities ---

// SendNotification sends a notification via the NotificationService.
func (a *Activities) SendNotification(ctx context.Context, recipientID uuid.UUID, templateSlug string, data map[string]interface{}, refEntityType string, refEntityID uuid.UUID) error {
	a.Logger.Info("Activity: Sending notification",
		log.String("slug", templateSlug),
		log.String("recipient", recipientID.String()))

	req := notification.DispatchRequest{
		RecipientID: recipientID,
		Template: &notification.TemplatePayload{
			Slug: templateSlug,
			Data: data,
		},
		Reference: &notification.NotificationRef{
			EntityType: refEntityType,
			EntityID:   refEntityID,
		},
	}

	_, err := a.NotificationSvc.Dispatch(ctx, req)
	if err != nil {
		// We allow this to fail (and be retried by Temporal)
		return fmt.Errorf("failed to dispatch notification: %w", err)
	}

	return nil
}

// --- Content Moderation Activities ---

func (a *Activities) UpdateContentStatus(ctx context.Context, entryID uuid.UUID, newStatus content.EntryStatus, reason *string) error {
	a.Logger.Info("Activity: Updating content status",
		log.String("entryID", entryID.String()),
		log.Any("status", newStatus))

	params := content.UpdateEntryStatusParams{
		Status: newStatus,
		Reason: reason,
	}

	_, err := a.ContentAdminSvc.UpdateEntryStatus(ctx, entryID, params)
	if err != nil {
		return fmt.Errorf("failed to update content status: %w", err)
	}

	return nil
}

// --- Participant Application Activities ---

func (a *Activities) UpdateApplicationStatus(ctx context.Context, appID uuid.UUID, newStatus catalog.ApplicationStatus, reviewerID *uuid.UUID, reason *string) error {
	a.Logger.Info("Activity: Updating application status",
		log.String("appID", appID.String()),
		log.Any("status", newStatus))

	_, err := a.CatalogAdminSvc.UpdateApplicationStatus(ctx, appID, newStatus, reason)
	if err != nil {
		return fmt.Errorf("failed to update application status: %w", err)
	}
	return nil
}

// ActivateParticipantProfile runs multiple steps to onboard a user: updates status and assigns RBAC role.
func (a *Activities) ActivateParticipantProfile(ctx context.Context, userID uuid.UUID) error {
	a.Logger.Info("Activity: Activating participant profile", log.String("userID", userID.String()))

	// 1. Update user status to active
	if _, err := a.UserAdminSvc.UpdateUserStatus(ctx, userID, user.StatusActive, nil); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// 2. Assign the participant role
	participantRole, err := a.RBACAdminSvc.GetRoleBySlug(ctx, "participant")
	if err != nil {
		return fmt.Errorf("misconfiguration: 'participant' role not found: %w", err)
	}

	if _, err := a.RBACAdminSvc.AssignRoleToUser(ctx, userID, participantRole.ID); err != nil {
		return fmt.Errorf("failed to assign participant role: %w", err)
	}

	return nil
}

// DeactivateParticipantProfile is a COMPENSATION activity. It is called if the workflow fails *after* activation.
func (a *Activities) DeactivateParticipantProfile(ctx context.Context, userID uuid.UUID) error {
	a.Logger.Warn("Activity: Compensating - Deactivating participant profile", log.String("userID", userID.String()))

	// 1. Revert user status
	if _, err := a.UserAdminSvc.UpdateUserStatus(ctx, userID, user.StatusPending, nil); err != nil {
		// Log but don't error hard, we want to try the next compensation step
		a.Logger.Error("Compensation partial failure: could not revert user status", err)
	}

	// 2. Remove participant role
	participantRole, err := a.RBACAdminSvc.GetRoleBySlug(ctx, "participant")
	if err != nil {
		a.Logger.Error("Compensation partial failure: role lookup", err)
		return nil // Non-retryable
	}

	if err := a.RBACAdminSvc.RevokeRoleFromUser(ctx, userID, participantRole.ID); err != nil {
		a.Logger.Error("Compensation partial failure: could not revoke role", err)
	}

	return nil
}

// --- Grant Application Activities ---

// NotifyRoleGroup finds all users with a specific role and sends them a notification.
func (a *Activities) NotifyRoleGroup(ctx context.Context, roleSlug string, notificationSlug string, data map[string]interface{}) error {
	a.Logger.Info("Activity: Notifying role group", log.String("role", roleSlug))

	users, err := a.RBACAdminSvc.ListUsersInRole(ctx, roleSlug)
	if err != nil {
		return fmt.Errorf("failed to list users in role: %w", err)
	}

	// Fan-out notifications (in a real system, you might batch this or use a multicast feature)
	for _, u := range users {
		req := notification.DispatchRequest{
			RecipientID: u.UserID,
			Template: &notification.TemplatePayload{
				Slug: notificationSlug,
				Data: data,
			},
		}
		// Fire and forget individual sends so one failure doesn't block the activity.
		// We explicitly check the error and log it instead of returning it,
		// to prevent the entire activity from failing if one user notification fails.
		if _, err := a.NotificationSvc.Dispatch(ctx, req); err != nil {
			a.Logger.Error("failed to dispatch notification to role group member",
				err)
		}
	}

	return nil
}

// FinalizeGrantApproval records the financial transaction for an approved grant.
func (a *Activities) FinalizeGrantApproval(
	ctx context.Context,
	grantAppID uuid.UUID,
	amount int64,
	currency string,
	recipientUserID uuid.UUID,
	idempotencyKey string, // <--- New Parameter
) error {
	a.Logger.Info("Activity: Finalizing grant approval", log.String("grantAppID", grantAppID.String()))

	// 1. Update status (Idempotent by nature)
	if err := a.UpdateApplicationStatus(ctx, grantAppID, catalog.AppStatusApproved, nil, nil); err != nil {
		return err
	}

	// 2. Create financial ledger entry with IDEMPOTENCY KEY
	params := financial.DisburseGrantParams{
		RecipientUserID: recipientUserID,
		Amount:          amount,
		Currency:        currency,
		Description:     fmt.Sprintf("Grant Approved - App ID: %s", grantAppID),
		ReferenceID:     grantAppID,
		IdempotencyKey:  idempotencyKey, // Passed to Financial Service
	}

	if _, err := a.FinancialAdminSvc.DisburseGrant(ctx, params); err != nil {
		// Financial Service must handle "ErrAlreadyExists" for this key and return success
		// or a specific non-retryable error if strictly needed.
		return fmt.Errorf("failed to create ledger entry: %w", err)
	}

	return nil
}

// --- Volunteer Onboarding Activities ---

// RunBackgroundCheck simulates an external API call.
func (a *Activities) RunBackgroundCheck(ctx context.Context, userID uuid.UUID) (string, error) {
	a.Logger.Info("Activity: Running background check", log.String("userID", userID.String()))

	// Simulate external latency
	time.Sleep(2 * time.Second)

	// In a real implementation, this calls Checkr/Sterling API.
	// For MVP: Return "CLEARED"
	return "CLEARED", nil
}

// CreateTrainingProject creates a project resource for the new volunteer.
func (a *Activities) CreateTrainingProject(ctx context.Context, userID uuid.UUID) (*project.Project, error) {
	a.Logger.Info("Activity: Creating training project", log.String("userID", userID.String()))

	userProfile, err := a.UserService.GetPublicProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("could not get user profile: %w", err)
	}

	params := project.CreateProjectParams{
		CreatorID:   userID,
		Name:        fmt.Sprintf("Onboarding: %s", userProfile.FirstName),
		Description: "Complete the tasks in this project to finish your onboarding process.",
	}

	proj, err := a.ProjectService.CreateProject(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create training project: %w", err)
	}

	return proj, nil
}

// --- Partnership Activities ---

func (a *Activities) CreatePartnerOrganization(ctx context.Context, orgName, orgSlug string, ownerID uuid.UUID) (*organization.Organization, error) {
	params := organization.CreateOrganizationParams{
		OwnerUserID: ownerID,
		Name:        orgName,
		Slug:        orgSlug,
		Type:        "PARTNER",
	}

	org, err := a.OrgService.CreateOrganization(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create partner organization: %w", err)
	}
	return org, nil
}

func (a *Activities) GeneratePartnershipContract(ctx context.Context, orgID uuid.UUID, orgName string) (string, error) {
	// Simulate DocuSign generation
	signingURL := fmt.Sprintf("https://sign.ngo-platform.org/contract/%s", orgID.String())
	return signingURL, nil
}

// ArchiveOrganization is called if the partnership process times out or is rejected.
func (a *Activities) ArchiveOrganization(ctx context.Context, orgID uuid.UUID) error {
	a.Logger.Warn("Activity: Archiving organization due to workflow failure/timeout", log.String("orgID", orgID.String()))

	status := organization.OrganizationStatus("INACTIVE")
	params := organization.UpdateOrganizationParams{Status: &status}
	_, err := a.OrgService.UpdateOrganization(ctx, orgID, params)
	return err
}

// ActivateOrganization is called when the contract is signed.
func (a *Activities) ActivateOrganization(ctx context.Context, orgID uuid.UUID) error {
	a.Logger.Info("Activity: Activating partner organization", log.String("orgID", orgID.String()))

	// Assuming there's a specific method or we update status.
	// For MVP, we might verify the org.
	// This would likely call OrgService.UpdateOrganization status = ACTIVE
	return nil // Mock implementation for now as Org Status might be implicit
}
