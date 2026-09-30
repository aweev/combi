package workflow

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the administrative and operational contract.
type Service interface {
	// Definition Management (Configuring the flows)
	CreateDefinition(ctx context.Context, params CreateDefinitionParams) (*WorkflowDefinition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*WorkflowDefinition, error)
	ListDefinitions(ctx context.Context, activeOnly bool) ([]WorkflowDefinition, error)

	// Execution (Running the flows)
	StartWorkflow(ctx context.Context, params StartWorkflowParams) (*WorkflowInstance, error)
	CancelWorkflow(ctx context.Context, instanceID uuid.UUID, reason string) error
	SignalWorkflow(ctx context.Context, instanceID uuid.UUID, signalName string, data any) error

	// Visibility
	GetInstance(ctx context.Context, id uuid.UUID) (*WorkflowInstance, error)
	ListInstances(ctx context.Context, filter InstanceFilter) ([]WorkflowInstance, int, error)

	// Human Interaction (Approvals)
	GetApprovalRequest(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error)
	ListPendingApprovals(ctx context.Context, assigneeID uuid.UUID, role string) ([]ApprovalRequest, error)
	SubmitApproval(ctx context.Context, requestID uuid.UUID, params DecideApprovalParams) error
}

// Repository defines the persistence contract (Postgres mirror of Temporal).
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	// Definitions
	CreateDefinition(ctx context.Context, d *WorkflowDefinition) error
	GetDefinition(ctx context.Context, id uuid.UUID) (*WorkflowDefinition, error)
	ListDefinitions(ctx context.Context, activeOnly bool) ([]WorkflowDefinition, error)

	// Instances
	CreateInstance(ctx context.Context, i *WorkflowInstance) error
	GetInstance(ctx context.Context, id uuid.UUID) (*WorkflowInstance, error)
	UpdateInstanceStatus(ctx context.Context, id uuid.UUID, status InstanceStatus, currentStep string) error
	ListInstances(ctx context.Context, filter InstanceFilter) ([]WorkflowInstance, int, error)

	// Approvals
	CreateApprovalRequest(ctx context.Context, r *ApprovalRequest) error
	GetApprovalRequest(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error)
	UpdateApprovalRequest(ctx context.Context, r *ApprovalRequest) error
	ListPendingApprovals(ctx context.Context, assigneeID *uuid.UUID, role string) ([]ApprovalRequest, error)
}

// TemporalClient abstracts the temporal.io SDK.
type TemporalClient interface {
	ExecuteWorkflow(ctx context.Context, options any, workflow any, args ...any) (any, error)
	SignalWorkflow(ctx context.Context, workflowID, runID, signalName string, arg any) error
	CancelWorkflow(ctx context.Context, workflowID, runID string) error
}

// --- DTOs ---

type CreateDefinitionParams struct {
	Name        string
	Type        WorkflowType
	Description string
	Config      map[string]any
}

type StartWorkflowParams struct {
	WorkflowDefinitionID uuid.UUID      `validate:"required"`
	SubjectID            uuid.UUID      `validate:"required"` // The entity (User, Expense) this is about
	Input                map[string]any `validate:"required"`
}

type DecideApprovalParams struct {
	DeciderID uuid.UUID `validate:"required"`
	Approved  bool      `validate:"required"`
	Note      string
}

type InstanceFilter struct {
	Type      *WorkflowType
	Status    *InstanceStatus
	SubjectID *uuid.UUID
	Limit     int
	Offset    int
}
