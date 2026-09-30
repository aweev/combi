package matchmaking_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/matchmaking"
	matchmaking_mocks "github.com/combi/pkg/domain/matchmaking/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	matchmaking_svc "github.com/combi/matchmaking"
	"github.com/combi/matchmaking/algorithm"
)

func TestService_RunMatching(t *testing.T) {
	repo := matchmaking_mocks.NewMockRepository(t)
	bus := events_mocks.NewMockBus(t)
	matcher := algorithm.NewWeightedScorer()

	// NewService(repo, algo, audit, bus, logger)
	svc := matchmaking_svc.NewService(repo, matcher, nil, bus, &log.NoOpLogger{})
	ctx := context.Background()

	requestID := uuid.New()
	userID := uuid.New()

	req := &matchmaking.MatchRequest{
		ID:     requestID,
		UserID: userID,
		Type:   matchmaking.TypeMentorship,
		Status: "OPEN",
		Criteria: map[string]any{
			"topics": []string{"golang"},
		},
	}

	// Candidate with IsAvailable=true is CRITICAL for scoring
	candidateGood := matchmaking.MatchCandidate{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		Type:        matchmaking.TypeMentorship,
		IsAvailable: true,
		Attributes: map[string]any{
			"skills": []string{"golang"},
		},
	}

	repo.EXPECT().GetRequest(ctx, requestID).Return(req, nil)
	repo.EXPECT().FindCandidates(ctx, matchmaking.TypeMentorship, mock.Anything).Return([]matchmaking.MatchCandidate{candidateGood}, nil)

	// Expect SaveSuggestions
	repo.EXPECT().SaveSuggestions(ctx, mock.MatchedBy(func(sugs []matchmaking.MatchSuggestion) bool {
		return len(sugs) == 1 && sugs[0].CandidateID == candidateGood.ID
	})).Return(nil)

	// Expect Event
	bus.EXPECT().Publish(ctx, mock.Anything).Return(nil)

	suggestions, err := svc.RunMatching(ctx, requestID)
	require.NoError(t, err)
	assert.Len(t, suggestions, 1)
}

func TestService_AcceptMatch(t *testing.T) {
	repo := matchmaking_mocks.NewMockRepository(t)
	bus := events_mocks.NewMockBus(t) // Use the mock bus
	algo := algorithm.NewWeightedScorer()
	audit := audit_mocks.NewMockEventRecorder(t)
	svc := matchmaking_svc.NewService(repo, algo, audit, bus, &log.NoOpLogger{})
	ctx := context.Background()

	matchID := uuid.New()
	menteeID := uuid.New()
	mentorID := uuid.New()
	reqID := uuid.New()

	rel := &matchmaking.MatchRelationship{
		ID:        matchID,
		RequestID: reqID,
		MenteeID:  menteeID,
		MentorID:  mentorID,
		Status:    matchmaking.StatusPending,
	}

	// 1. Success Path (Mentee Accepts)
	repo.EXPECT().GetRelationship(ctx, matchID).Return(rel, nil).Once()
	repo.EXPECT().UpdateRelationshipStatus(ctx, matchID, matchmaking.StatusActive).Return(nil).Once()
	repo.EXPECT().UpdateRequestStatus(ctx, reqID, "MATCHED").Return(nil).Once()

	// FIX: Expect Event Emission
	bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()

	err := svc.AcceptMatch(ctx, matchID, menteeID)
	require.NoError(t, err)

	// 2. Failure Path (Wrong User)
	randomUser := uuid.New()
	repo.EXPECT().GetRelationship(ctx, matchID).Return(rel, nil).Once()
	// Bus should NOT be called here

	err = svc.AcceptMatch(ctx, matchID, randomUser)
	// FIX: Update error string expectation case
	assert.ErrorContains(t, err, "FORBIDDEN")
}
