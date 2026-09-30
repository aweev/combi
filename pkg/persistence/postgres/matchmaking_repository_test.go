package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchmakingRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewMatchmakingRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	menteeID := uuid.New()
	mentorID := uuid.New()

	t.Run("Full Match Lifecycle", func(t *testing.T) {
		// 1. Create Request (Demand)
		req := &matchmaking.MatchRequest{
			ID:        uuid.New(),
			UserID:    menteeID,
			Type:      matchmaking.TypeMentorship,
			Status:    "OPEN",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Criteria:  map[string]any{"topic": "go"},
		}
		require.NoError(t, repo.CreateRequest(ctx, req))

		// 2. Upsert Candidate (Supply)
		cand := &matchmaking.MatchCandidate{
			UserID:      mentorID,
			Type:        matchmaking.TypeMentorship,
			IsAvailable: true,
			Attributes:  map[string]any{"skills": []string{"go", "rust"}},
		}
		require.NoError(t, repo.UpsertCandidate(ctx, cand))
		// Upsert again to test ID retention/update
		cand.IsAvailable = true
		require.NoError(t, repo.UpsertCandidate(ctx, cand))

		// Verify Candidate
		candidates, err := repo.FindCandidates(ctx, matchmaking.TypeMentorship, nil)
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		assert.Equal(t, mentorID, candidates[0].UserID)
		candidateID := candidates[0].ID

		// 3. Save Suggestions (Algo Result)
		reasonJSON, _ := json.Marshal(map[string]string{"match": "skill_overlap"})
		sug := matchmaking.MatchSuggestion{
			RequestID:   req.ID,
			CandidateID: candidateID,
			Score:       0.95,
			Reason:      reasonJSON,
			CreatedAt:   time.Now(),
		}
		require.NoError(t, repo.SaveSuggestions(ctx, []matchmaking.MatchSuggestion{sug}))

		// Verify Suggestions
		fetchedSugs, err := repo.GetSuggestions(ctx, req.ID)
		require.NoError(t, err)
		require.Len(t, fetchedSugs, 1)
		assert.Equal(t, 0.95, fetchedSugs[0].Score)

		// 4. Create Relationship (Match Made)
		startTime := time.Now().UTC()
		rel := &matchmaking.MatchRelationship{
			RequestID:   req.ID,
			CandidateID: candidateID,
			MenteeID:    menteeID,
			MentorID:    mentorID,
			Status:      matchmaking.StatusActive,
			StartedAt:   &startTime,
		}
		require.NoError(t, repo.CreateRelationship(ctx, rel))

		// 5. Update Relationship (Complete)
		require.NoError(t, repo.UpdateRelationshipStatus(ctx, rel.ID, matchmaking.StatusCompleted))

		// Verify EndedAt was set automatically
		updatedRel, err := repo.GetRelationship(ctx, rel.ID)
		require.NoError(t, err)
		assert.Equal(t, matchmaking.StatusCompleted, updatedRel.Status)
		assert.NotNil(t, updatedRel.EndedAt)
	})
}
