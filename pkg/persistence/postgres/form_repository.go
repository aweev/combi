package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/form"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ form.Repository = (*FormRepository)(nil)

type FormRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewFormRepository(db *sqlx.DB, logger log.Logger) *FormRepository {
	return &FormRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "form_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *FormRepository) WithTx(ctx context.Context, fn func(txRepo form.Repository) error) error {
	// 1. Begin Transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// 2. Create a shallow copy of the repository using the transaction as the runner
	// This ensures all queries inside 'fn' use the transaction.
	txRepo := &FormRepository{
		db:     r.db,
		runner: tx, // <--- Key change: uses the Tx, not the DB pool
		logger: r.logger,
		sq:     r.sq,
	}

	// 3. Execute the business logic
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}

	// 4. Commit
	return tx.Commit()
}

// -----------------------------------------------------------------------------
// Form Definitions
// -----------------------------------------------------------------------------

func (r *FormRepository) CreateForm(ctx context.Context, f *form.FormDefinition) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	f.UpdatedAt = f.CreatedAt

	dbF, err := toDBForm(f)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("form_definitions").
		Columns("id", "slug", "title", "description", "type", "is_active", "is_anonymous",
			"schema", "voting_config", "expires_at", "created_by", "created_at", "updated_at").
		Values(dbF.ID, dbF.Slug, dbF.Title, dbF.Description, dbF.Type, dbF.IsActive, dbF.IsAnonymous,
			dbF.Schema, dbF.VotingConfig, dbF.ExpiresAt, dbF.CreatedBy, dbF.CreatedAt, dbF.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isFormUniqueViolation(err) {
			return fmt.Errorf("slug already exists: %w", err)
		}
		return err
	}
	return nil
}

func (r *FormRepository) GetForm(ctx context.Context, id uuid.UUID) (*form.FormDefinition, error) {
	query, args, _ := r.sq.Select("*").From("form_definitions").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchForm(ctx, query, args...)
}

func (r *FormRepository) GetFormBySlug(ctx context.Context, slug string) (*form.FormDefinition, error) {
	query, args, _ := r.sq.Select("*").From("form_definitions").Where(squirrel.Eq{"slug": slug}).ToSql()
	return r.fetchForm(ctx, query, args...)
}

func (r *FormRepository) ListForms(ctx context.Context, fType *form.FormType, activeOnly bool) ([]form.FormDefinition, error) {
	builder := r.sq.Select("*").From("form_definitions")

	if fType != nil {
		builder = builder.Where(squirrel.Eq{"type": *fType})
	}
	if activeOnly {
		builder = builder.Where(squirrel.Eq{"is_active": true})
	}

	query, args, err := builder.OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, err
	}

	var rows []dbFormDefinition
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	forms := make([]form.FormDefinition, len(rows))
	for i, row := range rows {
		f, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		forms[i] = *f
	}
	return forms, nil
}

