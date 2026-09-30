// internal/persistence/postgres/monitoring_repository.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ monitoring.AlertRuleRepository = (*MonitoringRepository)(nil)
var _ monitoring.AlertSilenceRepository = (*MonitoringRepository)(nil)

type MonitoringRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewMonitoringRepository(db *sqlx.DB, logger log.Logger) *MonitoringRepository {
	return &MonitoringRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "monitoring_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// --- Alert Rules ---

func (r *MonitoringRepository) CreateRule(ctx context.Context, rule *monitoring.AlertRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now().UTC()
	}
	rule.UpdatedAt = rule.CreatedAt

	dbModel, err := dbAlertRuleFromDomain(rule)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("alert_rules").
		Columns("id", "name", "description", "is_enabled", "severity", "prom_ql_query", "for_duration_ns", "labels", "annotations", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Name, dbModel.Description, dbModel.IsEnabled, dbModel.Severity, dbModel.PromQLQuery, dbModel.ForDurationNS, dbModel.Labels, dbModel.Annotations, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return monitoring.ErrAlertRuleNameTaken
		}
		return fmt.Errorf("failed to create alert rule: %w", err)
	}
	return nil
}

func (r *MonitoringRepository) GetRule(ctx context.Context, id uuid.UUID) (*monitoring.AlertRule, error) {
	query, args, _ := r.sq.Select("*").From("alert_rules").Where(squirrel.Eq{"id": id}).ToSql()
	return r.fetchRule(ctx, query, args...)
}

func (r *MonitoringRepository) GetRuleByName(ctx context.Context, name string) (*monitoring.AlertRule, error) {
	query, args, _ := r.sq.Select("*").From("alert_rules").Where(squirrel.Eq{"name": name}).ToSql()
	return r.fetchRule(ctx, query, args...)
}

func (r *MonitoringRepository) ListRules(ctx context.Context) ([]*monitoring.AlertRule, error) {
	query, args, _ := r.sq.Select("*").From("alert_rules").OrderBy("name ASC").ToSql()

	var dbr []dbAlertRule
	if err := sqlx.SelectContext(ctx, r.runner, &dbr, query, args...); err != nil {
		return nil, err
	}

	rules := make([]*monitoring.AlertRule, len(dbr))
	for i, d := range dbr {
		dom, err := d.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal alert rule", err)
			continue
		}
		rules[i] = dom
	}
	return rules, nil
}

func (r *MonitoringRepository) UpdateRule(ctx context.Context, rule *monitoring.AlertRule) error {
	rule.UpdatedAt = time.Now().UTC()
	dbModel, err := dbAlertRuleFromDomain(rule)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("alert_rules").
		Set("name", dbModel.Name).
		Set("description", dbModel.Description).
		Set("is_enabled", dbModel.IsEnabled).
		Set("severity", dbModel.Severity).
		Set("prom_ql_query", dbModel.PromQLQuery).
		Set("for_duration_ns", dbModel.ForDurationNS).
		Set("labels", dbModel.Labels).
		Set("annotations", dbModel.Annotations).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return monitoring.ErrAlertRuleNameTaken
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return monitoring.ErrAlertRuleNotFound
	}
	return nil
}

func (r *MonitoringRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("alert_rules").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return monitoring.ErrAlertRuleNotFound
	}
	return nil
}

// --- Alert Silences ---

func (r *MonitoringRepository) CreateSilence(ctx context.Context, s *monitoring.AlertSilence) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}

	dbModel, err := dbSilenceFromDomain(s)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("alert_silences").
		Columns("id", "matchers", "starts_at", "ends_at", "comment", "created_by", "created_at").
		Values(dbModel.ID, dbModel.Matchers, dbModel.StartsAt, dbModel.EndsAt, dbModel.Comment, dbModel.CreatedBy, dbModel.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create silence: %w", err)
	}
	return nil
}

func (r *MonitoringRepository) GetSilence(ctx context.Context, id uuid.UUID) (*monitoring.AlertSilence, error) {
	query, args, _ := r.sq.Select("*").From("alert_silences").Where(squirrel.Eq{"id": id}).ToSql()

	var dbs dbAlertSilence
	if err := sqlx.GetContext(ctx, r.runner, &dbs, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, monitoring.ErrAlertSilenceNotFound
		}
		return nil, err
	}
	return dbs.toDomain()
}

func (r *MonitoringRepository) DeleteSilence(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("alert_silences").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return monitoring.ErrAlertSilenceNotFound
	}
	return nil
}

func (r *MonitoringRepository) ListActiveSilences(ctx context.Context, at time.Time) ([]*monitoring.AlertSilence, error) {
	query, args, _ := r.sq.Select("*").From("alert_silences").
		Where(squirrel.LtOrEq{"starts_at": at}).
		Where(squirrel.Gt{"ends_at": at}).
		ToSql()
	return r.listSilences(ctx, query, args...)
}

