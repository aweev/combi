package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Statically verify implementation
var _ impact.Repository = (*ImpactRepository)(nil)

type ImpactRepository struct {
	db     *sqlx.DB
	logger log.Logger
}

func NewImpactRepository(db *sqlx.DB, logger log.Logger) *ImpactRepository {
	return &ImpactRepository{
		db:     db,
		logger: logger.With(log.String("component", "impact_repository")),
	}
}

// --- Metrics ---

const createMetricQuery = `
	INSERT INTO impact_metrics (id, key, name, unit, type, description, created_at)
	VALUES (:id, :key, :name, :unit, :type, :description, :created_at)
`

func (r *ImpactRepository) CreateMetric(ctx context.Context, def *impact.MetricDefinition) error {
	_, err := sqlx.NamedExecContext(ctx, r.db, createMetricQuery, def)
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}
	return nil
}

func (r *ImpactRepository) GetMetricByKey(ctx context.Context, key string) (*impact.MetricDefinition, error) {
	var def impact.MetricDefinition
	query := `SELECT * FROM impact_metrics WHERE key = $1`
	if err := r.db.GetContext(ctx, &def, query, key); err != nil {
		return nil, fmt.Errorf("failed to get metric by key: %w", err)
	}
	return &def, nil
}

func (r *ImpactRepository) GetMetricByID(ctx context.Context, id uuid.UUID) (*impact.MetricDefinition, error) {
	var def impact.MetricDefinition
	query := `SELECT * FROM impact_metrics WHERE id = $1`
	if err := r.db.GetContext(ctx, &def, query, id); err != nil {
		return nil, fmt.Errorf("failed to get metric by id: %w", err)
	}
	return &def, nil
}

func (r *ImpactRepository) ListMetrics(ctx context.Context) ([]impact.MetricDefinition, error) {
	var defs []impact.MetricDefinition
	query := `SELECT * FROM impact_metrics ORDER BY name ASC`
	if err := r.db.SelectContext(ctx, &defs, query); err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}
	return defs, nil
}

// --- Goals ---

const createGoalQuery = `
	INSERT INTO impact_goals (
		id, metric_id, name, target_value, current_value, start_date, end_date, updated_at
	) VALUES (
		:id, :metric_id, :name, :target_value, :current_value, :start_date, :end_date, :updated_at
	)
`

func (r *ImpactRepository) CreateGoal(ctx context.Context, goal *impact.Goal) error {
	_, err := sqlx.NamedExecContext(ctx, r.db, createGoalQuery, goal)
	if err != nil {
		return fmt.Errorf("failed to create goal: %w", err)
	}
	return nil
}

func (r *ImpactRepository) ListGoals(ctx context.Context, activeOnly bool) ([]impact.Goal, error) {
	var goals []impact.Goal
	query := `SELECT * FROM impact_goals`

	if activeOnly {
		query += ` WHERE end_date >= NOW()`
	}
	query += ` ORDER BY end_date ASC`

	if err := r.db.SelectContext(ctx, &goals, query); err != nil {
		return nil, fmt.Errorf("failed to list goals: %w", err)
	}
	return goals, nil
}

