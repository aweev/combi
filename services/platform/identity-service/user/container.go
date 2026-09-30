// internal/services/user/container.go
package user

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
)


// Container holds the fully constructed services for the user domain.
// AdminService is the concrete type (not the domain interface) so the host
// can late-bind the auth admin service via SetAuthAdminService — auth and
// user depend on each other, so neither direction can be an fx edge.
type Container struct {
	Service      user.Service
	AdminService *AdminService
	Workers      []queue.JobProcessor
}

// Dependencies holds all external services and infrastructure required by the user domain.
type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	StorageSvc    storage.Service
	AuthAdminSvc  auth.AdminService
	RBACService   rbac.AuthorizationService
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
	Dispatcher    queue.Dispatcher
}

// NewContainer builds the user service container and registers its permissions.
func NewContainer(deps Dependencies) (*Container, error) {
	// --- Permission Registration ---
	// It's good practice to define all domain-specific permissions here.
	const category = "User Management"
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserRead, Description: "Allows listing and reading any user's profile.", Category: category})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserManage, Description: "Allows updating/managing any user's profile and status.", Category: category})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserProfileReadSelf, Description: "Allows a user to read their own profile.", Category: category})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserProfileUpdateSelf, Description: "Allows a user to update their own profile.", Category: category})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserConnectionManage, Description: "Allows a user to follow/block other users.", Category: category})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: user.PermUserAnonymize, Description: "Allows triggering the anonymization job for any user.", Category: category}) // Added this missing permission registration

	deps.Registry.RegisterService(BuildManifest())

	repo := postgres.NewUserRepository(deps.DB, deps.Logger)

	// --- Service Instantiation ---
	service := NewService(
		repo,
		deps.StorageSvc,
		deps.AuditRecorder,
		deps.RBACService,
		deps.Dispatcher,
		deps.Logger,
	)
	adminService := NewAdminService(
		repo,
		deps.AuditRecorder,
		deps.AuthAdminSvc,
		deps.RBACService,
		deps.StorageSvc,
		deps.Logger,
	)

	workers := []queue.JobProcessor{
		NewCleanupWorker(repo, deps.Logger),
		NewFileCleanupWorker(deps.StorageSvc, deps.Logger),
	}

	return &Container{
		Service:      service,
		AdminService: adminService,
		Workers:      workers,
	}, nil
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "user-service",
		DisplayName: "User Profiles",
		Description: "Manages user identity details, profiles, and social connections.",
		Icon:        "users",
		Events: []core.EventDefinition{
			{
				Key:         core.EventUserCreated,
				Label:       "User Created",
				Description: "A new user has registered and their profile is initialized.",
				Schema:      map[string]string{"user_id": "uuid", "email": "string"},
			},
			{
				Key:         core.EventUserProfileUpdated,
				Label:       "Profile Updated",
				Description: "A user changed their name, bio, or preferences.",
				Schema:      map[string]string{"user_id": "uuid"},
			},
			{
				Key:         core.EventUserStatusUpdated,
				Label:       "Status Changed",
				Description: "An admin changed a user's status (e.g. active -> suspended).",
				Schema:      map[string]string{"user_id": "uuid", "old_status": "string", "new_status": "string"},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: user.PermUserRead, Label: "Read Any Profile", Category: "User Management"},
			{Key: user.PermUserManage, Label: "Manage Any User", Category: "User Management"},
			{Key: user.PermUserAnonymize, Label: "Anonymize User", Category: "User Management", Dangerous: true},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         user.JobCleanupPendingUsers,
				Label:       "Cleanup Pending Users",
				Description: "Removes unverified accounts older than 7 days.",
			},
		},
	}
}
