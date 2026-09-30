package messaging

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


// Container holds the public API of the Messaging Service.
type Container struct {
	Service messaging.Service
	// If you had background workers (e.g. archiving old chats), they would go here.
}

// Dependencies required to initialize the module.
type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	EventBus  events.Bus
	UserSvc   user.Service
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
}

// NewContainer initializes the service, repository, and wires everything together.
func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions & Service Manifest
	RegisterPermissions(deps.Registrar)
	deps.Registry.RegisterService(BuildManifest())

	// 2. Initialize Repository
	repo := postgres.NewMessagingRepository(deps.DB, deps.Logger)

	// 3. Initialize Service
	svc := NewService(repo, deps.UserSvc, deps.EventBus, deps.Logger)

	return &Container{
		Service: svc,
	}, nil
}

// RegisterSubscribers implements the events.Subscriber interface.
// Even if empty now, it ensures the app.go loop handles this service gracefully.
func (c *Container) RegisterSubscribers(bus events.Bus) {
	// Example Future Logic:
	// bus.Subscribe("project.created", func(ctx context.Context, evt events.DomainEvent) error {
	//     // Logic to auto-create "Project Team" chat
	//     return nil
	// })
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "messaging-service",
		DisplayName: "Chat & Messaging",
		Description: "Real-time communication channels and inbox management.",
		Icon:        "message-square",

		// Events emitted by this service
		Events: []core.EventDefinition{
			{
				Key:         core.EventMessageCreated,
				Label:       "Message Sent",
				Description: "Triggered when a user sends a message in a channel.",
				Schema:      map[string]string{"channel_id": "uuid", "sender_id": "uuid", "content": "string"},
			},
			{
				Key:         core.EventMessageRead,
				Label:       "Message Read",
				Description: "Triggered when a user marks a channel as read.",
				Schema:      map[string]string{"channel_id": "uuid", "user_id": "uuid"},
			},
			{
				Key:         core.EventMessageFlagged,
				Label:       "Message Flagged",
				Description: "Triggered when a message is reported for moderation.",
				Schema:      map[string]string{"message_id": "uuid", "reporter_id": "uuid", "reason": "string"},
			},
		},

		// Permissions required to access this service
		Permissions: []core.PermDefinition{
			{
				Key:         messaging.PermChatUse,
				Label:       "Use Chat",
				Description: "Send and receive messages.",
				Category:    "Messaging",
			},
			{
				Key:         messaging.PermChatModerator,
				Label:       "Moderate Chat",
				Description: "Delete messages and manage channel bans.",
				Category:    "Messaging",
				Dangerous:   true,
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
