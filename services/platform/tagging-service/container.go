package tagging

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/tagging"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service tagging.Service
	// If you split Admin/Public services, list them both here.
	// Since Service implements both, we just expose one.
	AdminService tagging.AdminService
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	Audit     audit.EventRecorder
	EventBus  events.Bus
	Authz     rbac.AuthorizationService
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions
	deps.Registrar.Register(rbac.PermissionDefinition{
		Name:        tagging.PermTagManage,
		Description: "Create, update, and merge system tags.",
		Category:    "Taxonomy",
	})

	// 2. Register Manifest
	deps.Registry.RegisterService(core.ServiceManifest{
		ServiceName: "tagging-service",
		DisplayName: "Taxonomy",
		Description: "Manages tags, skills, and interests.",
		Icon:        "tag",
		Events: []core.EventDefinition{
			{Key: core.EventEntityTagged, Label: "Entity Tagged", Description: "An entity (user/project) was tagged."},
			{Key: core.EventTagsMerged, Label: "Tags Merged", Description: "Duplicate tags were merged."},
		},
		Permissions: []core.PermDefinition{
			{Key: tagging.PermTagManage, Label: "Manage Tags", Category: "Taxonomy"},
		},
	})

	// 3. Initialize Repo & Service
	repo := postgres.NewTaggingRepository(deps.DB, deps.Logger)
	svc := NewService(repo, deps.Audit, deps.EventBus, deps.Authz, deps.Logger)

	return &Container{
		Service:      svc,
		AdminService: svc, // Service implements AdminService interface
	}, nil
}
