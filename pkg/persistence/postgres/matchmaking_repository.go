package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/matchmaking"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ matchmaking.Repository = (*MatchmakingRepository)(nil)

type MatchmakingRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewMatchmakingRepository(db *sqlx.DB, logger log.Logger) *MatchmakingRepository {
	return &MatchmakingRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "matchmaking_repo")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx implementation
func (r *MatchmakingRepository) WithTx(ctx context.Context, fn func(txRepo matchmaking.Repository) error) error {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return fn(r)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	// Fix: internal/persistence/postgres/matchmaking_repository.go:45:19: Error return value of tx.Rollback is not checked
	defer func() {
		_ = tx.Rollback()
	}()

	txRepo := &MatchmakingRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Requests ---

func (r *MatchmakingRepository) CreateRequest(ctx context.Context, req *matchmaking.MatchRequest) error {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}
	if len(req.Criteria) == 0 {
		req.Criteria = map[string]any{}
	}

	// FIX: Marshal map to JSON for sqlx
	criteriaJSON, err := json.Marshal(req.Criteria)
	if err != nil {
		return fmt.Errorf("failed to marshal request criteria: %w", err)
	}

	// Use a struct that sqlx can map easily, or pass arguments manually
	// Passing manually is safer here to control the JSON bytes
	query := `
		INSERT INTO match_requests (id, user_id, type, status, criteria, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	// Using ExecContext with positional args instead of NamedExec to pass the []byte
	_, err = r.runner.ExecContext(ctx, query,
		req.ID, req.UserID, req.Type, req.Status, criteriaJSON, req.CreatedAt, req.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create match request: %w", err)
	}
	return nil
}

func (r *MatchmakingRepository) GetRequest(ctx context.Context, id uuid.UUID) (*matchmaking.MatchRequest, error) {
	// We fetch into a temporary struct that uses []byte for JSON columns
	var dbReq struct {
		matchmaking.MatchRequest
		Criteria []byte `db:"criteria"`
	}

	query := `SELECT * FROM match_requests WHERE id = $1`
	if err := sqlx.GetContext(ctx, r.runner, &dbReq, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, matchmaking.ErrRequestNotFound
		}
		return nil, err
	}

	// Unmarshal back to map
	req := dbReq.MatchRequest
	if len(dbReq.Criteria) > 0 {
		// Fix: Check unmarshal error
		if err := json.Unmarshal(dbReq.Criteria, &req.Criteria); err != nil {
			return nil, fmt.Errorf("failed to unmarshal request criteria: %w", err)
		}
	}
	return &req, nil
}

func (r *MatchmakingRepository) UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.runner.ExecContext(ctx, "UPDATE match_requests SET status = $1 WHERE id = $2", status, id)
	return err
}

func (r *MatchmakingRepository) GetOpenRequests(ctx context.Context, limit int) ([]matchmaking.MatchRequest, error) {
	var reqs []matchmaking.MatchRequest
	query := `SELECT * FROM match_requests WHERE status = 'OPEN' ORDER BY created_at ASC LIMIT $1`
	if err := sqlx.SelectContext(ctx, r.runner, &reqs, query, limit); err != nil {
		return nil, err
	}
	return reqs, nil
}

// --- Candidates ---

func (r *MatchmakingRepository) UpsertCandidate(ctx context.Context, c *matchmaking.MatchCandidate) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	c.UpdatedAt = time.Now().UTC()

	// FIX: Marshal map
	attrJSON, err := json.Marshal(c.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	query := `
		INSERT INTO match_candidates (id, user_id, type, attributes, is_available, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, type) DO UPDATE SET
			attributes = EXCLUDED.attributes,
			is_available = EXCLUDED.is_available,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	var id uuid.UUID
	err = r.runner.QueryRowxContext(ctx, query,
		c.ID, c.UserID, c.Type, attrJSON, c.IsAvailable, c.UpdatedAt,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to upsert candidate: %w", err)
	}
	c.ID = id
	return nil
}

