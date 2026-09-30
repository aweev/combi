package workflow

import (
	_ "embed"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/domain/workflow"
	"github.com/combi/pkg/log"
)


// Dependencies holds all external services required by the workflow system.
// This ensures the Workflow Service has access to the rest of the monolith.
type Dependencies struct {
	Repo              workflow.Repository
	Logger            log.Logger
	Registrar         rbac.PermissionRegistrar
	TemporalClient    client.Client
	AuditRecorder     audit.EventRecorder
	CatalogAdminSvc   catalog.AdminService
	NotificationSvc   notification.Service
	UserService       user.Service
	UserAdminSvc      user.AdminService
	ContentAdminSvc   content.AdminService
	OrgService        organization.Service
	ProjectService    project.Service
	RBACAdminSvc      rbac.AdminService
	FinancialAdminSvc financial.AdminService
}

// Worker wraps a Temporal worker for lifecycle management (Start/Stop).
type Worker struct {
	temporalWorker worker.Worker
	logger         log.Logger
}

// Start begins listening for tasks. This is a blocking call (run in goroutine).
func (w *Worker) Start() error {
	w.logger.Info("Starting Temporal worker")
	return w.temporalWorker.Run(worker.InterruptCh())
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	w.logger.Info("Stopping Temporal worker")
	w.temporalWorker.Stop()
}

// Container holds the client-facing service and all background workers.
type Container struct {
	Service    workflow.Service
	Repository workflow.Repository
	Workers    []*Worker
}

// StartAll starts all configured workers in background goroutines.
func (c *Container) StartAll() error {
	if len(c.Workers) == 0 {
		return nil
	}

	errChan := make(chan error, len(c.Workers))
	for _, w := range c.Workers {
		go func(workerToStart *Worker) {
			if err := workerToStart.Start(); err != nil {
				errChan <- err
			}
		}(w)
	}

	// In a real app, you might handle immediate startup errors here
	return nil
}

// StopAll gracefully stops all configured workers.
func (c *Container) StopAll() {
	for _, w := range c.Workers {
		w.Stop()
	}
}

// NewContainer builds the complete workflow service container.
func NewContainer(deps Dependencies) (*Container, error) {
	const category = "Workflows"

	// 1. Register Permissions
	permissions := []rbac.PermissionDefinition{
		{Name: workflow.PermWorkflowManage, Description: "Can manage workflow instances", Category: category},
		{Name: workflow.PermWorkflowApprove, Description: "Can approve workflow requests", Category: category},
	}
	for _, p := range permissions {
		deps.Registrar.Register(p)
	}

	// 2. Build Service
	svc := NewService(
		deps.TemporalClient,
		deps.Repo,
		deps.AuditRecorder,
		deps.Logger,
	)

	// 3. Build Activities (The bridge to other domains)
	activities := NewActivities(
		deps.Logger,
		deps.CatalogAdminSvc,
		deps.NotificationSvc,
		deps.UserService,
		deps.UserAdminSvc,
		deps.ContentAdminSvc,
		deps.OrgService,
		deps.ProjectService,
		deps.RBACAdminSvc,
		deps.FinancialAdminSvc,
	)

	// 4. Configure Temporal Workers
	var workers []*Worker

	// -- General Worker (Handles most workflows) --
	generalWorkerOpts := worker.Options{
		MaxConcurrentActivityExecutionSize:     50,
		MaxConcurrentWorkflowTaskExecutionSize: 50,
	}
	generalWorker := worker.New(deps.TemporalClient, workflow.GeneralTaskQueue, generalWorkerOpts)

	// Register Workflows & Activities
	generalWorker.RegisterWorkflow(ContentModerationWorkflow)
	generalWorker.RegisterWorkflow(GrantApplicationWorkflow)
	generalWorker.RegisterWorkflow(VolunteerOnboardingWorkflow)
	generalWorker.RegisterWorkflow(PartnershipProposalWorkflow)
	generalWorker.RegisterActivity(activities)

	workers = append(workers, &Worker{
		temporalWorker: generalWorker,
		logger:         deps.Logger.With(log.String("worker_queue", workflow.GeneralTaskQueue)),
	})

	// -- Application Worker (Handles high-priority or isolated workflows) --
	appWorkerOpts := worker.Options{
		MaxConcurrentActivityExecutionSize:     20,
		MaxConcurrentWorkflowTaskExecutionSize: 20,
	}
	applicationWorker := worker.New(deps.TemporalClient, workflow.ApplicationTaskQueue, appWorkerOpts)

	applicationWorker.RegisterWorkflow(ParticipantApplicationWorkflow)
	applicationWorker.RegisterActivity(activities)

	workers = append(workers, &Worker{
		temporalWorker: applicationWorker,
		logger:         deps.Logger.With(log.String("worker_queue", workflow.ApplicationTaskQueue)),
	})

	return &Container{
		Service:    svc,
		Repository: deps.Repo,
		Workers:    workers,
	}, nil
}
