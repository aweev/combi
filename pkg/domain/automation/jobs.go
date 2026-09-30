package automation

import "github.com/combi/pkg/domain/core"

const (
	// JobAutomationProcessEvent is the primary job that evaluates an event against rules and triggers executions.
	JobAutomationProcessEvent core.JobType = "automation:process_event"

	// JobAutomationExecuteAction handles the execution of a single, potentially long-running or retryable action.
	JobAutomationExecuteAction core.JobType = "automation:execute_action"
	// JobAutomationContinueExecution is dispatched by a 'delay' action to resume a paused workflow.
	JobAutomationContinueExecution core.JobType = "automation:continue_execution"
)
