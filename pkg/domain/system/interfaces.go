package system

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
)

// Service defines the contract for system-wide metadata and control.
type Service interface {
	// Metadata Discovery
	GetEventTypes(ctx context.Context) ([]EventDefinition, error)
	GetPermissions(ctx context.Context) ([]PermissionDefinition, error)
	GetJobTypes(ctx context.Context) ([]string, error)

	// System Status
	GetSystemStatus(ctx context.Context) (*SystemStatus, error)

	// Maintenance Control (Admin only)
	EnableMaintenanceMode(ctx context.Context, reason string) error
	DisableMaintenanceMode(ctx context.Context) error
}

// EventDefinition describes an event for the UI (dropdowns/docs).
type EventDefinition struct {
	Action      core.EventAction `json:"action"`
	Category    string           `json:"category"`    // e.g., "Identity", "Financial"
	Description string           `json:"description"` // e.g., "Triggered when a user logs in"
}

// PermissionDefinition describes a permission for Role management UI.
type PermissionDefinition struct {
	Name        core.PermissionName `json:"name"`
	Category    string              `json:"category"`
	Description string              `json:"description"`
}

// SystemStatus represents the overall health.
type SystemStatus struct {
	Healthy     bool              `json:"healthy"`
	Version     string            `json:"version"`
	Maintenance bool              `json:"maintenance_mode"`
	Uptime      time.Duration     `json:"uptime"`
	Services    map[string]string `json:"services"` // e.g. "database": "up"
}
