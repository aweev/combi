package campaign

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/campaign"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service      campaign.Service
	AdminService campaign.AdminService
	Subscriber   *Subscriber
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
	// 1. Register Manifest
	deps.Registry.RegisterService(BuildManifest())
	RegisterPermissions(deps.Registrar)

	// 2. Repo
	repo := postgres.NewCampaignRepository(deps.DB, deps.Logger)

	// 3. Service
	svc := NewService(repo, deps.AuditRecorder, deps.EventBus, deps.Logger)

	// 4. Subscriber
	sub := NewSubscriber(svc, deps.Logger)

	return &Container{
		Service:      svc,
		AdminService: svc,
		Subscriber:   sub,
	}, nil
}

func (c *Container) RegisterSubscribers(bus events.Bus) {
	bus.Subscribe(string(core.EventPaymentSucceeded), c.Subscriber.Handle)
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "campaign-service",
		DisplayName: "Fundraising Campaigns",
		Description: "Manage major fundraising initiatives and peer-to-peer pages.",
		Icon:        "heart", // Fontngo.ome icon

		Events: []core.EventDefinition{
			{
				Key:         core.EventCampaignCreated,
				Label:       "Campaign Created",
				Description: "Triggered when a new campaign is published.",
				Schema:      map[string]string{"campaign_id": "uuid", "name": "string"},
			},
			{
				Key:         core.EventCampaignCompleted,
				Label:       "Campaign Goal Met",
				Description: "Triggered when a campaign reaches its financial target.",
				Schema:      map[string]string{"campaign_id": "uuid", "total_raised": "int"},
			},
			{
				Key:         core.EventFundraiserCreated,
				Label:       "Fundraiser Page Created",
				Description: "Triggered when a volunteer creates a personal fundraising page.",
				Schema:      map[string]string{"fundraiser_id": "uuid", "owner_id": "uuid", "campaign_id": "uuid"},
			},
			{
				Key:         core.EventFundraiserGoalMet,
				Label:       "Fundraiser Goal Met",
				Description: "Triggered when a personal fundraiser reaches its target.",
				Schema:      map[string]string{"fundraiser_id": "uuid", "owner_id": "uuid"},
			},
		},

		Permissions: []core.PermDefinition{
			{
				Key:         campaign.PermCampaignManage,
				Label:       "Manage Campaigns",
				Category:    "Campaigns",
				Description: "Create, edit, and archive main campaigns.",
				Dangerous:   true,
			},
			{
				Key:         campaign.PermFundraiserManage,
				Label:       "Manage Own Fundraiser",
				Category:    "Campaigns",
				Description: "Edit personal fundraising page details.",
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
