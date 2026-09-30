// storage-service — storage service as an fx library module.
//
// Ported from api/internal/services/storage (Phase 3, Tier 2) onto the combi
// chassis: persistence lives in pkg/persistence/postgres (StorageRepository),
// domain types in pkg/domain/storage. This module has no main package; a host
// binary (cmd/platform-server) mounts it via Module below.
package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
)

// Module is the fx module for the storage service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, plus audit.EventRecorder (auditlog
// module) and rbac.AuthorizationService (acl module) — REQUIRED, not
// optional: the storage container hard-rejects nil for both, so failing
// loudly at graph validation beats a construction panic.
//
// queue.Dispatcher is optional with a discard fallback: the queue service
// was replaced by Temporal per plan.md and nothing provides a dispatcher
// yet. Dispatch is used only for background jobs (file delete, cleanup,
// archive, processing); the discard reports an honest error on those paths
// while CRUD stays fully functional. Remove it when the queue lands.
//
// Registrar/registry fall back to discards so the module also validates
// standalone (construction then fails with a clear error naming the
// missing mount, instead of an fx graph error).
var Module = fx.Module(
	"storage",
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
	Config *config.Config
	Logger log.Logger

	Audit      audit.EventRecorder       `optional:"true"`
	Dispatcher queue.Dispatcher          `optional:"true"`
	Bus        events.Bus                `optional:"true"`
	RBAC       rbac.AuthorizationService `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service       storage.Service
	AdminService  storage.AdminService
	WorkerService storage.WorkerService
}

func provideServices(d moduleDeps) (servicesOut, error) {
	if d.Audit == nil || d.RBAC == nil {
		return servicesOut{}, fmt.Errorf("storage: requires audit recorder and RBAC service mounted in the host (the container rejects nil dependencies)")
	}
	dispatcher := d.Dispatcher
	if dispatcher == nil {
		dispatcher = fxnoop.NoopDispatcher{}
	}
	registrar := d.Registrar
	if registrar == nil {
		registrar = fxnoop.NoopRegistrar{}
	}
	registry := d.Registry
	if registry == nil {
		registry = fxnoop.NoopRegistry{}
	}
	repo := pPostgres.NewStorageRepository(d.DB, d.Logger)
	providerFct := NewProviderFactory(d.Config, repo, d.Logger)
	processorFct := NewProcessorFactory(d.Logger, dispatcher)
	c, err := NewContainer(d.Config, Dependencies{
		Repo:          repo,
		ProviderFct:   providerFct,
		ProcessorFct:  processorFct,
		Dispatcher:    dispatcher,
		AuditRecorder: d.Audit,
		RBACService:   d.RBAC,
		Registrar:     registrar,
		Registry:      registry,
		Logger:        d.Logger,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:     c,
		Service:       c.Service,
		AdminService:  c.AdminService,
		WorkerService: c.WorkerService,
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
	d.Checker.Register("storage", func(ctx context.Context) error {
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
