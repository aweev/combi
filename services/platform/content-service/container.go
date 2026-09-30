package content

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/content/workers"
)


type Container struct {
	Service      content.Service
	AdminService content.AdminService
	Workers      []queue.JobProcessor
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	EventBus      events.Bus
	Dispatcher    queue.Dispatcher
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Manifest & Permissions
	deps.Registry.RegisterService(BuildManifest())
	registerPermissions(deps.Registrar)

	// 2. Repo
	repo := postgres.NewContentRepository(deps.DB, deps.Logger)

	// 3. Helpers
	validator := NewValidator() // Simple schema validator

	// 4. Services
	service := NewService(repo, deps.AuditRecorder, deps.EventBus, deps.Dispatcher, validator, deps.Logger)
	adminService := NewAdminService(repo, deps.AuditRecorder, deps.Logger)

	// 5. Workers
	workers := []queue.JobProcessor{
		workers.NewReindexWorker(repo, deps.EventBus, deps.Logger),
	}

	return &Container{
		Service:      service,
		AdminService: adminService,
		Workers:      workers,
	}, nil
}

func registerPermissions(r rbac.PermissionRegistrar) {
	const cat = "Content (CMS)"
	r.Register(rbac.PermissionDefinition{Name: content.PermContentRead, Description: "Read content entries", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: content.PermContentEdit, Description: "Create/Edit content entries", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: content.PermContentPublish, Description: "Publish/Unpublish entries", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: content.PermContentAdmin, Description: "Manage Spaces and Models", Category: cat})
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "content-service",
		DisplayName: "Content Management",
		Description: "Headless CMS for managing dynamic text, assets, and layouts.",
		Icon:        "file-text",
		Events: []core.EventDefinition{
			{Key: core.EventSpaceCreated, Label: "Space Created"},
			{Key: core.EventModelCreated, Label: "Content Model Created"},
			{Key: core.EventContentCreated, Label: "Entry Created"},
			{Key: core.EventContentPublished, Label: "Entry Published"},
			{Key: core.EventContentUnpublished, Label: "Entry Unpublished"},
			{Key: core.EventContentDeleted, Label: "Entry Deleted"},
		},
		Permissions: []core.PermDefinition{
			{Key: content.PermContentRead, Label: "View Content", Category: "Content"},
			{Key: content.PermContentEdit, Label: "Edit Content", Category: "Content"},
			{Key: content.PermContentPublish, Label: "Publish Content", Category: "Content"},
			{Key: content.PermContentAdmin, Label: "Administer Content", Category: "Content"},
		},
		Jobs: []core.JobDefinition{
			{Key: content.JobContentReindexEntry, Label: "Reindex Content", Description: "Syncs content changes to search index."},
		},
	}
}
