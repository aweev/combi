// internal/domain/automation/interfaces.go
package automation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// Service defines the administrative contract for the automation system.
type Service interface {
	// Rule Management
	CreateRule(ctx context.Context, params CreateRuleParams) (*Rule, error)
	GetRule(ctx context.Context, id uuid.UUID) (*Rule, error)
	UpdateRule(ctx context.Context, ruleID uuid.UUID, params UpdateRuleParams) (*Rule, error)
	DeleteRule(ctx context.Context, id uuid.UUID) error
	ListRules(ctx context.Context, filter RuleFilter) ([]Rule, int64, error)
	EnableRule(ctx context.Context, id uuid.UUID) error
	DisableRule(ctx context.Context, id uuid.UUID) error

	// Execution Management
	GetExecution(ctx context.Context, id uuid.UUID) (*Execution, error)
	ListExecutions(ctx context.Context, filter ExecutionFilter) ([]Execution, int64, error)
	RetryExecution(ctx context.Context, id uuid.UUID) error
	CancelExecution(ctx context.Context, id uuid.UUID) error

	// Testing & Debugging
	TestRule(ctx context.Context, id uuid.UUID, testEvent Event) (*TestResult, error)
	ValidateCondition(ctx context.Context, condition Condition) error
	ValidateAction(ctx context.Context, action Action) error

	// Statistics
	GetRuleStatistics(ctx context.Context, id uuid.UUID, period time.Duration) (*RuleStatistics, error)
	GetSystemStatistics(ctx context.Context, period time.Duration) (*SystemStatistics, error)

	// Ad-hoc action execution
	ExecuteAction(ctx context.Context, actionType string, actionConfig map[string]interface{}) error

	// --- Methods for Background Workers ---
	ProcessEvent(ctx context.Context, event Event) error
	ExecuteSingleAction(ctx context.Context, executionID uuid.UUID, action Action, eventData map[string]any) error
	ContinueExecution(ctx context.Context, executionID uuid.UUID, eventData map[string]any, actionsToRun []Action) error
}

// EventIngestService is the high-performance port for ingesting events.
type EventIngestService interface {
	Ingest(ctx context.Context, event Event) error
}

// WorkerService defines the contract for background job processors.
type WorkerService interface {
	ProcessEvent(ctx context.Context, event Event) error
	ContinueExecution(ctx context.Context, executionID uuid.UUID, eventData map[string]any, actionsToRun []Action) error
}

type SchemaValidator interface {
	// Validate returns error if the event data doesn't match the expected schema for the event type
	Validate(eventType string, data map[string]any) error
}

// --- DTOs for Service Methods ---

type CreateRuleParams struct {
	Name        string `validate:"required,min=3"`
	Description string
	Status      RuleStatus `validate:"required,oneof=ACTIVE INACTIVE"`
	Priority    int        `validate:"min=0,max=100"`
	Trigger     Trigger    `validate:"required"`
	Conditions  []Condition
	Logic       ConditionLogic `validate:"required,oneof=AND OR"`
	Actions     []Action       `validate:"required,min=1"`
	Metadata    map[string]any
}

type UpdateRuleParams struct {
	Name        *string `validate:"omitempty,min=3"`
	Description *string
	Status      *RuleStatus `validate:"omitempty,oneof=ACTIVE INACTIVE"`
	Priority    *int        `validate:"omitempty,min=0,max=100"`
	Trigger     *Trigger
	Conditions  *[]Condition
	Logic       *ConditionLogic `validate:"omitempty,oneof=AND OR"`
	Actions     *[]Action       `validate:"omitempty,min=1"`
	Metadata    map[string]any
}

// --- System Components & Infrastructure ---

// ConditionEvaluator defines the interface for evaluating conditions.
type ConditionEvaluator interface {
	Evaluate(ctx context.Context, conditions []Condition, logic ConditionLogic, eventData map[string]any) (bool, []ConditionResult, error)
	Validate(ctx context.Context, condition Condition) error
	GetSupportedOperators() []string
}

// ActionExecutor is the orchestrator interface used by the Service.
// It handles routing the Action to the correct Handler.
type ActionExecutor interface {
	// Execute takes the full Action struct and the Idempotency Key.
	Execute(ctx context.Context, action Action, eventData map[string]any, idempotencyKey string) (output map[string]any, err error)

	// Validate checks the full Action struct.
	Validate(ctx context.Context, action Action) error

	GetSupportedActions() []string
}

// ActionHandler is the interface for specific plugins (e.g. Email, Webhook).
// It deals with the configuration map, not the Action struct.
type ActionHandler interface {
	Type() string
	Execute(ctx context.Context, config map[string]any, eventData map[string]any, idempotencyKey string) (output map[string]any, err error)
	Validate(ctx context.Context, config map[string]any) error
}

type CreateRuleRequest struct {
	Name        string         `validate:"required"`
	Description string         `validate:"required"`
	Priority    int            `validate:"min=0,max=1000"`
	Trigger     Trigger        `validate:"required"`
	Conditions  []Condition    `validate:"dive"`
	Logic       ConditionLogic `validate:"required,oneof=AND OR"`
	Actions     []Action       `validate:"required,min=1,dive"`
	Metadata    map[string]any `validate:"omitempty"`
}

type UpdateRuleRequest struct {
	Name        *string         `validate:"omitempty,min=3"`
	Description *string         `validate:"omitempty"`
	Status      *RuleStatus     `validate:"omitempty,oneof=DRAFT ACTIVE INACTIVE"`
	Priority    *int            `validate:"omitempty,min=0,max=1000"`
	Trigger     *Trigger        `validate:"omitempty"`
	Conditions  *[]Condition    `validate:"omitempty,dive"`
	Logic       *ConditionLogic `validate:"omitempty,oneof=AND OR"`
	Actions     *[]Action       `validate:"omitempty,min=1,dive"`
	Metadata    *map[string]any `validate:"omitempty"`
}

type RuleFilter struct {
	Status      *RuleStatus `query:"status"`
	TriggerType *string     `query:"triggerType"`
	Search      string      `query:"search"`
	Limit       int         `query:"limit"`
	Offset      int         `query:"offset"`
}

type ExecutionFilter struct {
	RuleID *uuid.UUID       `query:"ruleId"`
	Status *ExecutionStatus `query:"status"`
	From   *time.Time       `query:"from"`
	To     *time.Time       `query:"to"`
	Limit  int              `query:"limit"`
	Offset int              `query:"offset"`
}

type TestResult struct {
	RuleID           uuid.UUID          `json:"ruleId"`
	Event            Event              `json:"event"`
	WouldTrigger     bool               `json:"wouldTrigger"`
	ConditionsResult []ConditionResult  `json:"conditionsResult"`
	ActionsSimulated []ActionSimulation `json:"actionsSimulated"`
	ExecutionTime    time.Duration      `json:"executionTime"`
}