func (r *MonitoringRepository) ListAllSilences(ctx context.Context) ([]*monitoring.AlertSilence, error) {
	query, args, _ := r.sq.Select("*").From("alert_silences").OrderBy("created_at DESC").ToSql()
	return r.listSilences(ctx, query, args...)
}

// --- Helpers ---

func (r *MonitoringRepository) fetchRule(ctx context.Context, query string, args ...interface{}) (*monitoring.AlertRule, error) {
	var dbr dbAlertRule
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, monitoring.ErrAlertRuleNotFound
		}
		return nil, err
	}
	return dbr.toDomain()
}

func (r *MonitoringRepository) listSilences(ctx context.Context, query string, args ...interface{}) ([]*monitoring.AlertSilence, error) {
	var dbs []dbAlertSilence
	if err := sqlx.SelectContext(ctx, r.runner, &dbs, query, args...); err != nil {
		return nil, err
	}
	silences := make([]*monitoring.AlertSilence, len(dbs))
	for i, s := range dbs {
		dom, err := s.toDomain()
		if err != nil {
			r.logger.Error("failed to unmarshal silence", err)
			continue
		}
		silences[i] = dom
	}
	return silences, nil
}

// --- DB Models ---

type dbAlertRule struct {
	ID            uuid.UUID      `db:"id"`
	Name          string         `db:"name"`
	Description   sql.NullString `db:"description"`
	IsEnabled     bool           `db:"is_enabled"`
	Severity      string         `db:"severity"`
	PromQLQuery   string         `db:"prom_ql_query"`
	ForDurationNS int64          `db:"for_duration_ns"`
	Labels        string         `db:"labels"`      // JSONB
	Annotations   string         `db:"annotations"` // JSONB
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

func (d *dbAlertRule) toDomain() (*monitoring.AlertRule, error) {
	r := &monitoring.AlertRule{
		ID:          d.ID,
		Name:        d.Name,
		IsEnabled:   d.IsEnabled,
		Severity:    monitoring.AlertSeverity(d.Severity),
		PromQLQuery: d.PromQLQuery,
		ForDuration: time.Duration(d.ForDurationNS),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.Description.Valid {
		r.Description = d.Description.String
	}
	if err := json.Unmarshal([]byte(d.Labels), &r.Labels); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(d.Annotations), &r.Annotations); err != nil {
		return nil, err
	}
	return r, nil
}

func dbAlertRuleFromDomain(r *monitoring.AlertRule) (*dbAlertRule, error) {
	lblBytes, err := json.Marshal(r.Labels)
	if err != nil {
		return nil, err
	}
	if r.Labels == nil {
		lblBytes = []byte("{}")
	}

	annBytes, err := json.Marshal(r.Annotations)
	if err != nil {
		return nil, err
	}
	if r.Annotations == nil {
		annBytes = []byte("{}")
	}

	return &dbAlertRule{
		ID:            r.ID,
		Name:          r.Name,
		Description:   sql.NullString{String: r.Description, Valid: r.Description != ""},
		IsEnabled:     r.IsEnabled,
		Severity:      string(r.Severity),
		PromQLQuery:   r.PromQLQuery,
		ForDurationNS: int64(r.ForDuration),
		Labels:        string(lblBytes),
		Annotations:   string(annBytes),
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}, nil
}

type dbAlertSilence struct {
	ID        uuid.UUID      `db:"id"`
	Matchers  string         `db:"matchers"` // JSONB
	StartsAt  time.Time      `db:"starts_at"`
	EndsAt    time.Time      `db:"ends_at"`
	Comment   sql.NullString `db:"comment"`
	CreatedBy string         `db:"created_by"`
	CreatedAt time.Time      `db:"created_at"`
}

func (d *dbAlertSilence) toDomain() (*monitoring.AlertSilence, error) {
	s := &monitoring.AlertSilence{
		ID:        d.ID,
		StartsAt:  d.StartsAt,
		EndsAt:    d.EndsAt,
		CreatedBy: d.CreatedBy,
		CreatedAt: d.CreatedAt,
	}
	if d.Comment.Valid {
		s.Comment = d.Comment.String
	}
	if err := json.Unmarshal([]byte(d.Matchers), &s.Matchers); err != nil {
		return nil, err
	}
	return s, nil
}

func dbSilenceFromDomain(s *monitoring.AlertSilence) (*dbAlertSilence, error) {
	matchBytes, err := json.Marshal(s.Matchers)
	if err != nil {
		return nil, err
	}
	if s.Matchers == nil {
		matchBytes = []byte("{}")
	}

	return &dbAlertSilence{
		ID:        s.ID,
		Matchers:  string(matchBytes),
		StartsAt:  s.StartsAt,
		EndsAt:    s.EndsAt,
		Comment:   sql.NullString{String: s.Comment, Valid: s.Comment != ""},
		CreatedBy: s.CreatedBy,
		CreatedAt: s.CreatedAt,
	}, nil
}
