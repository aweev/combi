package form

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 400 Bad Request
	ErrInvalidSubmission = apperror.New(http.StatusBadRequest, "INVALID_SUBMISSION", "The submission data does not match the form schema.", nil)
	ErrFormExpired       = apperror.New(http.StatusBadRequest, "FORM_EXPIRED", "This form is no longer accepting submissions.", nil)
	ErrAlreadyVoted      = apperror.New(http.StatusBadRequest, "ALREADY_VOTED", "You have already cast a vote for this item.", nil)

	ErrInvalidInput = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Invalid input.", nil)
	ErrAccessDenied = apperror.New(http.StatusForbidden, "ACCESS_DENIED", "Access denied.", nil)

	// 404 Not Found
	ErrFormNotFound       = apperror.New(http.StatusNotFound, "FORM_NOT_FOUND", "The specified form could not be found.", nil)
	ErrSubmissionNotFound = apperror.New(http.StatusNotFound, "SUBMISSION_NOT_FOUND", "The specified submission could not be found.", nil)

	// 403 Forbidden
	ErrVotingClosed = apperror.New(http.StatusForbidden, "VOTING_CLOSED", "Voting for this item has ended.", nil)
)

const (
	PermFormManage      core.PermissionName = "form:manage"
	PermFormViewResults core.PermissionName = "form:view_results"
	PermVoteBoard       core.PermissionName = "vote:board"
)

type SubmissionCreatedPayload struct {
	SubmissionID uuid.UUID      `json:"submission_id"`
	FormID       uuid.UUID      `json:"form_id"`
	UserID       *uuid.UUID     `json:"user_id,omitempty"` // Anonymous allowed
	Data         map[string]any `json:"data"`              // The answers
	SubmittedAt  time.Time      `json:"submitted_at"`
}

// --- Enums ---

type FormType string

const (
	TypeApplication FormType = "APPLICATION" // Linked to Catalog Offering
	TypeSurvey      FormType = "SURVEY"      // General feedback
	TypeVote        FormType = "VOTE"        // Governance/Board voting
	TypeContact     FormType = "CONTACT"     // Simple contact form
)

type FieldType string

const (
	FieldText     FieldType = "TEXT"
	FieldTextArea FieldType = "TEXTAREA"
	FieldSelect   FieldType = "SELECT"
	FieldMulti    FieldType = "MULTI_SELECT"
	FieldRadio    FieldType = "RADIO"
	FieldFile     FieldType = "FILE" // Uploads
	FieldDate     FieldType = "DATE"
	FieldRating   FieldType = "RATING" // 1-5 stars
)

type SubmissionStatus string

const (
	StatusDraft     SubmissionStatus = "DRAFT"
	StatusSubmitted SubmissionStatus = "SUBMITTED"
	StatusReviewed  SubmissionStatus = "REVIEWED"
	StatusApproved  SubmissionStatus = "APPROVED"
	StatusRejected  SubmissionStatus = "REJECTED"
)

// --- Core Models ---

// FormDefinition defines the structure and rules of a form.
type FormDefinition struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Slug        string    `json:"slug" db:"slug"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Type        FormType  `json:"type" db:"type"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	IsAnonymous bool      `json:"is_anonymous" db:"is_anonymous"` // If true, UserID is not stored

	// Schema defines the fields. Stored as JSONB for flexibility.
	// Structure: [ { "id": "q1", "type": "TEXT", "label": "Name", "required": true, "options": [] } ]
	Schema json.RawMessage `json:"schema" db:"schema"`

	// Config for Voting (if Type == VOTE)
	VotingConfig *VotingConfig `json:"voting_config,omitempty" db:"voting_config"`

	ExpiresAt *time.Time `json:"expires_at" db:"expires_at"`
	CreatedBy uuid.UUID  `json:"created_by" db:"created_by"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

type VotingConfig struct {
	AllowAbstain bool `json:"allow_abstain"`
	QuorumPct    int  `json:"quorum_pct"` // Percentage required for validity
	PassPct      int  `json:"pass_pct"`   // Percentage required to pass
}

// Submission represents a user's completed form.
type Submission struct {
	ID     uuid.UUID        `json:"id" db:"id"`
	FormID uuid.UUID        `json:"form_id" db:"form_id"`
	UserID *uuid.UUID       `json:"user_id" db:"user_id"` // Nullable for anonymous
	Status SubmissionStatus `json:"status" db:"status"`

	// The answers. Keyed by Field ID from the schema.
	// { "q1": "John Doe", "q2": ["Option A"] }
	Data json.RawMessage `json:"data" db:"data"`

	// Computed score (for quizzes or weighted applications)
	Score *float64 `json:"score,omitempty" db:"score"`

	SubmittedAt time.Time `json:"submitted_at" db:"submitted_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// VoteRecord tracks *that* a user voted, but separates identity from the choice for anonymity (if configured).