func (r *ImpactRepository) UpdateGoalProgress(ctx context.Context, goalID uuid.UUID, currentVal float64) error {
	query := `
		UPDATE impact_goals 
		SET current_value = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, currentVal, goalID)
	if err != nil {
		return fmt.Errorf("failed to update goal progress: %w", err)
	}
	return nil
}

// --- Entries ---

type dbImpactEntry struct {
	ID             uuid.UUID       `db:"id"`
	MetricID       uuid.UUID       `db:"metric_id"`
	Value          float64         `db:"value"`
	OccurredAt     time.Time       `db:"occurred_at"`
	SourceService  string          `db:"source_service"`
	SourceID       string          `db:"source_id"`
	ProjectID      *uuid.UUID      `db:"project_id"`
	OrganizationID *uuid.UUID      `db:"organization_id"`
	Metadata       json.RawMessage `db:"metadata"`
	CreatedAt      time.Time       `db:"created_at"`
}

const insertEntryQuery = `
	INSERT INTO impact_entries (
		id, metric_id, value, occurred_at, source_service, source_id, 
		project_id, organization_id, metadata, created_at
	) VALUES (
		:id, :metric_id, :value, :occurred_at, :source_service, :source_id,
		:project_id, :organization_id, :metadata, :created_at
	)
`

func (r *ImpactRepository) InsertEntry(ctx context.Context, entry *impact.ImpactEntry) error {
	metaBytes, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal entry metadata: %w", err)
	}

	dbModel := dbImpactEntry{
		ID:             entry.ID,
		MetricID:       entry.MetricID,
		Value:          entry.Value,
		OccurredAt:     entry.OccurredAt,
		SourceService:  entry.SourceService,
		SourceID:       entry.SourceID,
		ProjectID:      entry.ProjectID,
		OrganizationID: entry.OrganizationID,
		Metadata:       metaBytes,
		CreatedAt:      entry.CreatedAt,
	}

	_, err = sqlx.NamedExecContext(ctx, r.db, insertEntryQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to insert impact entry: %w", err)
	}
	return nil
}

// --- Aggregation / Analytics ---

func (r *ImpactRepository) GetMappingsForEvent(ctx context.Context, eventType string) ([]impact.EventMapping, error) {
	// Fix: Explicitly select known columns to avoid "missing destination name created_at" error
	// if the struct lacks that field.
	query := `SELECT id, event_type, metric_id, value_path, multiplier FROM impact_event_mappings WHERE event_type = $1`
	var mappings []impact.EventMapping
	if err := r.db.SelectContext(ctx, &mappings, query, eventType); err != nil {
		return nil, fmt.Errorf("failed to get mappings: %w", err)
	}
	return mappings, nil
}

const createMappingQuery = `
	INSERT INTO impact_event_mappings (id, event_type, metric_id, value_path, multiplier)
	VALUES (:id, :event_type, :metric_id, :value_path, :multiplier)
	ON CONFLICT (event_type, metric_id) DO NOTHING
`

func (r *ImpactRepository) CreateMapping(ctx context.Context, m impact.EventMapping) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	_, err := sqlx.NamedExecContext(ctx, r.db, createMappingQuery, m)
	if err != nil {
		return fmt.Errorf("failed to create mapping: %w", err)
	}
	return nil
}

func (r *ImpactRepository) AggregateStats(ctx context.Context, targetDate time.Time) error {
	dayStart := targetDate.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	query := `
		INSERT INTO impact_daily_aggregates (metric_id, date, value)
		SELECT metric_id, $1::date, SUM(value)
		FROM impact_entries
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY metric_id
		ON CONFLICT (metric_id, date) 
		DO UPDATE SET value = EXCLUDED.value;
	`
	_, err := r.db.ExecContext(ctx, query, dayStart, dayEnd)
	return err
}

func (r *ImpactRepository) SumMetric(ctx context.Context, metricID uuid.UUID, start, end time.Time) (float64, error) {
	// Logic: We check daily aggregates first for performance,
	// but here we fall back to raw entries for maximum precision.
	query := `
		SELECT COALESCE(SUM(value), 0)
		FROM impact_entries
		WHERE metric_id = $1
		  AND occurred_at >= $2 
		  AND occurred_at <= $3
	`
	var total float64
	err := r.db.GetContext(ctx, &total, query, metricID, start, end)
	return total, err
}

func (r *ImpactRepository) SumMetricForOrg(ctx context.Context, metricID, orgID uuid.UUID, start, end time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(value), 0)
		FROM impact_entries
		WHERE metric_id = $1
		  AND organization_id = $2
		  AND occurred_at >= $3 
		  AND occurred_at <= $4
	`
	var total float64
	err := r.db.GetContext(ctx, &total, query, metricID, orgID, start, end)
	return total, err
}

func (r *ImpactRepository) GetFundingMetric(ctx context.Context) (*impact.MetricDefinition, error) {
	// Finds the metric that tracks money (e.g. key="donations_received")
	query := `SELECT * FROM impact_metrics WHERE is_funding_source = TRUE LIMIT 1`
	var def impact.MetricDefinition
	err := r.db.GetContext(ctx, &def, query)
	return &def, err
}
