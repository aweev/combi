package algorithm

import (
	"math"
	"strings"

	"github.com/combi/pkg/domain/matchmaking"
)

// WeightedScorer implements a flexible scoring logic based on configurable weights.
type WeightedScorer struct {
	Weights Weights
}

type Weights struct {
	SkillOverlap float64 // e.g. 0.5 (50% of score)
	Language     float64 // e.g. 0.3
	Timezone     float64 // e.g. 0.2
}

func NewWeightedScorer() *WeightedScorer {
	return &WeightedScorer{
		Weights: Weights{
			SkillOverlap: 0.5,
			Language:     0.3,
			Timezone:     0.2,
		},
	}
}

func (s *WeightedScorer) Score(req matchmaking.MatchRequest, cand matchmaking.MatchCandidate) float64 {
	// 1. Hard Constraints (Must be met, otherwise 0)
	if !cand.IsAvailable {
		return 0.0
	}
	// Check Type (e.g. Mentorship vs Volunteer) - usually filtered by DB query, but good to double check
	if req.Type != cand.Type {
		return 0.0
	}

	totalScore := 0.0

	// 2. Skill/Topic Matching (Jaccard Similarity)
	// Request: { "topics": ["go", "k8s"] }
	// Candidate: { "skills": ["go", "rust", "k8s", "java"] }
	reqTopics := extractStringSlice(req.Criteria, "topics")
	candSkills := extractStringSlice(cand.Attributes, "skills")

	skillScore := calculateJaccardIndex(reqTopics, candSkills)
	totalScore += skillScore * s.Weights.SkillOverlap

	// 3. Language Matching (Boolean or Overlap)
	// Request: { "language": "en" } or ["en", "de"]
	// Candidate: { "languages": ["en", "fr"] }
	reqLangs := extractStringSlice(req.Criteria, "languages")
	// Fallback to single string if array not found
	if len(reqLangs) == 0 {
		if l, ok := req.Criteria["language"].(string); ok && l != "" {
			reqLangs = []string{l}
		}
	}

	candLangs := extractStringSlice(cand.Attributes, "languages")

	langScore := 0.0
	if len(reqLangs) == 0 {
		langScore = 1.0 // No preference = Match all
	} else {
		// If they share ANY language, it's a match (1.0), otherwise 0.
		// Alternatively, Jaccard if multiple languages matter.
		if hasIntersection(reqLangs, candLangs) {
			langScore = 1.0
		}
	}
	totalScore += langScore * s.Weights.Language

	// 4. Timezone Affinity (1.0 = Same hour, 0.0 = 12 hours apart)
	// Request: { "timezone_offset": 3 } (UTC+3)
	// Candidate: { "timezone_offset": 5 } (UTC+5)
	reqTZ, ok1 := getNumber(req.Criteria, "timezone_offset")
	candTZ, ok2 := getNumber(cand.Attributes, "timezone_offset")

	tzScore := 0.5 // Neutral default
	if ok1 && ok2 {
		diff := math.Abs(reqTZ - candTZ)
		// Normalize: 0 diff -> 1.0, 12 diff -> 0.0
		// Formula: Max(0, 1 - (diff / 12))
		tzScore = math.Max(0, 1.0-(diff/12.0))
	}
	totalScore += tzScore * s.Weights.Timezone

	return totalScore
}

// --- Helpers ---

// extractStringSlice safely parses a []string or []interface{} from a map
func extractStringSlice(m map[string]any, key string) []string {
	val, ok := m[key]
	if !ok {
		return []string{}
	}

	// Case 1: Already []string
	if strs, ok := val.([]string); ok {
		return normalize(strs)
	}

	// Case 2: []interface{} (from JSON unmarshal)
	if interfaces, ok := val.([]interface{}); ok {
		strs := make([]string, 0, len(interfaces))
		for _, i := range interfaces {
			if s, ok := i.(string); ok {
				strs = append(strs, s)
			}
		}
		return normalize(strs)
	}

	return []string{}
}

func normalize(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(strings.TrimSpace(s))
	}
	return out
}

func getNumber(m map[string]any, key string) (float64, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	// JSON numbers are often float64
	if f, ok := val.(float64); ok {
		return f, true
	}
	if i, ok := val.(int); ok {
		return float64(i), true
	}
	return 0, false
}

// calculateJaccardIndex = Intersection / Union
func calculateJaccardIndex(setA, setB []string) float64 {
	if len(setA) == 0 {
		return 0.0 // Request has no specific topic needs? Maybe 1.0?
		// Usually if Mentee asks for nothing, matching is low quality.
	}
	if len(setB) == 0 {
		return 0.0
	}

	intersection := 0
	unionMap := make(map[string]bool)

	for _, a := range setA {
		unionMap[a] = true
	}
	for _, b := range setB {
		if unionMap[b] {
			intersection++
		}
		unionMap[b] = true
	}

	unionSize := len(unionMap)
	if unionSize == 0 {
		return 0.0
	}

	return float64(intersection) / float64(unionSize)
}

func hasIntersection(setA, setB []string) bool {
	m := make(map[string]bool)
	for _, a := range setA {
		m[a] = true
	}
	for _, b := range setB {
		if m[b] {
			return true
		}
	}
	return false
}
