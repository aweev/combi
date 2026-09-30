// identity-service — user profiles (identity) as an fx library module.
//
// Ports api/internal/services/user (Phase 3, Tier 3; merged into identity per
// plan.md) onto the combi chassis: persistence lives in
// pkg/persistence/postgres (UserRepository), domain types in pkg/domain/user.
// The service stays in its own subpackage (user/) — merged only at the fx
// wiring layer here. This module has no main package; a host binary
// (cmd/platform-server) mounts it via Module below.
package identity

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	usersvc "github.com/combi/identity/user"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// Module is the fx module for the identity (user profiles) service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger.
//
// Optional dependencies — audit recorder, storage service, dispatcher,
// event bus, RBAC authorization service — arrive nil (dig `optional:"true"`)
// until their modules are mounted. The auth admin service is NOT an fx edge
// (it would cycle auth↔user): it stays nil at construction and the host
// late-binds it via SetAuthAdminService (see platform-server bindIdentityAuth).
// Registrar/registry fall back to discards so the module also validates
// standalone.
var Module = fx.Module(
	"identity",
	fx.Provide(
		provideServices,
	),
	fx.Invoke(
		RegisterHooks,
	),
)

type moduleDeps struct {
	fx.In

	DB     *sqlx.DB
	Logger log.Logger

	Audit      audit.EventRecorder       `optional:"true"`
	StorageSvc storage.Service          `optional:"true"`
	Dispatcher queue.Dispatcher         `optional:"true"`
	Bus        events.Bus                `optional:"true"`
	RBAC       rbac.AuthorizationService `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	UserContainer *usersvc.Container

	UserService user.Service
	UserAdmin   user.AdminService
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registrar := d.Registrar
	if registrar == nil {
		registrar = discardRegistrar{}
	}
	registry := d.Registry
	if registry == nil {
		registry = discardRegistry{}
	}
	c, err := usersvc.NewContainer(usersvc.Dependencies{
		DB:            d.DB,
		Logger:        d.Logger,
		AuditRecorder: d.Audit,
		StorageSvc:    d.StorageSvc,
		AuthAdminSvc:  nil, // late-bound by the host (bindIdentityAuth)
		RBACService:   d.RBAC,
		Registrar:     registrar,
		Registry:      registry,
		Dispatcher:    d.Dispatcher,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		UserContainer: c,
		UserService:   c.Service,
		UserAdmin:     c.AdminService,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC      fx.Lifecycle
	Checker *health.Checker
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("identity", func(ctx context.Context) error {
		return nil
	})

	d.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
}

type discardRegistrar struct{}

func (discardRegistrar) Register(rbac.PermissionDefinition) {}
func (discardRegistrar) SyncWithDataStore(context.Context) error { return nil }
func (discardRegistrar) GetDefinitions() []rbac.PermissionDefinition { return nil }

type discardRegistry struct{}

func (discardRegistry) RegisterService(core.ServiceManifest) {}
func (discardRegistry) GetSystemCapabilities(context.Context) ([]core.ServiceManifest, error) {
	return nil, nil
}
