package algorithm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/matchmaking/algorithm"
)

func TestWeightedScorer(t *testing.T) {
	scorer := algorithm.NewWeightedScorer()

	// Base Request
	req := matchmaking.MatchRequest{
		Type: matchmaking.TypeMentorship,
		Criteria: map[string]any{
			"topics":          []string{"Go", "Kubernetes"},
			"languages":       []string{"en"},
			"timezone_offset": 2.0, // UTC+2 (Berlin)
		},
	}

	t.Run("Perfect Match", func(t *testing.T) {
		cand := matchmaking.MatchCandidate{
			Type:        matchmaking.TypeMentorship,
			IsAvailable: true,
			Attributes: map[string]any{
				"skills":          []string{"go", "kubernetes", "linux"},
				"languages":       []string{"en", "de"},
				"timezone_offset": 2.0,
			},
		}
		score := scorer.Score(req, cand)
		assert.InDelta(t, 0.833, score, 0.01)
	})

	t.Run("Zero Availability", func(t *testing.T) {
		cand := matchmaking.MatchCandidate{
			Type:        matchmaking.TypeMentorship, // Type matches...
			IsAvailable: false,                      // ...but unavailable
			Attributes:  map[string]any{"skills": []string{"go"}},
		}
		assert.Equal(t, 0.0, scorer.Score(req, cand))
	})

	t.Run("Timezone Mismatch", func(t *testing.T) {
		cand := matchmaking.MatchCandidate{
			Type:        matchmaking.TypeMentorship, // <--- FIX: Added Type
			IsAvailable: true,
			Attributes: map[string]any{
				"skills":          []string{"go"}, // 0.5 overlap -> 0.25 score
				"languages":       []string{"en"}, // Match -> 0.3 score
				"timezone_offset": -8.0,           // San Francisco -> ~0.033 score
			},
		}

		score := scorer.Score(req, cand)

		// Expected: ~0.583
		assert.Greater(t, score, 0.5, "Score should be > 0.5")
		assert.Less(t, score, 0.6, "Score should be < 0.6")
	})
}
