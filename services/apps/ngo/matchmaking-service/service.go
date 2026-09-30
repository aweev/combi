package matchmaking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ matchmaking.Service = (*service)(nil)

type service struct {
	repo      matchmaking.Repository
	algorithm matchmaking.Algorithm
	audit     audit.EventRecorder
	eventBus  events.Bus
	logger    log.Logger
}

func NewService(
	repo matchmaking.Repository,
	algo matchmaking.Algorithm,
	audit audit.EventRecorder,
	eventBus events.Bus,
	logger log.Logger,
) matchmaking.Service {
	return &service{
		repo:      repo,
		algorithm: algo,
		audit:     audit,
		eventBus:  eventBus,
		logger:    logger.With(log.String("service", "matchmaking")),
	}
}

// --- Requests ---

func (s *service) CreateRequest(ctx context.Context, req matchmaking.MatchRequest) (*matchmaking.MatchRequest, error) {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.Status = "OPEN"
	req.CreatedAt = time.Now().UTC()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
	}

	if err := s.repo.CreateRequest(ctx, &req); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Audit
	if s.audit != nil {
		evt := audit.NewEvent(req.UserID, "user", core.EventMatchRequestCreated).
			WithTarget(req.ID, "match_request", string(req.Type)).
			WithSuccess().
			Build()

		if err := s.audit.Record(ctx, evt); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}

	return &req, nil
}

func (s *service) CancelRequest(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateRequestStatus(ctx, id, "CANCELLED")
}

// --- Candidates ---

func (s *service) UpsertCandidate(ctx context.Context, c matchmaking.MatchCandidate) error {
	return s.repo.UpsertCandidate(ctx, &c)
}

// --- Matching Logic ---

func (s *service) RunMatching(ctx context.Context, requestID uuid.UUID) ([]matchmaking.MatchSuggestion, error) {
	req, err := s.repo.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	// Find candidates matching the TYPE (Mentorship/Volunteer)
	candidates, err := s.repo.FindCandidates(ctx, req.Type, nil)
	if err != nil {
		return nil, err
	}

	var suggestions []matchmaking.MatchSuggestion
	for _, c := range candidates {
		if !c.IsAvailable {
			continue
		}

		score := s.algorithm.Score(*req, c)
		// Threshold: 0.4 allows partial matches (e.g. good skills but timezone difference)
		if score > 0.4 {
			reasonJSON, err := json.Marshal(map[string]any{"score": score})
			if err != nil {
				s.logger.Error("failed to marshal score reason", err)
				continue
			}

			sug := matchmaking.MatchSuggestion{
				ID:          uuid.New(),
				RequestID:   req.ID,
				CandidateID: c.ID,
				Score:       score,
				Reason:      reasonJSON,
				CreatedAt:   time.Now().UTC(),
			}
			suggestions = append(suggestions, sug)
		}
	}

	// Save results
	if len(suggestions) > 0 {
		if err := s.repo.SaveSuggestions(ctx, suggestions); err != nil {
			return nil, err
		}

		// Notify User
		if s.eventBus != nil {
			evt, err := events.NewEvent(core.EventMatchSuggestions, req.UserID, "user", map[string]string{
				"request_id": req.ID.String(),
				"count":      fmt.Sprintf("%d", len(suggestions)),
			})
			if err != nil {
				s.logger.Error("failed to create suggestions event", err)
			} else {
				if err := s.eventBus.Publish(ctx, evt); err != nil {
					s.logger.Error("failed to publish suggestions event", err)
				}
			}
		}
	}

	return suggestions, nil
}

func (s *service) ProcessBatch(ctx context.Context, limit int) error {
	requests, err := s.repo.GetOpenRequests(ctx, limit)
	if err != nil {
		return err
	}

	s.logger.Info("Running batch matching", log.Int("count", len(requests)))

	for _, req := range requests {
		// Run matching for each request.
		// We ignore individual errors to ensure the batch proceeds.
		if _, err := s.RunMatching(ctx, req.ID); err != nil {
			s.logger.Error("Failed to match request", err, log.String("req_id", req.ID.String()))
		}
	}
	return nil
}

func (s *service) GetSuggestions(ctx context.Context, requestID uuid.UUID) ([]matchmaking.MatchSuggestion, error) {
	return s.repo.GetSuggestions(ctx, requestID)
}

// --- Lifecycle ---

func (s *service) ProposeMatch(ctx context.Context, requestID, candidateID uuid.UUID) (*matchmaking.MatchRelationship, error) {
	req, err := s.repo.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	// We need candidate details to get the Mentor UserID
	// Optimization: Add GetCandidate(ID) to repo, or reuse FindCandidates filter
	candidates, err := s.repo.FindCandidates(ctx, req.Type, nil)
	// Fix: internal/services/matchmaking/service.go:164:14: ineffectual assignment to err (ineffassign)
	if err != nil {
		return nil, err
	}

	// Real implementation should filter by ID in SQL, but for now we filter in memory or assume repo support
	var candidate *matchmaking.MatchCandidate
	for _, c := range candidates {
		if c.ID == candidateID {
			candidate = &c
			break
		}
	}
	if candidate == nil {
		return nil, matchmaking.ErrMatchNotFound
	}

	rel := &matchmaking.MatchRelationship{
		ID:          uuid.New(),
		RequestID:   requestID,
		CandidateID: candidateID,
		MenteeID:    req.UserID,
		MentorID:    candidate.UserID,
		Status:      matchmaking.StatusPending,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateRelationship(ctx, rel); err != nil {
		return nil, err
	}

	// Notify
	if s.eventBus != nil {
		evt, err := events.NewEvent("match.proposed", candidate.UserID, "user", rel)
		if err != nil {
			s.logger.Error("failed to create match proposed event", err)
		} else {
			if err := s.eventBus.Publish(ctx, evt); err != nil {
				s.logger.Error("failed to publish match proposed event", err)
			}
		}
	}

	return rel, nil
}

func (s *service) AcceptMatch(ctx context.Context, matchID, actorID uuid.UUID) error {
	rel, err := s.repo.GetRelationship(ctx, matchID)
	if err != nil {
		return err
	}

	if actorID != rel.MentorID && actorID != rel.MenteeID {
		return matchmaking.ErrForbidden
	}

	if err := s.repo.UpdateRelationshipStatus(ctx, matchID, matchmaking.StatusActive); err != nil {
		return err
	}

	// Also close the request
	// Fix: Checked return value
	if err := s.repo.UpdateRequestStatus(ctx, rel.RequestID, "MATCHED"); err != nil {
		s.logger.Error("failed to update request status after match acceptance", err, log.String("req_id", rel.RequestID.String()))
		// Proceeding despite error, as the match relationship is already active
	}

	// Emit Event
	if s.eventBus != nil {
		evt, err := events.NewEvent(core.EventMatchEstablished, rel.ID, "match", map[string]string{
			"mentee_id": rel.MenteeID.String(),
			"mentor_id": rel.MentorID.String(),
		})
		if err != nil {
			s.logger.Error("failed to create match established event", err)
		} else {
			if err := s.eventBus.Publish(ctx, evt); err != nil {
				s.logger.Error("failed to publish match established event", err)
			}
		}
	}

	return nil
}

func (s *service) RejectMatch(ctx context.Context, matchID, actorID uuid.UUID) error {
	return s.repo.UpdateRelationshipStatus(ctx, matchID, matchmaking.StatusRejected)
}

func (s *service) CompleteMatch(ctx context.Context, matchID uuid.UUID) error {
	return s.repo.UpdateRelationshipStatus(ctx, matchID, matchmaking.StatusCompleted)
}