func (r *MatchmakingRepository) FindCandidates(ctx context.Context, mType matchmaking.MatchType, criteria map[string]any) ([]matchmaking.MatchCandidate, error) {

	builder := r.sq.Select("*").From("match_candidates").
		Where(squirrel.Eq{"type": mType, "is_available": true})

	// Fix: Check ToSql error
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build candidates query: %w", err)
	}

	// Use temporary struct for scanning
	var rows []struct {
		matchmaking.MatchCandidate
		Attributes []byte `db:"attributes"`
	}

	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("failed to find candidates: %w", err)
	}

	candidates := make([]matchmaking.MatchCandidate, len(rows))
	for i, row := range rows {
		candidates[i] = row.MatchCandidate
		if len(row.Attributes) > 0 {
			// Fix: Check unmarshal error
			if err := json.Unmarshal(row.Attributes, &candidates[i].Attributes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal attributes for candidate %s: %w", row.ID, err)
			}
		}
	}

	return candidates, nil
}

// --- Suggestions ---

func (r *MatchmakingRepository) SaveSuggestions(ctx context.Context, suggestions []matchmaking.MatchSuggestion) error {
	if len(suggestions) == 0 {
		return nil
	}

	// Bulk Insert
	query := `
		INSERT INTO match_suggestions (request_id, candidate_id, score, reason, created_at)
		VALUES (:request_id, :candidate_id, :score, :reason, :created_at)
		ON CONFLICT (request_id, candidate_id) DO UPDATE SET
			score = EXCLUDED.score,
			reason = EXCLUDED.reason,
			created_at = EXCLUDED.created_at
	`
	_, err := sqlx.NamedExecContext(ctx, r.runner, query, suggestions)
	if err != nil {
		return fmt.Errorf("failed to save suggestions: %w", err)
	}
	return nil
}

func (r *MatchmakingRepository) GetSuggestions(ctx context.Context, requestID uuid.UUID) ([]matchmaking.MatchSuggestion, error) {
	var sugs []matchmaking.MatchSuggestion
	query := `SELECT * FROM match_suggestions WHERE request_id = $1 ORDER BY score DESC`
	if err := sqlx.SelectContext(ctx, r.runner, &sugs, query, requestID); err != nil {
		return nil, err
	}
	return sugs, nil
}

// --- Relationships ---

func (r *MatchmakingRepository) CreateRelationship(ctx context.Context, rel *matchmaking.MatchRelationship) error {
	if rel.ID == uuid.Nil {
		rel.ID = uuid.New()
	}
	rel.CreatedAt = time.Now().UTC()

	query := `
		INSERT INTO match_relationships 
		(id, request_id, candidate_id, mentee_id, mentor_id, status, started_at, created_at)
		VALUES 
		(:id, :request_id, :candidate_id, :mentee_id, :mentor_id, :status, :started_at, :created_at)
	`
	_, err := sqlx.NamedExecContext(ctx, r.runner, query, rel)
	if err != nil {
		return fmt.Errorf("failed to create relationship: %w", err)
	}
	return nil
}

func (r *MatchmakingRepository) GetRelationship(ctx context.Context, id uuid.UUID) (*matchmaking.MatchRelationship, error) {
	var rel matchmaking.MatchRelationship
	query := `SELECT * FROM match_relationships WHERE id = $1`
	if err := sqlx.GetContext(ctx, r.runner, &rel, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, matchmaking.ErrMatchNotFound
		}
		return nil, err
	}
	return &rel, nil
}

func (r *MatchmakingRepository) UpdateRelationshipStatus(ctx context.Context, id uuid.UUID, status matchmaking.MatchStatus) error {
	// Handle End Date Logic automatically
	var endedAt *time.Time
	if status == matchmaking.StatusCompleted || status == matchmaking.StatusDissolved || status == matchmaking.StatusRejected {
		now := time.Now().UTC()
		endedAt = &now
	}

	query := `
		UPDATE match_relationships 
		SET status = $1, ended_at = COALESCE($2, ended_at)
		WHERE id = $3
	`
	_, err := r.runner.ExecContext(ctx, query, status, endedAt, id)
	return err
}
