package workflow

import "github.com/google/uuid"

// --- Workflow Names ---
// These constants map the internal system names to the Temporal workflow function names.
const (
	ContentModerationWorkflowName   = "ContentModerationWorkflow"
	ApplicationReviewWorkflowName   = "ParticipantApplicationWorkflow"
	GrantApplicationWorkflowName    = "GrantApplicationWorkflow"
	VolunteerOnboardingWorkflowName = "VolunteerOnboardingWorkflow"
	PartnershipProposalWorkflowName = "PartnershipProposalWorkflow"
)

// --- Task Queues ---
// Grouping workflows by resource requirements or priority.
const (
	GeneralTaskQueue     = "ngo_GENERAL_QUEUE"
	ApplicationTaskQueue = "ngo_APPLICATION_QUEUE"
)

// --- Input DTOs ---

// WorkflowInput is a marker interface for workflow input parameters.
type WorkflowInput interface{}

// StartWorkflowInput is the payload received from the QueueService (Job: workflow:start).
type StartWorkflowInput struct {
	WorkflowName string                 `json:"workflow_name"`
	BusinessKey  string                 `json:"business_key"` // String representation of UUID
	Input        map[string]interface{} `json:"input"`
}

// ContentModerationInput is the input for the Content Moderation Workflow.
type ContentModerationInput struct {
	ContentID uuid.UUID
	AuthorID  uuid.UUID
	Type      string // e.g., "blog_post", "comment"
}

// ApplicationReviewInput is the input for the Participant Application Workflow.
type ApplicationReviewInput struct {
	ApplicationID uuid.UUID
	UserID        uuid.UUID
}

// GrantApplicationInput is the input for the Grant Application Workflow.
type GrantApplicationInput struct {
	ApplicationID   uuid.UUID
	RecipientUserID uuid.UUID
	Amount          int64
	Currency        string
}

// VolunteerOnboardingInput is the input for the Volunteer Onboarding Workflow.
type VolunteerOnboardingInput struct {
	UserID uuid.UUID
}

// PartnershipProposalInput is the input for the Partnership Proposal Workflow.
type PartnershipProposalInput struct {
	ProposalID    uuid.UUID
	ContactUserID uuid.UUID
	OrgName       string
	OrgSlug       string
}
