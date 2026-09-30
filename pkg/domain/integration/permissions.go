package integration

import "github.com/combi/pkg/domain/core"

const (
	// PermIntegrationRead allows viewing integration configurations and their status.
	PermIntegrationRead core.PermissionName = "integration:read"
	// PermIntegrationManage allows creating, updating, and deleting integration configurations.
	PermIntegrationManage core.PermissionName = "integration:manage"
	// PermIntegrationTest allows triggering a connection test for an integration.
	PermIntegrationTest core.PermissionName = "integration:test"
	// PermWebhookRead allows viewing received webhook events and their status.
	PermWebhookRead core.PermissionName = "integration:webhook:read"
	// PermWebhookReplay allows re-triggering the processing of a webhook.
	PermWebhookReplay core.PermissionName = "integration:webhook:replay"
)
