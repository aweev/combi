package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ privacy.Repository = (*PrivacyRepository)(nil)

type PrivacyRepository struct {
	db     *sqlx.DB
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewPrivacyRepository(db *sqlx.DB, logger log.Logger) *PrivacyRepository {
	return &PrivacyRepository{
		db:     db,
		logger: logger.With(log.String("component", "privacy_repo")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *PrivacyRepository) SaveRequest(ctx context.Context, req *privacy.Request) error {
	// Serialize the complex fields
	details, err := json.Marshal(req.ErasureDetails)
	if err != nil {
		return fmt.Errorf("failed to marshal erasure details: %w", err)
	}

	query := r.sq.Insert("privacy_requests").
		Columns(
			"id", "user_id", "type", "status",
			"requested_at", "completed_at", "reason",
			"result_location", "erasure_details",
		).
		Values(
			req.ID, req.UserID, req.Type, req.Status,
			req.RequestedAt, req.CompletedAt, req.Reason,
			req.ResultLocation, details,
		).
		Suffix(`ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			result_location = EXCLUDED.result_location,
			erasure_details = EXCLUDED.erasure_details
		`)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	if _, err := r.db.ExecContext(ctx, sqlStr, args...); err != nil {
		return fmt.Errorf("failed to save privacy request: %w", err)
	}
	return nil
}

func (r *PrivacyRepository) GetRequest(ctx context.Context, id uuid.UUID) (*privacy.Request, error) {
	query, args, _ := r.sq.Select("*").From("privacy_requests").Where(squirrel.Eq{"id": id}).ToSql()

	var dbReq dbPrivacyRequest
	if err := r.db.GetContext(ctx, &dbReq, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, privacy.ErrRequestNotFound
		}
		return nil, err
	}
	return dbReq.ToDomain()
}

func (r *PrivacyRepository) GetPendingRequest(ctx context.Context, userID uuid.UUID, typ privacy.RequestType) (*privacy.Request, error) {
	query, args, _ := r.sq.Select("*").
		From("privacy_requests").
		Where(squirrel.Eq{
			"user_id": userID,
			"type":    typ,
		}).
		Where("status IN (?, ?)", privacy.StatusPending, privacy.StatusProcessing).
		ToSql()

	var dbReq dbPrivacyRequest
	if err := r.db.GetContext(ctx, &dbReq, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No pending request found
		}
		return nil, err
	}
	return dbReq.ToDomain()
}

// --- DB Model DTO ---

type dbPrivacyRequest struct {
	ID             uuid.UUID      `db:"id"`
	UserID         uuid.UUID      `db:"user_id"`
	Type           string         `db:"type"`
	Status         string         `db:"status"`
	RequestedAt    time.Time      `db:"requested_at"`
	CompletedAt    *time.Time     `db:"completed_at"`
	Reason         sql.NullString `db:"reason"`
	ResultLocation sql.NullString `db:"result_location"`
	ErasureDetails []byte         `db:"erasure_details"` // JSONB
}

func (d *dbPrivacyRequest) ToDomain() (*privacy.Request, error) {
	var details *privacy.ErasureReport
	if len(d.ErasureDetails) > 0 {
		// Check if it's "null" JSON
		if string(d.ErasureDetails) != "null" {
			details = &privacy.ErasureReport{}
			if err := json.Unmarshal(d.ErasureDetails, details); err != nil {
				return nil, err
			}
		}
	}

	req := &privacy.Request{
		ID:             d.ID,
		UserID:         d.UserID,
		Type:           privacy.RequestType(d.Type),
		Status:         privacy.RequestStatus(d.Status),
		RequestedAt:    d.RequestedAt,
		CompletedAt:    d.CompletedAt,
		ErasureDetails: details,
	}
	if d.Reason.Valid {
		req.Reason = d.Reason.String
	}
	if d.ResultLocation.Valid {
		req.ResultLocation = &d.ResultLocation.String
	}
	return req, nil
}
