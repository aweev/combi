package configuration

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/configuration"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


// Container holds the service and its dependencies.
type Container struct {
	Service configuration.Service
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	EventBus  events.Bus
	Registrar rbac.PermissionRegistrar
	Config    *config.Config
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions
	registerPermissions(deps.Registrar)

	// 2. Init Repository
	repo := postgres.NewConfigRepository(deps.DB, deps.Logger)

	// 3. Init Service
	svc := NewService(repo, deps.EventBus, deps.Logger)

	// 4. Register Own Manifest
	svc.RegisterService(buildManifest())

	return &Container{
		Service: svc,
	}, nil
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	registrar.Register(rbac.PermissionDefinition{
		Name:        configuration.PermRead,
		Description: "Can view system settings and themes.",
		Category:    "Configuration",
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        configuration.PermManage,
		Description: "Can modify system settings, themes, and branding.",
		Category:    "Configuration",
	})
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "config-service",
		DisplayName: "System Configuration",
		Description: "Manages global settings, branding, and themes.",
		Events: []core.EventDefinition{
			{
				Key:   core.EventSettingUpdated,
				Label: "Setting Updated",
				Schema: map[string]string{
					"key":   "string",
					"value": "any",
				},
			},
			{
				Key:   core.EventBrandUpdated,
				Label: "Brand Updated",
				Schema: map[string]string{
					"org_id": "uuid",
					"name":   "string",
				},
			},
		},
		Jobs: []core.JobDefinition{
			// No background jobs for config service currently
		},
		Permissions: []core.PermDefinition{
			{Key: configuration.PermRead, Label: "View Config", Category: "Configuration"},
			{Key: configuration.PermManage, Label: "Manage Config", Category: "Configuration"},
		},
	}
}
