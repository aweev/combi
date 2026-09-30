package search

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service       search.Service
	RebuildWorker queue.JobProcessor
	IndexWorker   queue.JobProcessor
	DeleteWorker  queue.JobProcessor
	Subscriber    *Subscriber
}

type Dependencies struct {
	DB         *sqlx.DB
	Logger     log.Logger
	Registrar  rbac.PermissionRegistrar
	Registry   core.Registry
	Dispatcher queue.Dispatcher
	RBAC       rbac.AuthorizationService
	EventBus   events.Bus
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Permissions & Manifest
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	// 2. Engine
	engine := postgres.NewPostgresSearchEngine(deps.DB, deps.Logger)

	// 3. Service
	svc := NewService(engine, deps.RBAC, deps.Dispatcher, deps.Logger)

	// 4. Workers
	rebuildWorker := NewRebuildWorker(svc, deps.Logger)
	indexWorker := NewIndexEntityWorker(svc, deps.Logger)
	deleteWorker := NewDeleteEntityWorker(svc, deps.Logger)

	// 5. Subscriber
	subscriber := NewSubscriber(svc, deps.Logger)
	// Note: We do NOT call SubscribeToAll here anymore.
	// It is called by app.go via RegisterSubscribers to ensure consistent startup order.

	return &Container{
		Service:       svc,
		RebuildWorker: rebuildWorker,
		IndexWorker:   indexWorker,
		DeleteWorker:  deleteWorker,
		Subscriber:    subscriber,
	}, nil
}

// RegisterSubscribers implements events.EventSubscriber
func (c *Container) RegisterSubscribers(bus events.Bus) {
	// Delegate to the subscriber's logic which knows the specific mappings (users, content, etc.)
	c.Subscriber.SubscribeToAll(bus)
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "search-service",
		DisplayName: "Search Engine",
		Description: "Centralized full-text search with RBAC scoping.",
		Events:      []core.EventDefinition{
			// Search mainly consumes events
		},
		Jobs: []core.JobDefinition{
			{
				Key:         search.JobRebuildIndex,
				Label:       "Rebuild Search Index",
				Description: "Wipes and re-indexes a specific entity type from source.",
				InputSchema: map[string]string{"index": "string (users, projects)"},
			},
		},
		Permissions: []core.PermDefinition{
			{
				Key:         search.PermSearchAdmin,
				Label:       "Search Admin",
				Description: "Allows triggering index rebuilds.",
				Category:    "System",
				Dangerous:   true,
			},
		},
	}
}

func registerPermissions(r rbac.PermissionRegistrar) {
	for _, p := range buildManifest().Permissions {
		r.Register(rbac.PermissionDefinition{Name: p.Key, Description: p.Description, Category: p.Category})
	}
}
