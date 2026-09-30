package core

import "context"

// ServiceManifest describes the public capabilities of a microservice.
// The UI consumes this to build the Automation Builder and RBAC screens.
type ServiceManifest struct {
	ServiceName  string                 `json:"service_name"` // e.g., "analytics-service"
	DisplayName  string                 `json:"display_name"` // e.g., "Data & Analytics"
	Description  string                 `json:"description"`
	Icon         string                 `json:"icon"`
	Version      string                 `json:"version,omitempty"`
	Tier         string                 `json:"tier,omitempty"`
	Owner        string                 `json:"owner,omitempty"`
	Portable     bool                   `json:"portable"`
	Interfaces   []InterfaceDefinition  `json:"interfaces,omitempty"`   // gRPC, REST, GraphQL surfaces exposed by this service
	Dependencies []DependencyDefinition `json:"dependencies,omitempty"` // Runtime dependencies needed when copying the service
	Telemetry    TelemetryDefinition    `json:"telemetry,omitempty"`
	Events       []EventDefinition      `json:"events"`      // Events this service EMITS
	Jobs         []JobDefinition        `json:"jobs"`        // Background jobs this service exposes
	Permissions  []PermDefinition       `json:"permissions"` // Permissions this service enforces
}

// EventDefinition describes an event so the UI can create "Triggers".
type EventDefinition struct {
	Key         EventAction       `json:"key"`
	Label       string            `json:"label"`       // Human readable: "Donation Received"
	Description string            `json:"description"` // Tooltip
	Schema      map[string]string `json:"schema"`      // Variables available: {"amount": "money", "donor_email": "string"}
}

// JobDefinition describes a job so the UI can create "Actions" or "Manual Triggers".
type JobDefinition struct {
	Key         JobType           `json:"key"`
	Label       string            `json:"label"` // Human readable: "Generate Monthly Report"
	Description string            `json:"description"`
	InputSchema map[string]string `json:"input_schema"` // Inputs needed: {"month": "int", "year": "int"}
}

// PermDefinition describes a permission for the RBAC UI.
type PermDefinition struct {
	Key         PermissionName `json:"key"`
	Label       string         `json:"label"`       // e.g., "View Dashboards"
	Description string         `json:"description"` // e.g., "Allows access to the main overview."
	Category    string         `json:"category"`    // Grouping: "Reporting", "Configuration"
	Dangerous   bool           `json:"dangerous"`
}

// InterfaceDefinition describes a service boundary exposed to other systems.
// Internal service-to-service calls should prefer gRPC. REST and GraphQL are for
// public APIs, admin workflows, or query aggregation where they fit better.
type InterfaceDefinition struct {
	Protocol    string   `json:"protocol"` // grpc, rest, graphql, websocket
	Package     string   `json:"package,omitempty"`
	Endpoint    string   `json:"endpoint,omitempty"`
	SpecPath    string   `json:"spec_path,omitempty"`
	Stability   string   `json:"stability,omitempty"` // experimental, stable, deprecated
	Description string   `json:"description,omitempty"`
	Consumers   []string `json:"consumers,omitempty"`
}

// DependencyDefinition documents everything a copied service needs to run.
type DependencyDefinition struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // postgres, redis, queue, event-bus, object-storage, temporal, external-api
	Required    bool   `json:"required"`
	ConfigKey   string `json:"config_key,omitempty"`
	Description string `json:"description,omitempty"`
}

// TelemetryDefinition makes observability requirements discoverable.
type TelemetryDefinition struct {
	MetricsPrefix string   `json:"metrics_prefix,omitempty"`
	TraceName     string   `json:"trace_name,omitempty"`
	LogFields     []string `json:"log_fields,omitempty"`
	Dashboards    []string `json:"dashboards,omitempty"`
	Alerts        []string `json:"alerts,omitempty"`
}

// Registry is the interface the ConfigService must implement to store these.
type Registry interface {
	RegisterService(manifest ServiceManifest)
	GetSystemCapabilities(ctx context.Context) ([]ServiceManifest, error)
}

type SystemCapabilities struct {
	Permissions []PermDefinition  `json:"permissions"`
	Events      []EventDefinition `json:"events"`
	Jobs        []JobDefinition   `json:"jobs"`
	Services    []ServiceManifest `json:"services"`
}
