package workflow

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type WorkflowType string

const (
	TypeVolunteerOnboarding  WorkflowType = "VOLUNTEER_ONBOARDING"
	TypeGrantApproval        WorkflowType = "GRANT_APPROVAL"
	TypeExpenseReimbursement WorkflowType = "EXPENSE_REIMBURSEMENT"
	TypeProjectReview        WorkflowType = "PROJECT_REVIEW"
)

type InstanceStatus string

const (
	StatusRunning   InstanceStatus = "RUNNING"
	StatusCompleted InstanceStatus = "COMPLETED"
	StatusFailed    InstanceStatus = "FAILED"
	StatusCancelled InstanceStatus = "CANCELLED"
	StatusWaiting   InstanceStatus = "WAITING_FOR_INPUT" // e.g., Human Approval
)

// --- Core Models ---

// WorkflowDefinition defines the structure of a process (mostly static config).
// Note: In Temporal, the "code is the definition", but we store metadata here
// to power the UI dashboard.
type WorkflowDefinition struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"` // e.g. "Standard Volunteer Onboarding"
	Type        WorkflowType    `json:"type" db:"type"`
	Version     int             `json:"version" db:"version"`
	Description string          `json:"description" db:"description"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	Config      json.RawMessage `json:"config" db:"config"` // e.g. { "approver_role": "manager", "timeout_hours": 48 }
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// WorkflowInstance represents a single execution of a workflow (e.g. "John Doe's Application").
type WorkflowInstance struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	WorkflowID  uuid.UUID       `json:"workflow_id" db:"workflow_id"` // Link to Definition
	TemporalID  string          `json:"temporal_id" db:"temporal_id"` // ID in Temporal
	RunID       string          `json:"run_id" db:"run_id"`           // Run ID in Temporal
	SubjectID   uuid.UUID       `json:"subject_id" db:"subject_id"`   // e.g., UserID, ExpenseID
	Status      InstanceStatus  `json:"status" db:"status"`
	CurrentStep string          `json:"current_step" db:"current_step"`
	InputData   json.RawMessage `json:"input_data" db:"input_data"`
	ResultData  json.RawMessage `json:"result_data" db:"result_data"`
	StartedAt   time.Time       `json:"started_at" db:"started_at"`
	CompletedAt *time.Time      `json:"completed_at" db:"completed_at"`
}

// ApprovalRequest represents a point where the workflow pauses for human intervention.
type ApprovalRequest struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	InstanceID   uuid.UUID  `json:"instance_id" db:"instance_id"`
	AssigneeRole string     `json:"assignee_role" db:"assignee_role"` // e.g. "finance_manager"
	AssigneeID   *uuid.UUID `json:"assignee_id" db:"assignee_id"`     // Optional specific user
	Message      string     `json:"message" db:"message"`
	Status       string     `json:"status" db:"status"` // "PENDING", "APPROVED", "REJECTED"
	DecisionNote *string    `json:"decision_note" db:"decision_note"`
	DecidedBy    *uuid.UUID `json:"decided_by" db:"decided_by"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	DecidedAt    *time.Time `json:"decided_at" db:"decided_at"`
}
