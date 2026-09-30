package feature

import (
	_ "embed"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service        feature.Service
	CacheLifecycle feature.CacheLifecycle
}

type Dependencies struct {
	DB        *sqlx.DB
	Redis     *redis.Client
	Logger    log.Logger
	Audit     audit.EventRecorder
	Registrar rbac.PermissionRegistrar
	EventBus  events.Bus
	Registry  core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Manifest
	deps.Registry.RegisterService(BuildManifest())
	RegisterPermissions(deps.Registrar)

	// 2. Repo
	repo := postgres.NewFeatureRepository(deps.DB, deps.Logger)

	// 3. Cache (In-Memory with Repo fallback)
	flagCache, err := NewFlagCache(repo, deps.Logger, 1*time.Minute)
	if err != nil {
		return nil, err
	}

	// 4. Notifier (Redis Pub/Sub)
	notifier := NewRedisNotifier(deps.Redis, deps.Logger)

	// 5. Engine
	engine := NewEngine()

	// 6. Service
	svc := NewService(repo, flagCache, notifier, engine, deps.Audit, deps.EventBus, deps.Logger)

	// 7. Lifecycle Manager (Orchestrates Cache + Notifier)
	lifecycle := NewLifecycleManager(flagCache, notifier, deps.Logger)

	return &Container{
		Service:        svc,
		CacheLifecycle: lifecycle,
	}, nil
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "feature-service",
		DisplayName: "Feature Flags",
		Description: "Dynamic configuration and rollout management.",
		Icon:        "toggle-right",

		Events: []core.EventDefinition{
			{
				Key:         core.EventFlagUpdated,
				Label:       "Flag Updated",
				Description: "Triggered when a feature flag configuration is modified.",
				Schema:      map[string]string{"flag_key": "string"},
			},
		},

		Permissions: []core.PermDefinition{
			{
				Key:         feature.PermFeatureManage,
				Label:       "Manage Flags",
				Category:    "Features",
				Description: "Create, edit, and archive feature flags.",
				Dangerous:   true,
			},
			{
				Key:         feature.PermFeatureRead,
				Label:       "View Flags",
				Category:    "Features",
				Description: "View feature flag configurations.",
			},
		},
	}
}

func RegisterPermissions(registrar rbac.PermissionRegistrar) {
	manifest := BuildManifest()
	for _, p := range manifest.Permissions {
		registrar.Register(rbac.PermissionDefinition{
			Name:        p.Key,
			Description: p.Description,
			Category:    p.Category,
		})
	}
}
