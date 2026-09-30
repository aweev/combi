package realtime

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/realtime"
	persistence_repo "github.com/combi/pkg/persistence/redis"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
)


// Container holds the fully instantiated Real-time service components.
type Container struct {
	Hub          realtime.Hub
	PresenceRepo realtime.PresenceRepository
	Workers      []queue.JobProcessor
	Subscriber   *EventSubscriber
}

// Dependencies holds all the external services required to build the real-time service.
type Dependencies struct {
	RedisClient   *redis.Client
	RBACService   rbac.AuthorizationService
	AuditRecorder audit.EventRecorder
	Metrics       monitoring.Recorder
	Logger        log.Logger
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
	Config        config.RealtimeConfig
}

// NewContainer is the factory function for the entire real-time service.
func NewContainer(deps Dependencies, config config.RealtimeConfig) (*Container, error) {
	// 1. Register permissions owned by this domain.
	deps.Registrar.Register(rbac.PermissionDefinition{
		Name:        realtime.PermRealtimeSubscribe,
		Description: "Base permission for subscribing to real-time channels.",
		Category:    "Real-time",
	})
	deps.Registry.RegisterService(buildManifest())

	// 2. Build the Broker for horizontal scaling.
	broker := NewRedisBroker(deps.RedisClient, deps.Logger)
	authorizer := NewRBACAuthorizer(deps.RBACService, deps.Logger)

	// 3. Build the Authorizer for security.
	presenceRepo := persistence_repo.NewRedisPresenceRepository(deps.RedisClient)

	// 4. Build the central Hub, injecting its dependencies.
	hub := NewHub(config, broker, authorizer, deps.AuditRecorder, deps.Metrics, deps.Logger)

	// 5. Workers
	presenceWorker := NewPresenceWorker(presenceRepo, deps.Logger)

	// 6. Subscribers
	subscriber := NewEventSubscriber(hub, deps.Logger)

	// 7. Assemble and return the container.
	container := &Container{
		Hub:          hub,
		PresenceRepo: presenceRepo,
		Workers:      []queue.JobProcessor{presenceWorker},
		Subscriber:   subscriber,
	}

	deps.Logger.Info("Real-time service container built successfully")
	return container, nil
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "realtime-service",
		DisplayName: "Realtime Gateway",
		Description: "WebSockets and live updates.",
		Events: []core.EventDefinition{
			{Key: realtime.EventUserConnected, Label: "User Online", Description: "User established a WS connection."},
			{Key: realtime.EventUserDisconnected, Label: "User Offline", Description: "User dropped connection."},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         realtime.JobCleanupStaleConnections,
				Label:       "Prune Connections",
				Description: "Force removal of ghost connections from presence store.",
				InputSchema: map[string]string{},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: realtime.PermRealtimeBroadcastSystem, Label: "Broadcast System Alert", Category: "Communication"},
		},
	}
}

func (c *Container) RegisterSubscribers(bus events.Bus) {
	c.Subscriber.RegisterSubscribers(bus)
}
