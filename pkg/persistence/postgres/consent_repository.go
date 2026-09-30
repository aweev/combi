package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/consent"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ consent.Repository = (*ConsentRepository)(nil)

type ConsentRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewConsentRepository(db *sqlx.DB, logger log.Logger) *ConsentRepository {
	return &ConsentRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "consent_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// --- Agreements ---

func (r *ConsentRepository) CreateAgreement(ctx context.Context, a *consent.Agreement) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}

	query, args, err := r.sq.Insert("agreements").
		Columns("id", "slug", "type", "title", "description", "is_mandatory", "created_at").
		Values(a.ID, a.Slug, a.Type, a.Title, a.Description, a.IsMandatory, a.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return fmt.Errorf("agreement slug already exists: %w", err)
		}
		return err
	}
	return nil
}

func (r *ConsentRepository) GetAgreementBySlug(ctx context.Context, slug string) (*consent.Agreement, error) {
	query, args, _ := r.sq.Select("*").From("agreements").Where(squirrel.Eq{"slug": slug}).ToSql()

	var dba dbAgreement
	if err := sqlx.GetContext(ctx, r.runner, &dba, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, consent.ErrAgreementNotFound
		}
		return nil, err
	}
	return dba.toDomain(), nil
}

// --- Revisions ---

func (r *ConsentRepository) CreateRevision(ctx context.Context, rev *consent.Revision) error {
	if rev.ID == uuid.Nil {
		rev.ID = uuid.New()
	}
	if rev.PublishedAt.IsZero() {
		rev.PublishedAt = time.Now().UTC()
	}

	query, args, err := r.sq.Insert("revisions").
		Columns("id", "agreement_id", "version", "content_url", "content_hash", "published_at", "is_active").
		Values(rev.ID, rev.AgreementID, rev.Version, rev.ContentURL, rev.ContentHash, rev.PublishedAt, rev.IsActive).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create revision: %w", err)
	}
	return nil
}

func (r *ConsentRepository) GetRevision(ctx context.Context, id uuid.UUID) (*consent.Revision, error) {
	query, args, _ := r.sq.Select("*").From("revisions").Where(squirrel.Eq{"id": id}).ToSql()

	var dbr dbRevision
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, consent.ErrRevisionNotFound
		}
		return nil, err
	}
	return dbr.toDomain(), nil
}

func (r *ConsentRepository) GetLatestActiveRevision(ctx context.Context, agreementID uuid.UUID) (*consent.Revision, error) {
	query, args, _ := r.sq.Select("*").From("revisions").
		Where(squirrel.Eq{"agreement_id": agreementID, "is_active": true}).
		OrderBy("published_at DESC").
		Limit(1).
		ToSql()

	var dbr dbRevision
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, consent.ErrRevisionNotFound
		}
		return nil, err
	}
	return dbr.toDomain(), nil
}

// --- User Consents ---

func (r *ConsentRepository) CreateUserConsent(ctx context.Context, uc *consent.UserConsent) error {
	if uc.ID == uuid.Nil {
		uc.ID = uuid.New()
	}
	if uc.GrantedAt.IsZero() {
		uc.GrantedAt = time.Now().UTC()
	}

	// Handle Nullable IP Address for INET column
	var ipAddr interface{} = nil
	if uc.IPAddress != "" {
		ipAddr = uc.IPAddress
	}

	query, args, err := r.sq.Insert("user_consents").
		Columns("id", "user_id", "revision_id", "status", "ip_address", "user_agent", "granted_at").
		Values(uc.ID, uc.UserID, uc.RevisionID, uc.Status, ipAddr, uc.UserAgent, uc.GrantedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return consent.ErrAlreadyConsented
		}
		return err
	}
	return nil
}

func (r *ConsentRepository) GetUserConsent(ctx context.Context, userID, revisionID uuid.UUID) (*consent.UserConsent, error) {
	query, args, _ := r.sq.Select("*").From("user_consents").
		Where(squirrel.Eq{"user_id": userID, "revision_id": revisionID}).
		ToSql()

	var dbu dbUserConsent
	if err := sqlx.GetContext(ctx, r.runner, &dbu, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is not an error here, just nil result
		}
		return nil, err
	}
	return dbu.toDomain(), nil
}

// RevokeUserConsent invalidates all active consents for a specific Agreement (all revisions).
func (r *ConsentRepository) RevokeUserConsent(ctx context.Context, userID, agreementID uuid.UUID, revokedAt time.Time) error {
	// We use raw SQL here because mixing subqueries with Squirrel's Dollar-placeholder
	// logic causes index mismatches (got 5 params, expected 4).
	// Postgres "UPDATE ... FROM" syntax is cleaner and more performant anyway.

	query := `
		UPDATE user_consents uc
		SET status = $1, revoked_at = $2
		FROM revisions r
		WHERE uc.revision_id = r.id
		  AND uc.user_id = $3
		  AND uc.status = $4
		  AND r.agreement_id = $5
	`

	// Arguments must match the numeric placeholders order ($1..$5)
	res, err := r.runner.ExecContext(ctx, query,
		consent.StatusRevoked, // $1
		revokedAt,             // $2
		userID,                // $3
		consent.StatusActive,  // $4
		agreementID,           // $5
	)

	if err != nil {
		return fmt.Errorf("failed to revoke consent: %w", err)
	}

	// We don't error if 0 rows affected (idempotent)
	_, _ = res.RowsAffected()
	return nil
}

