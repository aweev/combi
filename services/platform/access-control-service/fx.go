// access-control-service — RBAC/ACL service as an fx library module.
//
// Ported from api/internal/services/rbac (service layer) onto the combi
// chassis: persistence lives in pkg/persistence/postgres (RBACRepository),
// domain types in pkg/domain/rbac. This module has no main package; a host
// binary (cmd/platform-server) mounts it via Module below.
package acl

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
)

// Module is the fx module for the access-control (RBAC) service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB (database), cache.Service (platform utils), log.Logger.
//
// Optional dependencies — audit.EventRecorder and queue.Dispatcher — are
// NOT required: the AdminService is nil-safe without them. Resolve the
// concrete *acl.AdminService and call SetAuditRecorder/SetDispatcher on it
// (or mount the audit/queue service modules in the same host) to activate
// auditing and async cache invalidation.
var Module = fx.Module(
	"acl",

	fx.Provide(
		// Persistence
		NewRepository,

		// Services — see provideServices for the multi-type wiring.
		provideServices,

		// Job processors (run by a queue/worker host when mounted)
		NewCacheInvalidationWorker,
	),

	fx.Invoke(
		RegisterHooks,
	),
)

// NewRepository provides the RBAC repository from the shared Postgres pool.
func NewRepository(db *sqlx.DB, logger log.Logger) rbac.Repository {
	return pPostgres.NewRBACRepository(db, logger)
}

// servicesOut emits each service both as its concrete type (for the
// worker and late binding) and as its domain interface (for in-process
// callers). dig does not auto-narrow interfaces — a rbac.Repository in
// the graph does not satisfy a rbac.PermissionRepository parameter — so
// every consumer type must be emitted explicitly.
type servicesOut struct {
	fx.Out

	AuthorizationService *AuthorizationService
	AdminService         *AdminService
	PermissionRegistrar  *PermissionRegistrar

	RBACAuthorizationService rbac.AuthorizationService
	RBACAdminService         rbac.AdminService
	RBACPermissionRegistrar  rbac.PermissionRegistrar
	RBACPermissionRepository rbac.PermissionRepository
}

// provideServices builds the service graph: authorization + admin
// services over the repository and shared cache, plus the permission
// registrar (which takes the narrow PermissionRepository port).
func provideServices(repo rbac.Repository, cacheSvc cache.Service, logger log.Logger) servicesOut {
	authz := NewAuthorizationService(repo, cacheSvc, logger)
	admin := NewAdminService(repo, cacheSvc, nil, nil, logger)
	registrar := NewPermissionRegistrar(repo, logger)

	return servicesOut{
		AuthorizationService:     authz,
		AdminService:             admin,
		PermissionRegistrar:      registrar,
		RBACAuthorizationService: authz,
		RBACAdminService:         admin,
		RBACPermissionRegistrar:  registrar,
		RBACPermissionRepository: repo,
	}
}

// RegisterHooks binds the RBAC service lifecycle to the fx app: health
// probe, self-permission registration, and startup permission sync.
func RegisterHooks(lc fx.Lifecycle, hc *health.Checker, logger log.Logger,
	registrar rbac.PermissionRegistrar,
) {
	hc.Register("rbac", func(ctx context.Context) error {
		// Liveness-only probe: deep readiness (Postgres) is covered by the
		// host's database health check bridged into grpc_health_v1.
		return nil
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Register the RBAC service's own permissions so the service
			// can control itself, then reconcile the code-defined
			// permission catalog with the database.
			registerRbacPermissions(registrar)

			if err := registrar.SyncWithDataStore(ctx); err != nil {
				// Best-effort: in dev the database may be empty or
				// unreachable; permissions sync again on next boot.
				logger.Warn("RBAC permission sync failed; permissions will re-sync on next start", log.ErrorAttr(err))
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
}
