package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ reporting.Repository = (*ReportingRepository)(nil)

type ReportingRepository struct {
	db     *sqlx.DB
	logger log.Logger
}

func NewReportingRepository(db *sqlx.DB, logger log.Logger) *ReportingRepository {
	return &ReportingRepository{
		db:     db,
		logger: logger.With(log.String("component", "reporting_repo")),
	}
}

// --- Reports ---

func (r *ReportingRepository) CreateReport(ctx context.Context, report *reporting.GeneratedReport) error {
	if report.ID == uuid.Nil {
		report.ID = uuid.New()
	}
	report.CreatedAt = time.Now().UTC()
	report.Status = reporting.StatusPending

	// Marshal params map to JSON
	// The struct uses json.RawMessage, but if populated via map in service, ensure it's marshaled
	// Assuming service handles marshalling into RawMessage or we do it here if needed.
	// For raw query params, struct scan handles []byte <-> RawMessage automatically.

	query := `
		INSERT INTO generated_reports 
		(id, template_id, requester_id, type, status, format, parameters, created_at, expires_at)
		VALUES 
		(:id, :template_id, :requester_id, :type, :status, :format, :parameters, :created_at, :expires_at)
	`
	_, err := sqlx.NamedExecContext(ctx, r.db, query, report)
	if err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}
	return nil
}

func (r *ReportingRepository) GetReport(ctx context.Context, id uuid.UUID) (*reporting.GeneratedReport, error) {
	var report reporting.GeneratedReport
	query := `SELECT * FROM generated_reports WHERE id = $1`
	if err := r.db.GetContext(ctx, &report, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, reporting.ErrReportNotFound
		}
		return nil, err
	}
	return &report, nil
}

func (r *ReportingRepository) UpdateReportStatus(ctx context.Context, id uuid.UUID, status reporting.ReportStatus, fileURL *string, errReason *string) error {
	var completedAt *time.Time
	if status == reporting.StatusCompleted || status == reporting.StatusFailed {
		now := time.Now().UTC()
		completedAt = &now
	}

	query := `
		UPDATE generated_reports 
		SET status = $1, file_url = $2, error_reason = $3, completed_at = COALESCE($4, completed_at)
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query, status, fileURL, errReason, completedAt, id)
	return err
}

func (r *ReportingRepository) ListReports(ctx context.Context, requesterID uuid.UUID, limit, offset int) ([]reporting.GeneratedReport, int, error) {
	var reports []reporting.GeneratedReport

	// Data
	query := `
		SELECT * FROM generated_reports 
		WHERE requester_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3
	`
	if err := r.db.SelectContext(ctx, &reports, query, requesterID, limit, offset); err != nil {
		return nil, 0, err
	}

	// Count
	var total int
	countQuery := `SELECT COUNT(*) FROM generated_reports WHERE requester_id = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, requesterID); err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

// --- Templates ---

func (r *ReportingRepository) CreateTemplate(ctx context.Context, t *reporting.ReportTemplate) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now

	query := `
		INSERT INTO report_templates (id, slug, name, type, description, content, param_schema, created_at, updated_at)
		VALUES (:id, :slug, :name, :type, :description, :content, :param_schema, :created_at, :updated_at)
	`
	_, err := sqlx.NamedExecContext(ctx, r.db, query, t)
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}
	return nil
}

func (r *ReportingRepository) GetTemplateBySlug(ctx context.Context, slug string) (*reporting.ReportTemplate, error) {
	var t reporting.ReportTemplate
	query := `SELECT * FROM report_templates WHERE slug = $1`
	if err := r.db.GetContext(ctx, &t, query, slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, reporting.ErrTemplateNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *ReportingRepository) ListTemplates(ctx context.Context) ([]reporting.ReportTemplate, error) {
	var templates []reporting.ReportTemplate
	query := `SELECT * FROM report_templates ORDER BY name ASC`
	if err := r.db.SelectContext(ctx, &templates, query); err != nil {
		return nil, err
	}
	return templates, nil
}
