package project

import (
	_ "embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service project.Service
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	AuditRecorder audit.EventRecorder
	EventBus      events.Bus
	Authz         rbac.AuthorizationService
}

func NewContainer(deps Dependencies, registrar rbac.PermissionRegistrar, registry core.Registry) (*Container, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("database connection required for project service")
	}

	// 1. Register Permissions & Manifest
	registerPermissions(registrar)
	registry.RegisterService(buildManifest())

	// 2. Repo
	repo := postgres.NewProjectRepository(deps.DB, deps.Logger)

	// 3. Service
	svc := NewService(repo, deps.AuditRecorder, deps.EventBus, deps.Authz, deps.Logger)

	return &Container{
		Service: svc,
	}, nil
}

func registerPermissions(r rbac.PermissionRegistrar) {
	cat := "Project Management"
	// Note: Most of these are checked internally against Member Role, but System Admin might need override
	// For now, we register Org-level creation perm.
	r.Register(rbac.PermissionDefinition{Name: project.PermProjectCreate, Description: "Create new projects in organization.", Category: cat})
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "project-service",
		DisplayName: "Project Workbench",
		Description: "Collaborative tasks, milestones, and file sharing.",
		Icon:        "kanban",
		Events: []core.EventDefinition{
			{Key: core.EventProjectCreated, Label: "Project Created", Description: "New project workspace initialized."},
			{Key: core.EventProjectTaskAssigned, Label: "Task Assigned", Description: "A user was assigned a task."},
			{Key: core.EventProjectTaskCompleted, Label: "Task Completed", Description: "A task was moved to Done."},
		},
		Permissions: []core.PermDefinition{
			{Key: project.PermProjectCreate, Label: "Create Projects", Category: "Projects"},
		},
	}
}
