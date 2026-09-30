package catalog

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service      catalog.Service
	AdminService catalog.AdminService
	Workers      []queue.JobProcessor
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
	AuditRecorder audit.EventRecorder
	EventBus      events.Bus

	// Domain Services
	UserService user.Service
	RBACService rbac.AuthorizationService
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Manifest
	deps.Registry.RegisterService(BuildManifest())
	for _, p := range BuildManifest().Permissions {
		deps.Registrar.Register(rbac.PermissionDefinition{
			Name: p.Key, Description: p.Description, Category: p.Category,
		})
	}

	// 2. Infra
	repo := postgres.NewCatalogRepository(deps.DB, deps.Logger)

	// 3. Services
	adminSvc := NewAdminService(
		repo,
		deps.UserService,
		deps.RBACService,
		deps.AuditRecorder,
		deps.EventBus,
		deps.Logger,
	)

	publicSvc := NewService(
		repo,
		deps.UserService,
		deps.Logger,
		deps.AuditRecorder,
		deps.EventBus,
	)

	// 4. Workers
	// Now compiles because ProcessJob is implemented
	workers := []queue.JobProcessor{
		NewReindexWorker(repo, deps.EventBus, deps.Logger),
		NewStatusUpdateWorker(repo, deps.AuditRecorder, deps.EventBus, deps.Logger),
	}

	return &Container{
		Service:      publicSvc,
		AdminService: adminSvc,
		Workers:      workers,
	}, nil
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "catalog-service",
		DisplayName: "Catalog & Applications",
		Description: "Manages programs, volunteer opportunities, and user applications.",
		Icon:        "shopping-bag",

		Events: []core.EventDefinition{
			{
				Key:         core.EventItemCreated,
				Label:       "Item Created",
				Description: "A new catalog item was created.",
				Schema:      map[string]string{"id": "uuid", "type": "string", "name": "string"},
			},
			{
				Key:         core.EventItemUpdated,
				Label:       "Item Updated",
				Description: "Catalog item details were modified.",
				Schema:      map[string]string{"id": "uuid"},
			},
			{
				Key:         core.EventItemDeleted,
				Label:       "Item Deleted",
				Description: "Catalog item was removed.",
				Schema:      map[string]string{"id": "uuid"},
			},
			{
				Key:         core.EventApplicationSubmitted,
				Label:       "Application Submitted",
				Description: "User submitted a new application.",
				Schema:      map[string]string{"id": "uuid", "user_id": "uuid", "item_id": "uuid"},
			},
			{
				Key:         core.EventApplicationStatusUpdated,
				Label:       "Application Status Changed",
				Description: "Application status updated (e.g. Approved/Rejected).",
				Schema:      map[string]string{"id": "uuid", "status": "string"},
			},
		},

		Permissions: []core.PermDefinition{
			{Key: catalog.PermCatalogManage, Label: "Manage Items", Category: "Catalog"},
			{Key: catalog.PermCatalogCategoryManage, Label: "Manage Categories", Category: "Catalog"},
			{Key: catalog.PermCatalogApplicationRead, Label: "View Applications", Category: "Catalog"},
			{Key: catalog.PermCatalogApplicationWrite, Label: "Manage Applications", Category: "Catalog"},
		},

		Jobs: []core.JobDefinition{
			{
				Key:         catalog.JobCatalogReindex,
				Label:       "Reindex Catalog",
				Description: "Broadcasts update events for all items to refresh downstream services.",
			},
			{
				Key:         "catalog:update_status",
				Label:       "Update Statuses",
				Description: "Time-based status transitions for items.",
			},
		},
	}
}
