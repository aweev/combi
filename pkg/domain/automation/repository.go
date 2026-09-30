package automation

import (
	"context"
	"time" // <-- ADDED

	"github.com/google/uuid"
)

// Repository is the master persistence interface for the automation domain.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	RuleRepository
	ExecutionRepository
}

type RuleRepository interface {
	CreateRule(ctx context.Context, rule *Rule) error
	GetRule(ctx context.Context, id uuid.UUID) (*Rule, error)
	UpdateRule(ctx context.Context, rule *Rule) error
	DeleteRule(ctx context.Context, id uuid.UUID) error
	ListRules(ctx context.Context, filter RuleFilter) ([]Rule, int64, error)         // Return value type, not pointer
	GetActiveRulesByTrigger(ctx context.Context, triggerType string) ([]Rule, error) // Corrected from your draft
	IncrementRuleStats(ctx context.Context, ruleID uuid.UUID, success bool) error    // Added
	GetUniqueActiveTriggers(ctx context.Context) ([]string, error)
}

type ExecutionRepository interface {
	CreateExecution(ctx context.Context, execution *Execution) error
	GetExecution(ctx context.Context, id uuid.UUID) (*Execution, error)
	UpdateExecution(ctx context.Context, execution *Execution) error
	ListExecutions(ctx context.Context, filter ExecutionFilter) ([]Execution, int64, error)                   // Return value type
	GetRuleExecutionStats(ctx context.Context, ruleID uuid.UUID, from, to time.Time) (*RuleStatistics, error) // Added
	GetSystemStats(ctx context.Context, from, to time.Time) (*SystemStatistics, error)                        // Added
	AppendActionResultBatch(ctx context.Context, executionID uuid.UUID, results []ActionResult) error

	AppendActionResult(ctx context.Context, executionID uuid.UUID, result ActionResult) error
	FindStaleExecutions(ctx context.Context, olderThan time.Time, limit int) ([]Execution, error)
	PruneExecutions(ctx context.Context, olderThan time.Time, batchSize int) (int64, error)
	IncrementRuleStatsBatch(ctx context.Context, ruleID uuid.UUID, totalDelta, successDelta int64) error
}
