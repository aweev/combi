// audit-log-service — audit trail service as an fx library module.
//
// Ported from api/internal/services/audit (service layer) onto the combi
// chassis: persistence lives in pkg/persistence/postgres (AuditRepository),
// domain types in pkg/domain/audit. This module has no main package; a host
// binary (cmd/platform-server) mounts it via Module below.
package auditlog

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the audit-log service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *config.Config (via fx.Supply), *sqlx.DB, log.Logger, and *health.Checker.
//
// Optional dependencies — queue.Dispatcher
// and rbac.PermissionRegistrar —
// are NOT required: they arrive nil from the graph (dig `optional:"true"`)
// until their service modules are ported and mounted. The service layer is
// nil-safe without them (see errors.go for the honest 503s); late-bind via
// the Set*/Inject* methods on the concrete types once they land.
// notification.Service is late-bound by the host via
// ExportWorker.SetNotificationService (constructor-wiring it would cycle
// with the notification module, which consumes audit.EventRecorder).
// privacy.Manager is late-bound by the host via
// ComplianceService.SetPrivacyManager (same cycle shape with the
// consent-privacy module).
var Module = fx.Module(
	"auditlog",

	fx.Provide(
		// Persistence — the module owns its repository (and the narrow
		// EventRepository view the workers consume, emitted by
		// provideServices below).
		NewRepository,

		// Module-owned infrastructure adapters: the audit encryption
		// service (AES-GCM over cfg.Auth.AuditEncryptionKey) and the
		// direct queue enqueuer used by the async recorder.
		ProvideCryptoService,
		NewQueueEnqueuer,

		// Services — see provideServices for the multi-type wiring.
		provideServices,

		// Job processors (run by a queue/worker host when mounted).
		NewStoreWorker,
		provideExportWorker,
		provideComplianceWorker,
	),

	fx.Invoke(
		RegisterHooks,
	),
)

// NewRepository provides the audit repository from the shared Postgres pool.
func NewRepository(db *sqlx.DB) audit.Repository {
	return pPostgres.NewAuditRepository(db)
}

// ProvideCryptoService builds the AES-GCM crypto service used to
// encrypt/decrypt sensitive audit payloads at rest. The key comes from
// auth.auditEncryptionKey (combi/config.yml; NGO_AUTH_AUDITENCRYPTIONKEY).
func ProvideCryptoService(cfg *config.Config, logger log.Logger) (security.CryptoService, error) {
	key := []byte(cfg.Auth.AuditEncryptionKey)
	if len(key) == 0 {
		return nil, errors.New("auth.auditEncryptionKey is not set: audit events cannot be encrypted at rest without it")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("auth.auditEncryptionKey must be 16, 24, or 32 bytes (AES-128/192/256)")
	}

	cryptoSvc, err := security.NewAesGcmCryptoService(key, security.NewRandomService())
	if err != nil {
		return nil, err
	}
	logger.Debug("audit crypto service initialized",
		log.Int("key_bytes", len(key)))
	return cryptoSvc, nil
}

// NewQueueEnqueuer provides the direct job enqueuer (no dispatcher
// overhead) used by the async event recorder to persist audit events.
func NewQueueEnqueuer(db *sqlx.DB, logger log.Logger) queue.JobEnqueuer {
	return pPostgres.NewQueueRepository(db, logger)
}

// serviceDeps gathers everything the audit service graph needs, including
// the optional platform dependencies that arrive nil until their modules
// are mounted in the host.
type serviceDeps struct {
	fx.In

	DB     *sqlx.DB
	Logger log.Logger

	// Module-provided (non-optional).
	Repo     audit.Repository
	Crypto   security.CryptoService
	Enqueuer queue.JobEnqueuer

	// Optional platform dependencies (nil until mounted).
	// NOTE: notification.Service is deliberately NOT an fx dependency here.
	// notification in turn (optionally) consumes audit.EventRecorder, so
	// constructor-wiring both directions creates a dependency cycle once
	// both modules are mounted. The export worker is built with a nil
	// notifier and the host late-binds the real service via
	// ExportWorker.SetNotificationService (mirrors api's "Injected later"
	// wiring in build_services.go).
	// NOTE: privacy.Manager is likewise NOT an fx dependency. The
	// consent-privacy module consumes audit.EventRecorder, so the manager
	// is late-bound by the host via ComplianceService.SetPrivacyManager
	// (mirrors api's InjectPrivacyManager wiring).
	// NOTE: storage.Service is likewise NOT an fx dependency. The storage
	// module hard-requires audit.EventRecorder at construction, so the
	// export worker is built with nil storage and the host late-binds it
	// via ExportWorker.SetStorageService (mirrors api's
	// InjectStorageService wiring).
	Dispatcher queue.Dispatcher `optional:"true"`
}

