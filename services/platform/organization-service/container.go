package organization

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)


type Container struct {
	Service      organization.Service
	RoleAdminSvc organization.RoleAdminService
	Workers      []queue.JobProcessor
}

type Dependencies struct {
	DB              *sqlx.DB
	Logger          log.Logger
	Registrar       rbac.PermissionRegistrar
	Registry        core.Registry
	UserService     user.Service
	NotificationSvc notification.Service
	Dispatcher      queue.Dispatcher
	AuditRecorder   audit.EventRecorder
	TokenGen        security.TokenGenerator
	// EventBus is used by Config/Analytics, usually passed implicitly or if needed for specific events
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions
	registerPermissions(deps.Registrar)

	// 2. Register Service Manifest
	deps.Registry.RegisterService(BuildManifest())

	// 3. Infrastructure
	repo := postgres.NewOrganizationRepository(deps.DB, deps.Logger)

	// 4. Services
	svc := NewService(
		repo,
		deps.UserService,
		deps.NotificationSvc,
		deps.AuditRecorder,
		deps.Dispatcher,
		deps.TokenGen,
		deps.Logger,
	)

	roleSvc := NewRoleService(
		repo,
		deps.AuditRecorder,
		deps.Logger,
		svc, // Injected for shared permission helpers
	)

	// 5. Workers
	workers := []queue.JobProcessor{
		NewInviteCleanupWorker(repo, deps.Logger),
		NewInviteSenderWorker(deps.NotificationSvc, deps.Logger),
	}

	return &Container{
		Service:      svc,
		RoleAdminSvc: roleSvc,
		Workers:      workers,
	}, nil
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	const cat = "Organization Management"

	// We use rbac.PermissionDefinition directly
	perms := []rbac.PermissionDefinition{
		{Name: organization.PermOrganizationAdmin, Description: "Full administrative access to an organization.", Category: cat},
		{Name: organization.PermOrgUpdate, Description: "Update organization details (name, slug, avatar).", Category: cat},
		{Name: organization.PermMembersInvite, Description: "Invite new members to the organization.", Category: cat},
		{Name: organization.PermMembersList, Description: "View the member list.", Category: cat},
		{Name: organization.PermMembersRemove, Description: "Remove members from the organization.", Category: cat},
		{Name: organization.PermMembersUpdateRole, Description: "Change the role of existing members.", Category: cat},
		{Name: organization.PermRolesList, Description: "View configured roles.", Category: cat},
		{Name: organization.PermRolesCreate, Description: "Create new custom roles.", Category: cat},
		{Name: organization.PermRolesUpdate, Description: "Modify existing roles.", Category: cat},
		{Name: organization.PermRolesDelete, Description: "Delete custom roles.", Category: cat},
	}

	for _, p := range perms {
		registrar.Register(p)
	}
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "organization-service",
		DisplayName: "Organizations",
		Description: "Multi-tenant organization management, teams, and RBAC.",
		Icon:        "briefcase",
		Events: []core.EventDefinition{
			{Key: core.EventOrganizationCreated, Label: "Org Created", Description: "New organization provisioned"},
			{Key: core.EventMemberInvited, Label: "Member Invited", Description: "Invitation email sent"},
			{Key: core.EventMemberJoined, Label: "Member Joined", Description: "User accepted invitation"},
		},
		// Permissions are registered dynamically via Registrar, but can be listed here for UI/Doc generation if desired.
		// For the manifest used by the Frontend, we typically list them here too.
		Permissions: []core.PermDefinition{
			{Key: organization.PermOrganizationAdmin, Label: "Org Admin", Category: "Organization"},
			{Key: organization.PermMembersInvite, Label: "Invite Members", Category: "Organization"},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         organization.JobOrgCleanupExpiredInvites,
				Label:       "Cleanup Invites",
				Description: "Removes expired invitation records.",
			},
			{
				Key:         organization.JobOrgSendInviteEmail,
				Label:       "Send Invite Email",
				Description: "Delivers invitation emails to new users.",
			},
		},
	}
}
