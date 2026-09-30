package monitoring

import (
	_ "embed"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"

	// This imports the domain interfaces and constants
	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/monitoring/prometheus"
)


type Container struct {
	Service monitoring.AlertAdminService
	Worker  queue.JobProcessor
}

type Dependencies struct {
	DB            *sqlx.DB
	Logger        log.Logger
	EventBus      events.Bus
	AuditRecorder audit.EventRecorder
	PrometheusURL string
	Registry      core.Registry // <--- Added to register the manifest
}

func NewContainer(deps Dependencies) *Container {
	// 1. Register Manifest (Crucial step for system discovery)
	deps.Registry.RegisterService(BuildManifest())

	// 2. Initialize Infrastructure Adapters
	repo := postgres.NewMonitoringRepository(deps.DB, deps.Logger)
	promClient := prometheus.NewClient(deps.PrometheusURL, deps.Logger)

	// 3. Initialize Domain Services
	// Note: 'repo' implements both RuleRepository and SilenceRepository interfaces
	svc := NewAdminService(repo, repo, deps.AuditRecorder, deps.Logger)

	// 4. Initialize Workers
	worker := NewAlertWorker(repo, repo, promClient, deps.EventBus, deps.Logger)

	return &Container{
		Service: svc,
		Worker:  worker,
	}
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "monitoring-service",
		DisplayName: "System Monitoring & Alerts",
		Description: "Proactive health monitoring with Prometheus integration.",
		Icon:        "activity",
		Version:     "1.0.0",
		Tier:        "platform",
		Owner:       "platform",
		Portable:    true,
		Interfaces: []core.InterfaceDefinition{
			{
				Protocol:    "grpc",
				Package:     "platform.monitoring.v1",
				SpecPath:    "api/proto/monitoring/v1/monitoring.proto",
				Stability:   "stable",
				Description: "Inter-service alert evaluation, silence management, and health status API.",
				Consumers:   []string{"queue-service", "workflow-service", "notification-service"},
			},
			{
				Protocol:    "rest",
				Endpoint:    "/api/v1/admin/monitoring",
				SpecPath:    "internal/services/monitoring/api/openapi.yaml",
				Stability:   "stable",
				Description: "Admin alert rules, silences, and incident operations.",
			},
		},
		Dependencies: []core.DependencyDefinition{
			{Name: "postgres", Type: "postgres", Required: true, ConfigKey: "database", Description: "Alert rules, silences, incidents, and evaluation history."},
			{Name: "prometheus", Type: "external-api", Required: true, ConfigKey: "monitoring.prometheusURL", Description: "Metric query source for alert evaluation."},
			{Name: "queue", Type: "queue", Required: true, ConfigKey: "queue", Description: "Scheduled alert evaluation jobs."},
			{Name: "event-bus", Type: "event-bus", Required: true, ConfigKey: "eventBus", Description: "Alert fired and resolved events."},
		},
		Telemetry: core.TelemetryDefinition{
			MetricsPrefix: "monitoring",
			TraceName:     "monitoring-service",
			LogFields:     []string{"rule_id", "severity", "status", "incident_id"},
			Dashboards:    []string{"monitoring-alerts", "monitoring-slo", "monitoring-queue"},
			Alerts:        []string{"monitoring_evaluation_failed", "monitoring_prometheus_unavailable", "monitoring_notification_delivery_failed"},
		},

		Events: []core.EventDefinition{
			{
				Key:         "monitoring.alert.fired",
				Label:       "Alert Triggered",
				Description: "Fired when an alert condition is met.",
				Schema: map[string]string{
					"rule_id":  "uuid",
					"severity": "string",
					"value":    "float",
					"labels":   "map",
				},
			},
			{
				Key:         "monitoring.alert.resolved",
				Label:       "Alert Resolved",
				Description: "Fired when an alert condition clears.",
				Schema: map[string]string{
					"rule_id":  "uuid",
					"duration": "duration",
				},
			},
		},

		Jobs: []core.JobDefinition{
			{
				// FIX: Use the constant from the imported 'monitoring' package
				Key:         monitoring.JobEvaluateAlerts,
				Label:       "Evaluate Alert Rules",
				Description: "Execute PromQL queries and fire alerts.",
				InputSchema: map[string]string{},
			},
		},

		Permissions: []core.PermDefinition{
			{
				Key:         "monitoring:manage",
				Label:       "Manage Alerts",
				Category:    "Monitoring",
				Description: "Create and edit alert rules and silences.",
				Dangerous:   true,
			},
		},
	}
}
