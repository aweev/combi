// internal/domain/automation/models.go
package automation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type RuleStatus string

const (
	RuleStatusDraft    RuleStatus = "DRAFT"
	RuleStatusActive   RuleStatus = "ACTIVE"
	RuleStatusInactive RuleStatus = "INACTIVE"
)

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "PENDING"
	ExecutionStatusRunning   ExecutionStatus = "RUNNING"
	ExecutionStatusCompleted ExecutionStatus = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatus = "FAILED"
	ExecutionStatusCancelled ExecutionStatus = "CANCELLED"
	ExecutionStatusSkipped   ExecutionStatus = "SKIPPED" // For throttled/debounced executions.
)

type ConditionLogic string

const (
	ConditionLogicAND ConditionLogic = "AND"
	ConditionLogicOR  ConditionLogic = "OR"
)

// --- Core Models ---

// Trigger defines the event that starts a rule's evaluation.
type Trigger struct {
	Type     string                 `json:"type"`              // e.g., "user.created", "donation.succeeded"
	Filters  map[string]interface{} `json:"filters,omitempty"` // Pre-filtering on event data
	Debounce *time.Duration         `json:"debounce,omitempty"`
	Throttle *time.Duration         `json:"throttle,omitempty"`
}

// Rule is the central model for an automation.
type Rule struct {
	ID                uuid.UUID
	Name              string
	Description       string
	Status            RuleStatus
	Priority          int
	Trigger           Trigger     `gorm:"type:jsonb"`
	Conditions        []Condition `gorm:"type:jsonb"`
	Logic             ConditionLogic
	Actions           []Action       `gorm:"type:jsonb"`
	Metadata          map[string]any `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ExecutionStrategy string `json:"execution_strategy" gorm:"default:'SEQUENCE'"`
}

// Condition now includes an optional Negate field.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
	Negate   bool   `json:"negate,omitempty"`
}

// Action is a single step to be executed if a rule's conditions are met.
type Action struct {
	Type   string         `json:"type"`             // e.g., "notification.send_email"
	Config map[string]any `json:"config,omitempty"` // Configuration for the action
	Delay  *time.Duration `json:"delay,omitempty"`
}

// Event now includes a Source field.
type Event struct {
	ID        uuid.UUID
	Type      string
	Source    string // e.g., "system", "webhook", "retry"
	Timestamp time.Time
	Data      map[string]any `gorm:"type:jsonb"`
	ActorID   *uuid.UUID
	Metadata  map[string]any `gorm:"type:jsonb"`
}

// Execution model is enhanced for better auditing and workflow control.
type Execution struct {
	ID               uuid.UUID
	RuleID           uuid.UUID
	EventID          uuid.UUID
	Status           ExecutionStatus
	TriggerData      json.RawMessage   `gorm:"type:jsonb"`
	ActionsSnapshot []Action `gorm:"type:jsonb" json:"actions_snapshot"` 
		ConditionsResult []ConditionResult `gorm:"type:jsonb"`
	ActionsResult    []ActionResult    `gorm:"type:jsonb"`
	StartedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	Duration         time.Duration
	Error            *string
}

// --- Result Models (for JSONB storage in Execution) ---

type ConditionResult struct {
	Condition Condition `json:"condition"`
	Matched   bool      `json:"matched"`
	Error     *string   `json:"error,omitempty"`
}

type ActionResult struct {
	Action   Action         `json:"action"`
	Success  bool           `json:"success"`
	Output   map[string]any `json:"output,omitempty"`
	Error    *string        `json:"error,omitempty"`
	Duration time.Duration  `json:"duration"`
}

// ActionSimulation is a DTO used in the TestResult.
type ActionSimulation struct {
	Action       Action         `json:"action"`
	WouldExecute bool           `json:"wouldExecute"`
	Preview      map[string]any `json:"preview"`
}

// RuleStatistics provides performance metrics for a single rule.
type RuleStatistics struct {
	RuleID             uuid.UUID     `json:"ruleId"`
	TotalExecutions    int64         `json:"totalExecutions"`
	SuccessCount       int64         `json:"successCount"`
	FailureCount       int64         `json:"failureCount"`
	SuccessRate        float64       `json:"successRate"`
	AvgExecutionTimeMs float64       `json:"avgExecutionTimeMs"`
	Period             time.Duration `json:"-"`
}

// SystemStatistics provides system-wide performance metrics.
type SystemStatistics struct {
	TotalExecutions    int64            `json:"totalExecutions"`
	TotalSuccesses     int64            `json:"totalSuccesses"`
	TotalFailures      int64            `json:"totalFailures"`
	AvgExecutionTimeMs float32          `json:"avgExecutionTimeMs"`
	TopFailingRules    []RuleStatistics `json:"topFailingRules"`
	Period             time.Duration    `json:"-"`
}
