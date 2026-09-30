package matchmaking

import (
	"context"

	"github.com/google/uuid"
)

// Service is the primary interface used by other domains.
type Service interface {
	// Request Management
	CreateRequest(ctx context.Context, req MatchRequest) (*MatchRequest, error)
	CancelRequest(ctx context.Context, id uuid.UUID) error

	// Candidate Management
	UpsertCandidate(ctx context.Context, c MatchCandidate) error

	// Matching Logic
	RunMatching(ctx context.Context, requestID uuid.UUID) ([]MatchSuggestion, error)
	GetSuggestions(ctx context.Context, requestID uuid.UUID) ([]MatchSuggestion, error)

	// Batch Processing (For Worker)
	ProcessBatch(ctx context.Context, limit int) error

	// Relationship Lifecycle
	ProposeMatch(ctx context.Context, requestID, candidateID uuid.UUID) (*MatchRelationship, error)
	AcceptMatch(ctx context.Context, matchID, actorID uuid.UUID) error
	RejectMatch(ctx context.Context, matchID, actorID uuid.UUID) error
	CompleteMatch(ctx context.Context, matchID uuid.UUID) error
}

// Repository defines the persistence contract.
type Repository interface {
	// Requests
	CreateRequest(ctx context.Context, r *MatchRequest) error
	GetRequest(ctx context.Context, id uuid.UUID) (*MatchRequest, error)
	UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) error
	GetOpenRequests(ctx context.Context, limit int) ([]MatchRequest, error)

	// Candidates
	UpsertCandidate(ctx context.Context, c *MatchCandidate) error
	FindCandidates(ctx context.Context, mType MatchType, criteria map[string]any) ([]MatchCandidate, error)

	// Suggestions
	SaveSuggestions(ctx context.Context, suggestions []MatchSuggestion) error
	GetSuggestions(ctx context.Context, requestID uuid.UUID) ([]MatchSuggestion, error)

	// Relationships
	CreateRelationship(ctx context.Context, r *MatchRelationship) error
	GetRelationship(ctx context.Context, id uuid.UUID) (*MatchRelationship, error)
	UpdateRelationshipStatus(ctx context.Context, id uuid.UUID, status MatchStatus) error
}

// Algorithm defines the contract for scoring logic.
// Implementations might be simple (tag overlap) or complex (ML models).
type Algorithm interface {
	Score(req MatchRequest, candidate MatchCandidate) float64
}
