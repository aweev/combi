package workflow

import "github.com/combi/pkg/domain/core"

const (
	// JobStartWorkflow is a task used to decouple the EventBus from Temporal.
	// Event -> Queue -> JobStartWorkflow -> Temporal
	JobStartWorkflow core.JobType = "workflow:start"
)
