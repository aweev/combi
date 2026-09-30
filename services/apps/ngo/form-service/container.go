package form

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/form"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service form.Service
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	EventBus      events.Bus
	Authz         rbac.AuthorizationService
	Registrar     rbac.PermissionRegistrar
	Registry      core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions & Manifest
	deps.Registry.RegisterService(BuildManifest())
	registerPermissions(deps.Registrar)

	// 2. Repo
	repo := postgres.NewFormRepository(deps.DB, deps.Logger)

	// 3. Service
	svc := NewService(repo, deps.AuditRecorder, deps.EventBus, deps.Authz, deps.Logger)

	return &Container{
		Service: svc,
	}, nil
}

func registerPermissions(r rbac.PermissionRegistrar) {
	const cat = "Forms & Voting"
	r.Register(rbac.PermissionDefinition{Name: form.PermFormManage, Description: "Create and edit forms/surveys.", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: form.PermFormViewResults, Description: "View submission data.", Category: cat})
	r.Register(rbac.PermissionDefinition{Name: form.PermVoteBoard, Description: "Participate in board governance.", Category: cat})
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "form-service",
		DisplayName: "Forms & Voting",
		Description: "Dynamic application forms, surveys, and governance voting.",
		Icon:        "clipboard-list",
		Events: []core.EventDefinition{
			{Key: core.EventSubmissionReceived, Label: "Form Submitted"},
			{Key: core.EventVoteConcluded, Label: "Vote Concluded"},
		},
		Permissions: []core.PermDefinition{
			{Key: form.PermFormManage, Label: "Manage Forms", Category: "Forms"},
		},
	}
}
