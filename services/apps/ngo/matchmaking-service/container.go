package matchmaking

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/matchmaking/algorithm"
)


type Container struct {
	Service    matchmaking.Service
	Subscriber *Subscriber
	Worker     []queue.JobProcessor
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	EventBus      events.Bus
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
	AuditRecorder audit.EventRecorder
}

func NewContainer(deps Dependencies) (*Container, error) {
	RegisterPermissions(deps.Registrar)
	deps.Registry.RegisterService(BuildManifest())

	repo := postgres.NewMatchmakingRepository(deps.DB, deps.Logger)
	algo := algorithm.NewWeightedScorer()

	svc := NewService(repo, algo, deps.AuditRecorder, deps.EventBus, deps.Logger)

	// Initialize Subscriber
	sub := NewSubscriber(svc, deps.Logger)
	worker := NewWorker(svc, deps.Logger)
	return &Container{
		Service:    svc,
		Subscriber: sub,
		Worker:     []queue.JobProcessor{worker},
	}, nil
}

// RegisterSubscribers wires the topics to the handler
func (c *Container) RegisterSubscribers(bus events.Bus) {
	topics := []string{
		"user.profile.updated",        // Sync skills
		"catalog.opportunity.created", // Sync volunteer jobs
	}

	for _, t := range topics {
		bus.Subscribe(t, c.Subscriber.Handle)
	}
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "matchmaking-service",
		DisplayName: "Mentorship & Matching",
		Description: "Connects users with mentors and volunteer opportunities.",
		Icon:        "users", // Fontngo.ome icon name

		Events: []core.EventDefinition{
			{
				Key:         core.EventMatchSuggestions,
				Label:       "Matches Found",
				Description: "Triggered when the algorithm identifies candidates.",
				Schema:      map[string]string{"request_id": "uuid", "count": "int"},
			},
			{
				Key:         core.EventMatchEstablished,
				Label:       "Match Started",
				Description: "Triggered when a mentorship/job relationship begins.",
				Schema:      map[string]string{"match_id": "uuid", "mentee_id": "uuid", "mentor_id": "uuid"},
			},
		},

		Permissions: []core.PermDefinition{
			{
				Key:         matchmaking.PermMatchRequest,
				Label:       "Request Mentor",
				Category:    "Matchmaking",
				Description: "Allows user to submit a match request.",
			},
			{
				Key:         matchmaking.PermMatchManage,
				Label:       "Manage Matches",
				Category:    "Matchmaking",
				Description: "Allows admins to view scores and manually pair users.",
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
