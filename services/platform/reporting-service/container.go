package reporting

import (
	_ "embed"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service reporting.Service
	Worker  queue.JobProcessor
}

type Dependencies struct {
	Repo     reporting.Repository
	Queue    queue.Dispatcher
	Storage  storage.Service
	Audit    audit.EventRecorder
	Logger   log.Logger
	Registry core.Registry
}

func NewContainer(deps Dependencies) *Container {
	if deps.Registry != nil {
		deps.Registry.RegisterService(BuildManifest())
	}

	// 1. Create Service
	svc := NewService(
		deps.Repo,
		deps.Queue,
		deps.Storage,
		deps.Audit,
		deps.Logger,
	)

	// 2. Create Worker (injecting concrete service to access internal registry)
	worker := NewGenerationWorker(svc.(*service), deps.Logger)

	return &Container{
		Service: svc,
		Worker:  worker,
	}
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "reporting-service",
		DisplayName: "Reporting Engine",
		Description: "Pluggable report generation for PDF, CSV, and dashboard-ready exports.",
		Icon:        "file-bar-chart",
		Version:     "1.0.0",
		Tier:        "capability",
		Owner:       "platform",
		Portable:    true,
		Interfaces: []core.InterfaceDefinition{
			{
				Protocol:    "grpc",
				Package:     "platform.reporting.v1",
				SpecPath:    "api/proto/reporting/v1/reporting.proto",
				Stability:   "stable",
				Description: "Preferred inter-service API for report requests, status polling, and fetcher registration.",
				Consumers:   []string{"analytics-service", "financial-service", "impact-service", "workflow-service"},
			},
			{
				Protocol:    "rest",
				Endpoint:    "/api/v1/admin/reports",
				SpecPath:    "internal/services/reporting/api/openapi.yaml",
				Stability:   "stable",
				Description: "Admin report requests, exports, and artifact downloads.",
			},
			{
				Protocol:    "graphql",
				Endpoint:    "/api/v1/graphql",
				Stability:   "stable",
				Description: "Read-side report summaries for dashboards and management workspaces.",
			},
		},
		Dependencies: []core.DependencyDefinition{
			{Name: "postgres", Type: "postgres", Required: true, ConfigKey: "database", Description: "Report requests, status history, and fetcher metadata."},
			{Name: "queue", Type: "queue", Required: true, ConfigKey: "queue", Description: "Asynchronous report generation and retries."},
			{Name: "storage", Type: "object-storage", Required: true, ConfigKey: "storage", Description: "Generated PDF, CSV, and attachment artifacts."},
			{Name: "audit", Type: "service", Required: true, Description: "Security audit trail for generated and downloaded reports."},
		},
		Telemetry: core.TelemetryDefinition{
			MetricsPrefix: "reporting",
			TraceName:     "reporting-service",
			LogFields:     []string{"report_id", "report_type", "format", "org_id"},
			Dashboards:    []string{"reporting-generation", "reporting-storage", "reporting-failures"},
			Alerts:        []string{"reporting_generation_failed", "reporting_queue_backlog", "reporting_storage_upload_failed"},
		},
		Events: []core.EventDefinition{
			{
				Key:         "reporting.report.generated",
				Label:       "Report Generated",
				Description: "Fired when a report artifact is ready.",
				Schema: map[string]string{
					"report_id": "uuid",
					"type":      "string",
					"format":    "string",
					"file_id":   "uuid",
				},
			},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         reporting.JobGenerateReport,
				Label:       "Generate Report",
				Description: "Generate a report artifact from a registered data fetcher.",
				InputSchema: map[string]string{
					"report_id": "uuid",
					"type":      "string",
					"format":    "string",
				},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: "reporting:read", Label: "View Reports", Category: "Reporting", Description: "View report requests and generated artifacts."},
			{Key: "reporting:generate", Label: "Generate Reports", Category: "Reporting", Description: "Create report generation requests."},
			{Key: "reporting:admin", Label: "Manage Reporting", Category: "Reporting", Description: "Manage report types and retention.", Dangerous: true},
		},
	}
}