// --- Compliance Queries ---

func (r *ConsentRepository) GetMissingMandatoryConsents(ctx context.Context, userID uuid.UUID) ([]consent.Agreement, error) {
	// We use raw SQL with a CTE to clearly define the "Latest Active Revision" set
	// and join it against the user's consent status.
	// This avoids "uuid = boolean" errors and placeholder mismatch issues.

	rawQuery := `
		WITH LatestRevisions AS (
			SELECT DISTINCT ON (agreement_id) id, agreement_id
			FROM revisions
			WHERE is_active = TRUE
			ORDER BY agreement_id, published_at DESC
		)
		SELECT a.* 
		FROM agreements a
		JOIN LatestRevisions lr ON a.id = lr.agreement_id
		WHERE a.is_mandatory = TRUE
		AND NOT EXISTS (
			SELECT 1 FROM user_consents uc
			WHERE uc.user_id = $1
			AND uc.revision_id = lr.id
			AND uc.status = 'ACTIVE'
		)
	`

	var dbas []dbAgreement
	if err := sqlx.SelectContext(ctx, r.runner, &dbas, rawQuery, userID); err != nil {
		return nil, fmt.Errorf("failed to check compliance: %w", err)
	}

	results := make([]consent.Agreement, len(dbas))
	for i, da := range dbas {
		results[i] = *da.toDomain()
	}
	return results, nil
}

func (r *ConsentRepository) HasActiveConsent(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM user_consents uc
			JOIN revisions r ON uc.revision_id = r.id
			JOIN agreements a ON r.agreement_id = a.id
			WHERE uc.user_id = $1
			AND a.slug = $2
			AND uc.status = 'ACTIVE'
			AND r.is_active = TRUE
		)
	`
	var exists bool
	if err := r.runner.QueryRowxContext(ctx, query, userID, slug).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// --- DB Models ---

type dbAgreement struct {
	ID          uuid.UUID `db:"id"`
	Slug        string    `db:"slug"`
	Type        string    `db:"type"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	IsMandatory bool      `db:"is_mandatory"`
	CreatedAt   time.Time `db:"created_at"`
}

func (d *dbAgreement) toDomain() *consent.Agreement {
	return &consent.Agreement{
		ID:          d.ID,
		Slug:        d.Slug,
		Type:        consent.ConsentType(d.Type),
		Title:       d.Title,
		Description: d.Description,
		IsMandatory: d.IsMandatory,
		CreatedAt:   d.CreatedAt,
	}
}

type dbRevision struct {
	ID          uuid.UUID `db:"id"`
	AgreementID uuid.UUID `db:"agreement_id"`
	Version     string    `db:"version"`
	ContentURL  string    `db:"content_url"`
	ContentHash string    `db:"content_hash"`
	PublishedAt time.Time `db:"published_at"`
	IsActive    bool      `db:"is_active"`
}

func (d *dbRevision) toDomain() *consent.Revision {
	return &consent.Revision{
		ID:          d.ID,
		AgreementID: d.AgreementID,
		Version:     d.Version,
		ContentURL:  d.ContentURL,
		ContentHash: d.ContentHash,
		PublishedAt: d.PublishedAt,
		IsActive:    d.IsActive,
	}
}

type dbUserConsent struct {
	ID         uuid.UUID      `db:"id"`
	UserID     uuid.UUID      `db:"user_id"`
	RevisionID uuid.UUID      `db:"revision_id"`
	Status     string         `db:"status"`
	IPAddress  sql.NullString `db:"ip_address"` // Changed to NullString for safe handling
	UserAgent  string         `db:"user_agent"`
	GrantedAt  time.Time      `db:"granted_at"`
	RevokedAt  *time.Time     `db:"revoked_at"`
}

func (d *dbUserConsent) toDomain() *consent.UserConsent {
	c := &consent.UserConsent{
		ID:         d.ID,
		UserID:     d.UserID,
		RevisionID: d.RevisionID,
		Status:     consent.ConsentStatus(d.Status),
		UserAgent:  d.UserAgent,
		GrantedAt:  d.GrantedAt,
		RevokedAt:  d.RevokedAt,
	}
	if d.IPAddress.Valid {
		c.IPAddress = d.IPAddress.String
	}
	return c
}

func (r *ConsentRepository) AnonymizeUserConsents(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		UPDATE user_consents
		SET ip_address = NULL, 
		    user_agent = 'ANONYMIZED'
		WHERE user_id = $1
	`
	res, err := r.runner.ExecContext(ctx, query, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
