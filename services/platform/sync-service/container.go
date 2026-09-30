package sync

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service sync_domain.Service
}

type Dependencies struct {
	DB        *sqlx.DB // <--- Added DB dependency
	Logger    log.Logger
	Audit     audit.EventRecorder
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
}

func NewContainer(deps Dependencies) *Container {
	// 1. Register Manifest
	deps.Registry.RegisterService(BuildManifest())

	// 2. Initialize Repository
	repo := postgres.NewSyncRepository(deps.DB, deps.Logger)

	// 3. Initialize Service (injecting repo)
	svc := NewService(repo, deps.Logger, deps.Audit)

	return &Container{
		Service: svc,
	}
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "sync-service",
		DisplayName: "Offline Synchronization",
		Description: "Coordinates data sync for PWA/Mobile clients with conflict resolution and audit logs.",
		Icon:        "refresh-cw",
		// The sync service itself doesn't enforce permissions (the domains do),
		// but we might add admin permissions later for viewing sync logs.
		Permissions: []core.PermDefinition{},
		Events:      []core.EventDefinition{
			// Example events if the sync service emits them directly
			// {Key: "sync.conflict_detected", Label: "Sync Conflict"},
		},
	}
}
