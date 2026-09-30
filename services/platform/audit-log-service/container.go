// audit-log-service (package auditlog) — ported from
// api/internal/services/audit/container.go.
//
// Container assembles the audit service components: the async event
// recorder, audit/compliance services, and the three job workers. It exists
// for direct (non-fx) construction; the fx module wiring in fx.go provides
// the same graph via DI.
package auditlog

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
	pPostgres "github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/security"
)

// Container holds the public API of the Audit service.
type Container struct {
	Recorder          audit.EventRecorder
	Service           audit.Service
	ComplianceService audit.ComplianceService
	Workers           []queue.JobProcessor // Group workers for easy registration
}

// Dependencies holds the raw infrastructure required to boot Audit.
type Dependencies struct {
	DB             *sqlx.DB
	Logger         log.Logger
	CryptoSvc      security.CryptoService
	Dispatcher     queue.Dispatcher         // Optional (nil-safe, see errors.go)
	Registrar      rbac.PermissionRegistrar // Optional (skipped when nil)
	Registry       core.Registry            // Optional (skipped when nil)
	StorageSvc     storage.Service          // Optional (nil-safe in ExportWorker)
	NotifSvc       notification.Service     // Optional (nil-safe in ExportWorker)
	QueueEnqueuer  queue.JobEnqueuer        // Optional (SetEnqueuer late-bind)
	PrivacyManager privacy.Manager          // Optional (nil-safe, see errors.go)
}

// NewContainer constructs the specific repository implementation and wires
// the service.
func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions and manifest (skipped when the platform
	// pieces are not provided; the fx path registers them OnStart).
	if deps.Registrar != nil {
		registerPermissions(deps.Registrar)
	}
	if deps.Registry != nil {
		deps.Registry.RegisterService(BuildManifest())
	}

	// 2. Initialize Repository (Postgres implementation)
	repo := pPostgres.NewAuditRepository(deps.DB)

	// 3. Build Service Logic
	// Async Recorder needs a way to enqueue without the full Dispatcher overhead
	recorder := NewAsyncEventRecorder(deps.CryptoSvc, deps.QueueEnqueuer, deps.Logger)

	service := NewService(repo, deps.CryptoSvc, deps.Dispatcher, deps.Logger)

	complianceService := NewComplianceService(repo, repo, deps.Dispatcher, recorder, deps.PrivacyManager, deps.Logger)

	// 4. Build Workers
	workers := []queue.JobProcessor{
		NewStoreWorker(repo, deps.Logger),
		NewExportWorker(repo, deps.CryptoSvc, deps.StorageSvc, deps.NotifSvc, deps.Logger),
		NewComplianceWorker(deps.Logger, deps.PrivacyManager),
	}

	return &Container{
		Recorder:          recorder,
		Service:           service,
		ComplianceService: complianceService,
		Workers:           workers,
	}, nil
}

// registerPermissions registers the audit service's own permissions.
func registerPermissions(registrar rbac.PermissionRegistrar) {
	const category = "Audit & Compliance"

	// We use domain constants to ensure type safety
	registrar.Register(rbac.PermissionDefinition{
		Name:        audit.PermAuditRead,
		Description: "Can view the audit trail for all system events.",
		Category:    category,
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        audit.PermAuditExport,
		Description: "Can request and download exports of the audit trail.",
		Category:    category,
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        audit.PermComplianceManage,
		Description: "Can manage legal holds and data retention policies.",
		Category:    category,
	})
}

// SetDispatcher late-binds the queue dispatcher into both services that
// need it (audit Service.ExportEvents, ComplianceService.AnonymizeUser).
func (c *Container) SetDispatcher(d queue.Dispatcher) {
	// Type assertion to access the setter on the concrete implementation
	if s, ok := c.Service.(*Service); ok {
		s.SetDispatcher(d)
	}
	if s, ok := c.ComplianceService.(*ComplianceService); ok {
		s.SetDispatcher(d)
	}
}

// InjectPrivacyManager allows late-binding of the privacy manager.
func (c *Container) InjectPrivacyManager(pm privacy.Manager) error {
	if svc, ok := c.ComplianceService.(*ComplianceService); ok {
		svc.privacyManager = pm
		return nil
	}
	return fmt.Errorf("compliance service is not the expected concrete type")
}

// InjectNotificationService allows late-binding of the notification service
// (consumed by the export worker).
func (c *Container) InjectNotificationService(ns notification.Service) error {
	if w, ok := c.findExportWorker(); ok {
		w.notifier = ns
		return nil
	}
	return fmt.Errorf("export worker is not present in the container")
}

// InjectStorageService allows late-binding of the storage service.
func (c *Container) InjectStorageService(ss storage.Service) error {
	if w, ok := c.findExportWorker(); ok {
		w.storageSvc = ss
		return nil
	}
	return fmt.Errorf("export worker is not present in the container")
}

// findExportWorker returns the container's ExportWorker, if present.
func (c *Container) findExportWorker() (*ExportWorker, bool) {
	for _, w := range c.Workers {
		if ew, ok := w.(*ExportWorker); ok {
			return ew, true
		}
	}
	return nil, false
}
