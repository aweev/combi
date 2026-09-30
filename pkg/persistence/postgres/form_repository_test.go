package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/combi/pkg/domain/form"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewFormRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Form_CRUD", func(t *testing.T) {
		fID := uuid.New()
		votingConfig := &form.VotingConfig{AllowAbstain: true, PassPct: 50}

		def := &form.FormDefinition{
			ID:           fID,
			Slug:         "board-vote-2023",
			Title:        "Board Election",
			Description:  "Vote for new members",
			Type:         form.TypeVote,
			IsActive:     true,
			IsAnonymous:  false,
			Schema:       json.RawMessage(`[{"id":"candidate","type":"RADIO"}]`),
			VotingConfig: votingConfig,
			CreatedBy:    uuid.New(),
		}

		// Create
		require.NoError(t, repo.CreateForm(ctx, def))

		// Duplicate Slug Check
		assert.Error(t, repo.CreateForm(ctx, def))

		// Get
		got, err := repo.GetForm(ctx, fID)
		require.NoError(t, err)
		assert.Equal(t, "Board Election", got.Title)
		assert.NotNil(t, got.VotingConfig)
		assert.True(t, got.VotingConfig.AllowAbstain)

		// List
		active := true
		list, err := repo.ListForms(ctx, nil, active)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 1)

		// Update
		def.Title = "Updated Election"
		require.NoError(t, repo.UpdateForm(ctx, def))

		updated, _ := repo.GetForm(ctx, fID)
		assert.Equal(t, "Updated Election", updated.Title)
	})

	t.Run("Submission_Flow", func(t *testing.T) {
		fID := uuid.New()
		repo.CreateForm(ctx, &form.FormDefinition{ID: fID, Slug: "survey", Title: "S", Type: "SURVEY", CreatedBy: uuid.New()})

		uID := uuid.New()
		sub := &form.Submission{
			ID:          uuid.New(),
			FormID:      fID,
			UserID:      &uID,
			Status:      form.StatusSubmitted,
			Data:        json.RawMessage(`{"feedback": "Great work!"}`),
			SubmittedAt: time.Now(),
		}

		// Create
		require.NoError(t, repo.CreateSubmission(ctx, sub))

		// Get
		got, err := repo.GetSubmission(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, form.StatusSubmitted, got.Status)

		// Update
		score := 95.5
		got.Score = &score
		got.Status = form.StatusApproved
		require.NoError(t, repo.UpdateSubmission(ctx, got))

		// List
		list, count, err := repo.ListSubmissions(ctx, fID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, sub.ID, list[0].ID)
	})

	t.Run("Voting_Logic", func(t *testing.T) {
		fID := uuid.New()
		repo.CreateForm(ctx, &form.FormDefinition{ID: fID, Slug: "vote-prop-1", Title: "Prop 1", Type: "VOTE", CreatedBy: uuid.New()})

		voterID := uuid.New()

		// 1. Check before voting
		hasVoted, err := repo.CheckVoteParticipation(ctx, fID, voterID)
		require.NoError(t, err)
		assert.False(t, hasVoted)

		// 2. Record vote
		require.NoError(t, repo.RecordVoteParticipation(ctx, fID, voterID))

		// 3. Check after voting
		hasVoted, err = repo.CheckVoteParticipation(ctx, fID, voterID)
		require.NoError(t, err)
		assert.True(t, hasVoted)

		// 4. Prevent double vote
		err = repo.RecordVoteParticipation(ctx, fID, voterID)
		assert.ErrorIs(t, err, form.ErrAlreadyVoted)
	})

	t.Run("Aggregation", func(t *testing.T) {
		fID := uuid.New()
		repo.CreateForm(ctx, &form.FormDefinition{ID: fID, Slug: "agg-test", Title: "Agg", Type: "SURVEY", CreatedBy: uuid.New()})

		// Create 3 submissions: 2 voted "Yes", 1 voted "No"
		// Data structure: {"vote": "Yes"}
		submissions := []string{
			`{"vote": "Yes"}`,
			`{"vote": "Yes"}`,
			`{"vote": "No"}`,
		}

		for _, data := range submissions {
			u := uuid.New()
			repo.CreateSubmission(ctx, &form.Submission{
				ID: uuid.New(), FormID: fID, UserID: &u, Status: "SUBMITTED", Data: json.RawMessage(data),
			})
		}

		// Aggregate
		results, err := repo.GetAggregateResults(ctx, fID)
		require.NoError(t, err)

		// Map should contain "Yes": 2, "No": 1
		assert.Equal(t, 2, results["Yes"])
		assert.Equal(t, 1, results["No"])
	})
}