// servicesOut emits each service both as its concrete type (for late
// binding and workers) and as its domain interface (for in-process
// callers). dig does not auto-narrow interfaces, so every consumer type
// must be emitted explicitly.
type servicesOut struct {
	fx.Out

	Recorder          *AsyncEventRecorder
	Service           *Service
	ComplianceService *ComplianceService

	AuditEventRecorder     audit.EventRecorder
	AuditService           audit.Service
	AuditComplianceService audit.ComplianceService
	AuditEventRepository   audit.EventRepository
	// Grouped (not bare): every service that feeds the future
	// reporting-service aggregator contributes its DataFetcher to the
	// "reportFetchers" group. A bare provide would collide the moment a
	// second feeder (e.g. payments) is mounted in the same host.
	ReportingDataFetcher reporting.DataFetcher `group:"reportFetchers"`
}

// provideServices builds the audit service graph: the async recorder over
// the crypto service and queue enqueuer, plus the audit and compliance
// services over the repository. The compliance service starts with a nil
// privacy manager (late-bound by the host via SetPrivacyManager).
func provideServices(deps serviceDeps) servicesOut {
	recorder := NewAsyncEventRecorder(deps.Crypto, deps.Enqueuer, deps.Logger)
	service := NewService(deps.Repo, deps.Crypto, deps.Dispatcher, deps.Logger)
	compliance := NewComplianceService(deps.Repo, deps.Repo, deps.Dispatcher, recorder, nil, deps.Logger)

	return servicesOut{
		Recorder:               recorder,
		Service:                service,
		ComplianceService:      compliance,
		AuditEventRecorder:     recorder,
		AuditService:           service,
		AuditComplianceService: compliance,
		AuditEventRepository:   deps.Repo,
		ReportingDataFetcher:   service,
	}
}

// provideExportWorker adapts NewExportWorker to the fx graph. Storage and
// notification are late-bound by the host via SetStorageService /
// SetNotificationService (wiring either here would cycle with the storage /
// notification modules, which consume audit.EventRecorder). The worker is
// nil-safe without them (fails exports non-retryably with a clear message).
func provideExportWorker(deps serviceDeps) *ExportWorker {
	return NewExportWorker(deps.Repo, deps.Crypto, nil, nil, deps.Logger)
}

// provideComplianceWorker adapts NewComplianceWorker to the fx graph. The
// privacy manager is late-bound by the host via SetPrivacyManager (wiring
// it here would cycle with the consent-privacy module, which consumes
// audit.EventRecorder); the compliance service is nil-safe without it.
func provideComplianceWorker(deps serviceDeps) *ComplianceWorker {
	return NewComplianceWorker(deps.Logger, nil)
}

// hooksDeps gathers the lifecycle wiring inputs; the permission registrar
// is optional so the module also validates standalone.
type hooksDeps struct {
	fx.In

	LC      fx.Lifecycle
	Checker *health.Checker
	Logger  log.Logger

	Registrar rbac.PermissionRegistrar `optional:"true"`
}

// RegisterHooks binds the audit service lifecycle to the fx app: health
// probe and startup permission registration.
func RegisterHooks(deps hooksDeps) {
	deps.Checker.Register("audit", func(ctx context.Context) error {
		// Liveness-only probe: deep readiness (Postgres) is covered by
		// the host's database health check bridged into grpc_health_v1.
		return nil
	})

	deps.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Register the audit service's own permissions so the
			// service can control itself. When a registrar is present
			// it also reconciles them with the database (acl's
			// SyncWithDataStore covers the whole catalog, including
			// these).
			if deps.Registrar != nil {
				registerPermissions(deps.Registrar)
			} else {
				deps.Logger.Debug("permission registrar not mounted; skipping audit permission registration")
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
}
