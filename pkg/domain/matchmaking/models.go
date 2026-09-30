package matchmaking

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type MatchType string

const (
	TypeMentorship MatchType = "MENTORSHIP"
	TypeVolunteer  MatchType = "VOLUNTEER_JOB"
)

type MatchStatus string

const (
	StatusProposed  MatchStatus = "PROPOSED"  // Algo suggestion visible to Admin/User
	StatusPending   MatchStatus = "PENDING"   // Invite sent, waiting for acceptance
	StatusActive    MatchStatus = "ACTIVE"    // Both parties agreed
	StatusCompleted MatchStatus = "COMPLETED" // Program duration finished
	StatusRejected  MatchStatus = "REJECTED"  // Declined by one party
	StatusDissolved MatchStatus = "DISSOLVED" // Ended prematurely
)

// --- Core Models ---

// MatchRequest represents the "Demand" (Mentee looking for Mentor).
type MatchRequest struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Type        MatchType `json:"type" db:"type"`
	Status      string    `json:"status" db:"status"` // "OPEN", "MATCHED", "CANCELLED"
	IsAvailable bool      `json:"is_available" db:"is_available"`
	// JSONB: { "topics": ["go", "k8s"], "availability": "weekends" }
	Criteria map[string]interface{} `json:"criteria" db:"criteria"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

// MatchCandidate represents the "Supply" (Mentor offering help).
// Stored separately from User Profile to allow specialized attributes/availability.
type MatchCandidate struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"` // The Mentor
	Type   MatchType `json:"type" db:"type"`

	// JSONB: { "skills": ["go", "java"], "capacity": 2 }
	Attributes map[string]any `json:"attributes" db:"attributes"`

	IsAvailable bool      `json:"is_available" db:"is_available"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// MatchSuggestion represents a scored pair.
type MatchSuggestion struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	RequestID   uuid.UUID       `json:"request_id" db:"request_id"`
	CandidateID uuid.UUID       `json:"candidate_id" db:"candidate_id"`
	Score       float64         `json:"score" db:"score"`   // 0.0 to 1.0
	Reason      json.RawMessage `json:"reason" db:"reason"` // JSON details on score breakdown
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// MatchRelationship tracks the active connection.
type MatchRelationship struct {
	ID          uuid.UUID `json:"id" db:"id"`
	RequestID   uuid.UUID `json:"request_id" db:"request_id"`
	CandidateID uuid.UUID `json:"candidate_id" db:"candidate_id"`

	// Denormalized for easier querying
	MenteeID uuid.UUID `json:"mentee_id" db:"mentee_id"`
	MentorID uuid.UUID `json:"mentor_id" db:"mentor_id"`

	Status    MatchStatus `json:"status" db:"status"`
	StartedAt *time.Time  `json:"started_at" db:"started_at"`
	EndedAt   *time.Time  `json:"ended_at" db:"ended_at"`

	// Feedback (Optional)
	MentorFeedback *string `json:"mentor_feedback" db:"mentor_feedback"`
	MenteeFeedback *string `json:"mentee_feedback" db:"mentee_feedback"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
