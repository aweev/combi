// internal/domain/queue/jobs.go
package queue

import "github.com/combi/pkg/domain/core"

const (
	JobQueueMaintenance core.JobType = "queue:maintenance"
	JobWebhookSend      core.JobType = "queue:webhook_send"
)
