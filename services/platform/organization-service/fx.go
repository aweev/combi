// organization-service — multi-tenant organization management as an fx
// library module.
//
// Ported onto the combi chassis: persistence lives in
// pkg/persistence/postgres (OrganizationRepository), domain types in
// pkg/domain/organization. This module has no main package; a host binary
// (cmd/platform-server) mounts it via Module below.
package organization

import (
	"context"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the organization service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, log.Logger + security.RandomService (platform utils, used to
// build the default TokenGenerator when none is mounted).
//
// Optional dependencies — user.Service, notification.Service,
// queue.Dispatcher, audit.EventRecorder, security.TokenGenerator — arrive
// nil (dig `optional:"true"`) until their modules are mounted; the service
// layer stores them for request-time use. Registrar/registry fall back to
// discards so the module also validates standalone.
var Module = fx.Module(
	"organization",
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
	Random security.RandomService

	UserService     user.Service            `optional:"true"`
	NotificationSvc notification.Service    `optional:"true"`
	Dispatcher      queue.Dispatcher        `optional:"true"`
	AuditRecorder   audit.EventRecorder     `optional:"true"`
	TokenGen        security.TokenGenerator `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service      organization.Service
	RoleAdminSvc organization.RoleAdminService
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
	tokenGen := d.TokenGen
	if tokenGen == nil {
		// No TokenGenerator provider exists yet; build the default
		// HMAC-backed generator from the host RandomService.
		tokenGen = security.NewTokenGenerator(d.Random)
	}
	// NOTE: UserService/NotificationSvc/Dispatcher/AuditRecorder arrive nil
	// (dig `optional:"true"`) until their modules are mounted; the service
	// layer stores them for request-time use (nil-safe stopgap convention).
	c, err := NewContainer(Dependencies{
		DB:              d.DB,
		Logger:          d.Logger,
		Registrar:       registrar,
		Registry:        registry,
		UserService:     d.UserService,
		NotificationSvc: d.NotificationSvc,
		Dispatcher:      d.Dispatcher,
		AuditRecorder:   d.AuditRecorder,
		TokenGen:        tokenGen,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:    c,
		Service:      c.Service,
		RoleAdminSvc: c.RoleAdminSvc,
	}, nil
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Logger    log.Logger
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("organization", func(ctx context.Context) error {
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