func (r *FormRepository) UpdateForm(ctx context.Context, f *form.FormDefinition) error {
	f.UpdatedAt = time.Now().UTC()
	dbF, err := toDBForm(f)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("form_definitions").
		Set("title", dbF.Title).
		Set("description", dbF.Description).
		Set("is_active", dbF.IsActive).
		Set("schema", dbF.Schema).
		Set("voting_config", dbF.VotingConfig).
		Set("expires_at", dbF.ExpiresAt).
		Set("updated_at", dbF.UpdatedAt).
		Where(squirrel.Eq{"id": dbF.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return form.ErrFormNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// Submissions
// -----------------------------------------------------------------------------

func (r *FormRepository) CreateSubmission(ctx context.Context, s *form.Submission) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.SubmittedAt.IsZero() {
		s.SubmittedAt = time.Now().UTC()
	}
	s.UpdatedAt = s.SubmittedAt

	dbS, err := toDBSubmission(s)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("submissions").
		Columns("id", "form_id", "user_id", "status", "data", "score", "submitted_at", "updated_at").
		Values(dbS.ID, dbS.FormID, dbS.UserID, dbS.Status, dbS.Data, dbS.Score, dbS.SubmittedAt, dbS.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *FormRepository) GetSubmission(ctx context.Context, id uuid.UUID) (*form.Submission, error) {
	query, args, _ := r.sq.Select("*").From("submissions").Where(squirrel.Eq{"id": id}).ToSql()
	var dbS dbSubmission
	if err := sqlx.GetContext(ctx, r.runner, &dbS, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, form.ErrSubmissionNotFound
		}
		return nil, err
	}
	return dbS.toDomain()
}

func (r *FormRepository) ListSubmissions(ctx context.Context, formID uuid.UUID, limit, offset int) ([]form.Submission, int, error) {
	builder := r.sq.Select("*").From("submissions").Where(squirrel.Eq{"form_id": formID})
	countBuilder := r.sq.Select("COUNT(*)").From("submissions").Where(squirrel.Eq{"form_id": formID})

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("submitted_at DESC").
		Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbSubmission
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	subs := make([]form.Submission, len(rows))
	for i, r := range rows {
		s, err := r.toDomain()
		if err != nil {
			return nil, 0, err
		}
		subs[i] = *s
	}
	return subs, total, nil
}

func (r *FormRepository) UpdateSubmission(ctx context.Context, s *form.Submission) error {
	s.UpdatedAt = time.Now().UTC()
	dbS, err := toDBSubmission(s)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("submissions").
		Set("status", dbS.Status).
		Set("score", dbS.Score).
		Set("data", dbS.Data).
		Set("updated_at", dbS.UpdatedAt).
		Where(squirrel.Eq{"id": dbS.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return form.ErrSubmissionNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// Voting Logic
// -----------------------------------------------------------------------------

func (r *FormRepository) RecordVoteParticipation(ctx context.Context, formID, userID uuid.UUID) error {
	query, args, _ := r.sq.Insert("vote_records").
		Columns("form_id", "user_id", "voted_at").
		Values(formID, userID, time.Now().UTC()).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isFormUniqueViolation(err) {
			return form.ErrAlreadyVoted
		}
		return err
	}
	return nil
}

func (r *FormRepository) CheckVoteParticipation(ctx context.Context, formID, userID uuid.UUID) (bool, error) {
	query, args, _ := r.sq.Select("1").From("vote_records").
		Where(squirrel.Eq{"form_id": formID, "user_id": userID}).
		Limit(1).ToSql()

	var exists int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *FormRepository) GetAggregateResults(ctx context.Context, formID uuid.UUID) (map[string]int, error) {
	// Fetch JSONB data as strings to avoid bytea encoding issues
	query, args, _ := r.sq.Select("data").From("submissions").
		Where(squirrel.Eq{"form_id": formID}).ToSql()

	var jsonRows []string
	if err := sqlx.SelectContext(ctx, r.runner, &jsonRows, query, args...); err != nil {
		return nil, err
	}

	results := make(map[string]int)

	for _, raw := range jsonRows {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			continue
		}

		for _, v := range data {
			if str, ok := v.(string); ok {
				results[str]++
			}
		}
	}

	return results, nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (r *FormRepository) fetchForm(ctx context.Context, query string, args ...interface{}) (*form.FormDefinition, error) {
	var dbF dbFormDefinition
	if err := sqlx.GetContext(ctx, r.runner, &dbF, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, form.ErrFormNotFound
		}
		return nil, err
	}
	return dbF.toDomain()
}

func isFormUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

// -----------------------------------------------------------------------------
// DB Models
// -----------------------------------------------------------------------------

// Use string for JSONB fields to ensure correct driver mapping
type dbFormDefinition struct {
	ID           uuid.UUID      `db:"id"`
	Slug         string         `db:"slug"`
	Title        string         `db:"title"`
	Description  sql.NullString `db:"description"`
	Type         string         `db:"type"`
	IsActive     bool           `db:"is_active"`
	IsAnonymous  bool           `db:"is_anonymous"`
	Schema       string         `db:"schema"`        // JSONB as string
	VotingConfig sql.NullString `db:"voting_config"` // JSONB as string
	ExpiresAt    sql.NullTime   `db:"expires_at"`
	CreatedBy    uuid.UUID      `db:"created_by"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

func (d *dbFormDefinition) toDomain() (*form.FormDefinition, error) {
	f := &form.FormDefinition{
		ID:          d.ID,
		Slug:        d.Slug,
		Title:       d.Title,
		Type:        form.FormType(d.Type),
		IsActive:    d.IsActive,
		IsAnonymous: d.IsAnonymous,
		Schema:      json.RawMessage(d.Schema),
		CreatedBy:   d.CreatedBy,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.Description.Valid {
		f.Description = d.Description.String
	}
	if d.ExpiresAt.Valid {
		f.ExpiresAt = &d.ExpiresAt.Time
	}
	if d.VotingConfig.Valid && d.VotingConfig.String != "null" {
		var vc form.VotingConfig
		if err := json.Unmarshal([]byte(d.VotingConfig.String), &vc); err != nil {
			return nil, err
		}
		f.VotingConfig = &vc
	}
	return f, nil
}

func toDBForm(f *form.FormDefinition) (*dbFormDefinition, error) {
	schemaJSON, err := json.Marshal(f.Schema)
	if err != nil {
		return nil, err
	}
	if f.Schema == nil {
		schemaJSON = []byte("[]")
	}

	var vcStr sql.NullString
	if f.VotingConfig != nil {
		vcJSON, err := json.Marshal(f.VotingConfig)
		if err != nil {
			return nil, err
		}
		vcStr.String = string(vcJSON)
		vcStr.Valid = true
	}

	d := &dbFormDefinition{
		ID:           f.ID,
		Slug:         f.Slug,
		Title:        f.Title,
		Type:         string(f.Type),
		IsActive:     f.IsActive,
		IsAnonymous:  f.IsAnonymous,
		Schema:       string(schemaJSON), // Pass as string
		VotingConfig: vcStr,
		CreatedBy:    f.CreatedBy,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
	if f.Description != "" {
		d.Description = sql.NullString{String: f.Description, Valid: true}
	}
	if f.ExpiresAt != nil {
		d.ExpiresAt = sql.NullTime{Time: *f.ExpiresAt, Valid: true}
	}
	return d, nil
}

type dbSubmission struct {
	ID          uuid.UUID       `db:"id"`
	FormID      uuid.UUID       `db:"form_id"`
	UserID      uuid.NullUUID   `db:"user_id"`
	Status      string          `db:"status"`
	Data        string          `db:"data"` // JSONB as string
	Score       sql.NullFloat64 `db:"score"`
	SubmittedAt time.Time       `db:"submitted_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

func (d *dbSubmission) toDomain() (*form.Submission, error) {
	s := &form.Submission{
		ID:          d.ID,
		FormID:      d.FormID,
		Status:      form.SubmissionStatus(d.Status),
		Data:        json.RawMessage(d.Data),
		SubmittedAt: d.SubmittedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.UserID.Valid {
		s.UserID = &d.UserID.UUID
	}
	if d.Score.Valid {
		s.Score = &d.Score.Float64
	}
	return s, nil
}

func toDBSubmission(s *form.Submission) (*dbSubmission, error) {
	dataJSON, err := json.Marshal(s.Data)
	if err != nil {
		return nil, err
	}
	if s.Data == nil {
		dataJSON = []byte("{}")
	}

	d := &dbSubmission{
		ID:          s.ID,
		FormID:      s.FormID,
		Status:      string(s.Status),
		Data:        string(dataJSON), // Pass as string
		SubmittedAt: s.SubmittedAt,
		UpdatedAt:   s.UpdatedAt,
	}
	if s.UserID != nil {
		d.UserID = uuid.NullUUID{UUID: *s.UserID, Valid: true}
	}
	if s.Score != nil {
		d.Score = sql.NullFloat64{Float64: *s.Score, Valid: true}
	}
	return d, nil
}
