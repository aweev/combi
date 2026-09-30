package localization

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service localization.Service
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	Cache     cache.Service
	Authz     rbac.AuthorizationService
	Audit     audit.EventRecorder
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	deps.Registrar.Register(rbac.PermissionDefinition{Name: localization.PermManage, Category: "Localization"})

	// Register Manifest
	deps.Registry.RegisterService(buildManifest())

	repo := postgres.NewLocalizationRepository(deps.DB, deps.Logger)
	svc := NewService(repo, deps.Cache, deps.Authz, deps.Audit, deps.Logger)

	return &Container{Service: svc}, nil
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "localization-service",
		DisplayName: "Translation Manager",
		Description: "Manages application locales and translation keys.",
		Events: []core.EventDefinition{
			{Key: core.EventLocalizationKeyCreated, Label: "Key Created", Description: "New translation key added."},
			{Key: core.EventLocalizationValueUpdated, Label: "Translation Updated", Description: "Text value updated for a locale."},
		},
		Permissions: []core.PermDefinition{
			{Key: localization.PermManage, Label: "Manage Translations", Category: "Content"},
			{Key: localization.PermRead, Label: "View Translations", Category: "Content"},
		},
	}
}
