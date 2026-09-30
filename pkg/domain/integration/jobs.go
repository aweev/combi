// internal/domain/integration/jobs.go
package integration

import "github.com/combi/pkg/domain/core"

const (
	// JobIntegrationProcessWebhook processes a received webhook event asynchronously.
	JobIntegrationProcessWebhook core.JobType = "integration:process_webhook"
	// JobIntegrationRunSync runs a bulk data synchronization task.
	JobIntegrationRunSync core.JobType = "integration:run_sync"
)