// Note: For Board Voting, we often WANT transparency, so anonymity is a config toggle.
type VoteRecord struct {
	FormID  uuid.UUID `json:"form_id" db:"form_id"`
	UserID  uuid.UUID `json:"user_id" db:"user_id"`
	VotedAt time.Time `json:"voted_at" db:"voted_at"`
}

// Service is the primary interface used by other domains.
type Service interface {
	// Definition Management
	CreateForm(ctx context.Context, def FormDefinition) (*FormDefinition, error)
	GetForm(ctx context.Context, id uuid.UUID) (*FormDefinition, error)
	GetFormBySlug(ctx context.Context, slug string) (*FormDefinition, error)
	ListForms(ctx context.Context, fType *FormType, activeOnly bool) ([]FormDefinition, error)
	UpdateForm(ctx context.Context, def FormDefinition) (*FormDefinition, error)

	// Submission Handling
	Submit(ctx context.Context, formID uuid.UUID, userID *uuid.UUID, data map[string]any) (*Submission, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (*Submission, error)
	ListSubmissions(ctx context.Context, formID uuid.UUID, limit, offset int) ([]Submission, int, error)
	UpdateSubmissionStatus(ctx context.Context, id uuid.UUID, status SubmissionStatus) error

	// Voting Specifics
	CastVote(ctx context.Context, formID uuid.UUID, userID uuid.UUID, data map[string]any) error
	GetVoteResults(ctx context.Context, formID uuid.UUID) (*VoteResultSummary, error)
	HasVoted(ctx context.Context, formID, userID uuid.UUID) (bool, error)
}

// Repository defines the persistence contract.
type Repository interface {
	// Forms
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error
	CreateForm(ctx context.Context, f *FormDefinition) error
	GetForm(ctx context.Context, id uuid.UUID) (*FormDefinition, error)
	GetFormBySlug(ctx context.Context, slug string) (*FormDefinition, error)
	ListForms(ctx context.Context, fType *FormType, activeOnly bool) ([]FormDefinition, error)
	UpdateForm(ctx context.Context, f *FormDefinition) error

	// Submissions
	CreateSubmission(ctx context.Context, s *Submission) error
	GetSubmission(ctx context.Context, id uuid.UUID) (*Submission, error)
	ListSubmissions(ctx context.Context, formID uuid.UUID, limit, offset int) ([]Submission, int, error)
	UpdateSubmission(ctx context.Context, s *Submission) error

	// Voting
	RecordVoteParticipation(ctx context.Context, formID, userID uuid.UUID) error
	CheckVoteParticipation(ctx context.Context, formID, userID uuid.UUID) (bool, error)
	// For anonymous votes, results are stored aggregated or separated from UserID
	GetAggregateResults(ctx context.Context, formID uuid.UUID) (map[string]int, error)
}

// DTOs
type VoteResultSummary struct {
	TotalVotes int
	Results    map[string]int // "Option A": 10, "Option B": 5
	QuorumMet  bool
	Passed     bool
}
