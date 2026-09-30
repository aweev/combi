// access-control-service (package acl) — ported from
// api/internal/services/rbac/container.go.
//
// Container assembles the RBAC service components: repository-backed
// admin/authorization services, permission registrar, and the cache
// invalidation worker. It exists for direct (non-fx) construction; the fx
// module wiring in fx.go provides the same graph via DI.
package acl

import (
	"fmt"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
	"github.com/jmoiron/sqlx"
)

// Container is the assembled RBAC service set, for direct construction.
type Container struct {
	AuthorizationService rbac.AuthorizationService
	AdminService         rbac.AdminService
	PermissionRegistrar  rbac.PermissionRegistrar
	Workers              []queue.JobProcessor
}

// Dependencies are the external components the RBAC service needs.
type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder // Optional (nil-safe in AdminService)
	Cache         cache.Service
	Dispatcher    queue.Dispatcher // Optional (nil-safe in AdminService)
}

// NewContainer builds the RBAC service.
func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Persistence
	repo := pPostgres.NewRBACRepository(deps.DB, deps.Logger)

	// 2. Services
	// We create the concrete implementation of AdminService first so we can
	// configure it later if dependencies are missing.
	adminSvc := NewAdminService(
		repo,
		deps.Cache,
		deps.Dispatcher,
		deps.AuditRecorder,
		deps.Logger,
	)

	authzSvc := NewAuthorizationService(repo, deps.Cache, deps.Logger)

	registrar := NewPermissionRegistrar(repo, deps.Logger)

	// 3. Workers
	// The worker needs the AdminService to clear caches.
	// This is why we need to build AdminService before Workers.
	cacheWorker := NewCacheInvalidationWorker(repo, adminSvc, deps.Logger)

	// 4. Register Self-Permissions
	// The RBAC service needs to register permissions to control itself.
	registerRbacPermissions(registrar)

	return &Container{
		AuthorizationService: authzSvc,
		AdminService:         adminSvc,
		PermissionRegistrar:  registrar,
		Workers:              []queue.JobProcessor{cacheWorker},
	}, nil
}

// InjectAuditRecorder allows late-binding of the audit recorder.
func (c *Container) InjectAuditRecorder(rec audit.EventRecorder) error {
	if admin, ok := c.AdminService.(*AdminService); ok {
		admin.SetAuditRecorder(rec)
		return nil
	}
	return fmt.Errorf("AdminService is not the expected concrete type")
}

// InjectDispatcher allows late-binding of the queue dispatcher.
func (c *Container) InjectDispatcher(d queue.Dispatcher) error {
	if admin, ok := c.AdminService.(*AdminService); ok {
		admin.SetDispatcher(d)
		return nil
	}
	return fmt.Errorf("AdminService is not the expected concrete type")
}

// registerRbacPermissions registers the RBAC service's own permissions —
// the service needs permissions to control itself.
func registerRbacPermissions(r rbac.PermissionRegistrar) {
	const cat = "System Administration"

	r.Register(rbac.PermissionDefinition{
		Name:        rbac.PermRBACRoleRead,
		Description: "View roles and permissions configuration.",
		Category:    cat,
	})
	r.Register(rbac.PermissionDefinition{
		Name:        rbac.PermRBACRoleManage,
		Description: "Create, update, and delete roles and permissions.",
		Category:    cat,
	})
	r.Register(rbac.PermissionDefinition{
		Name:        rbac.PermRBACUserRoleAssign,
		Description: "Assign and revoke roles for users.",
		Category:    cat,
	})
}

// BuildManifest returns the service manifest describing the RBAC service's
// public capabilities (consumed by the configuration service's registry).
func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "rbac-service",
		DisplayName: "Access Control",
		Description: "Manages Roles, Permissions, and User Assignments.",
		Icon:        "shield",
		Events: []core.EventDefinition{
			{Key: rbac.EventRoleCreated, Label: "Role Created", Description: "New role added"},
			{Key: rbac.EventUserRoleAssigned, Label: "Role Assigned", Description: "User granted new privileges"},
		},
		Permissions: nil, // Registered dynamically
		Jobs: []core.JobDefinition{
			{
				Key:         rbac.JobRBACInvalidateUserCache,
				Label:       "Cache Invalidation",
				Description: "Clears permission caches when roles change.",
			},
		},
	}
}
