package event

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/event/workers"
)


type Container struct {
	Service      event.Service
	AdminService event.AdminService
	Workers      []queue.JobProcessor
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	EventBus      events.Bus
	RBACService   rbac.AuthorizationService
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
	FinancialSvc  financial.Service // Can be nil if Phase 3 not fully wired yet
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Permissions & Manifest
	deps.Registry.RegisterService(BuildManifest())
	registerPermissions(deps.Registrar)

	// 2. Repo
	repo := postgres.NewRepository(deps.DB, deps.Logger)

	// 3. Services
	adminSvc := NewAdminService(
		repo,
		deps.RBACService,
		deps.AuditRecorder,
		deps.EventBus,
		deps.Logger,
	)

	service := NewService(
		repo,
		deps.AuditRecorder,
		deps.EventBus,
		deps.FinancialSvc,
		deps.Logger,
	)

	// 4. Workers
	workers := []queue.JobProcessor{
		workers.NewStatusUpdateWorker(repo, deps.Logger),
		// Reminders worker would go here
	}

	return &Container{
		Service:      service,
		AdminService: adminSvc,
		Workers:      workers,
	}, nil
}

func registerPermissions(r rbac.PermissionRegistrar) {
	const cat = "Events"
	r.Register(rbac.PermissionDefinition{Name: event.PermEventManage, Description: "Create and edit events", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: event.PermEventCheckIn, Description: "Check in attendees", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: event.PermEventRegistrationRead, Description: "View attendee list", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: event.PermEventVenueManage, Description: "Manage venues", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: event.PermEventSpeakerManage, Description: "Manage speakers", Category: cat})
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "event-service",
		DisplayName: "Events",
		Description: "Manages workshops, conferences, and meetups.",
		Icon:        "calendar",
		Events: []core.EventDefinition{
			{Key: core.EventCreated, Label: "Event Created"},
			{Key: core.EventUpdated, Label: "Event Published"},
			{Key: core.EventRegistrationCreated, Label: "Registration Confirmed"},
		},
		Permissions: []core.PermDefinition{
			{Key: event.PermEventManage, Label: "Manage Events", Category: "Events"},
		},
		Jobs: []core.JobDefinition{
			{Key: event.JobEventUpdateStatus, Label: "Update Event Status", Description: "Marks past events as completed."},
		},
	}
}
