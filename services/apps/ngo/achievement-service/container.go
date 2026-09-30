package achievement

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service    achievement.Service
	Subscriber *Subscriber
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	EventBus  events.Bus
	UserSvc   user.Service
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry
	Cache     cache.Service
	Config    config.AchievementConfig
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Metadata
	RegisterPermissions(deps.Registrar)
	deps.Registry.RegisterService(BuildManifest())

	// 2. Infra
	repo := postgres.NewAchievementRepository(deps.DB, deps.Logger)

	// 3. Core Logic
	svc := NewService(
		repo,
		deps.UserSvc,
		deps.EventBus,
		deps.Cache,
		deps.Config,
		deps.Logger,
	)

	// 4. Async Listener
	sub := NewSubscriber(svc, deps.Logger)

	return &Container{
		Service:    svc,
		Subscriber: sub,
	}, nil
}

// RegisterSubscribers wires up specific event topics to handlers.
func (c *Container) RegisterSubscribers(bus events.Bus) {
	// We use the specific handler methods defined in the subscriber
	bus.Subscribe(core.EventUserLoggedIn.String(), c.Subscriber.HandleLogin)
	bus.Subscribe(core.EventDonationReceived.String(), c.Subscriber.HandleDonation)
	bus.Subscribe(core.EventHoursLogged.String(), c.Subscriber.HandleVolunteerHours)
	bus.Subscribe(core.EventProjectCompleted.String(), c.Subscriber.HandleProjectCompletion)
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "achievement-service",
		DisplayName: "Gamification",
		Description: "Manages badges, rules, and leaderboards.",
		Icon:        "trophy",
		Events: []core.EventDefinition{
			{
				Key:         core.EventBadgeAwarded,
				Label:       "Badge Awarded",
				Description: "Triggered when a user earns a new badge.",
				Schema:      map[string]string{"user_id": "uuid", "badge_name": "string"},
			},
		},
		Permissions: []core.PermDefinition{
			{
				Key:         achievement.PermAchievementManage,
				Label:       "Manage Badges",
				Description: "Create and edit badges and rules.",
				Category:    "Gamification",
			},
			{
				Key:         achievement.PermAchievementAward,
				Label:       "Award Badge",
				Description: "Manually grant badges to users.",
				Category:    "Gamification",
			},
		},
	}
}

func RegisterPermissions(registrar rbac.PermissionRegistrar) {
	for _, p := range BuildManifest().Permissions {
		registrar.Register(rbac.PermissionDefinition{
			Name:        p.Key,
			Description: p.Description,
			Category:    p.Category,
		})
	}
}
