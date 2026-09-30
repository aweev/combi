package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Statically verify implementation.
var _ automation.Repository = (*AutomationRepository)(nil)

type AutomationRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewAutomationRepository(db *sqlx.DB, logger log.Logger) *AutomationRepository {
	return &AutomationRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "automation_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx implements the transaction provider.
func (r *AutomationRepository) WithTx(ctx context.Context, fn func(txRepo automation.Repository) error) (err error) {
	_, isTx := r.runner.(*sqlx.Tx)
	if isTx {
		return errors.New("cannot begin transaction from within another transaction")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("automation repo: failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	txRepo := &AutomationRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	return fn(txRepo)
}

// --- RuleRepository ---

// dbRule maps to automation_rules
// Note: We use string for JSONB fields to ensure correct driver mapping
type dbRule struct {
	ID                   uuid.UUID                 `db:"id"`
	Name                 string                    `db:"name"`
	Description          string                    `db:"description"`
	Status               automation.RuleStatus     `db:"status"`
	Priority             int                       `db:"priority"`
	Trigger              string                    `db:"trigger"`    // JSONB
	Conditions           string                    `db:"conditions"` // JSONB
	Logic                automation.ConditionLogic `db:"logic"`
	Actions              string                    `db:"actions"`  // JSONB
	Metadata             string                    `db:"metadata"` // JSONB
	ExecutionStrategy    string                    `db:"execution_strategy"`
	CreatedAt            time.Time                 `db:"created_at"`
	UpdatedAt            time.Time                 `db:"updated_at"`
	TotalExecutions      int64                     `db:"total_executions"`
	SuccessfulExecutions int64                     `db:"successful_executions"`
}

func (d *dbRule) toDomain() (automation.Rule, error) {
	rule := automation.Rule{
		ID:                d.ID,
		Name:              d.Name,
		Description:       d.Description,
		Status:            d.Status,
		Priority:          d.Priority,
		Logic:             d.Logic,
		ExecutionStrategy: d.ExecutionStrategy,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}
	if err := json.Unmarshal([]byte(d.Trigger), &rule.Trigger); err != nil {
		return automation.Rule{}, fmt.Errorf("failed to unmarshal trigger: %w", err)
	}
	if len(d.Conditions) > 0 && d.Conditions != "null" {
		if err := json.Unmarshal([]byte(d.Conditions), &rule.Conditions); err != nil {
			return automation.Rule{}, fmt.Errorf("failed to unmarshal conditions: %w", err)
		}
	}
	if len(d.Actions) > 0 && d.Actions != "null" {
		if err := json.Unmarshal([]byte(d.Actions), &rule.Actions); err != nil {
			return automation.Rule{}, fmt.Errorf("failed to unmarshal actions: %w", err)
		}
	}
	if len(d.Metadata) > 0 && d.Metadata != "null" {
		if err := json.Unmarshal([]byte(d.Metadata), &rule.Metadata); err != nil {
			return automation.Rule{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	return rule, nil
}

func dbRuleFromDomain(r *automation.Rule) (*dbRule, error) {
	trigger, err := json.Marshal(r.Trigger)
	if err != nil {
		return nil, err
	}
	conditions, err := json.Marshal(r.Conditions)
	if err != nil {
		return nil, err
	}
	actions, err := json.Marshal(r.Actions)
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(r.Metadata)
	if err != nil {
		return nil, err
	}

	// Default strategy if missing
	strategy := r.ExecutionStrategy
	if strategy == "" {
		strategy = "SEQUENCE"
	}

	return &dbRule{
		ID:                r.ID,
		Name:              r.Name,
		Description:       r.Description,
		Status:            r.Status,
		Priority:          r.Priority,
		Trigger:           string(trigger),
		Conditions:        string(conditions),
		Logic:             r.Logic,
		Actions:           string(actions),
		Metadata:          string(metadata),
		ExecutionStrategy: strategy,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}, nil
}

const createRuleQuery = `
	INSERT INTO automation_rules (
		id, name, description, status, priority, trigger, conditions, logic, actions,
		metadata, execution_strategy, created_at, updated_at, total_executions, successful_executions
	) VALUES (
		:id, :name, :description, :status, :priority, :trigger, :conditions, :logic, :actions,
		:metadata, :execution_strategy, :created_at, :updated_at, 0, 0
	)`

func (r *AutomationRepository) CreateRule(ctx context.Context, rule *automation.Rule) error {
	dbModel, err := dbRuleFromDomain(rule)
	if err != nil {
		return err
	}
	_, err = sqlx.NamedExecContext(ctx, r.runner, createRuleQuery, dbModel)
	if err != nil {
		return fmt.Errorf("create rule failed: %w", err)
	}
	return nil
}

func (r *AutomationRepository) GetRule(ctx context.Context, id uuid.UUID) (*automation.Rule, error) {
	query, args, _ := r.sq.Select("*").From("automation_rules").Where(squirrel.Eq{"id": id}).ToSql()
	var dbR dbRule
	if err := sqlx.GetContext(ctx, r.runner, &dbR, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, automation.ErrRuleNotFound
		}
		return nil, fmt.Errorf("get rule failed: %w", err)
	}
	rule, err := dbR.toDomain()
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

const updateRuleQuery = `
	UPDATE automation_rules SET
		name = :name,
		description = :description,
		status = :status,
		priority = :priority,
		trigger = :trigger,
		conditions = :conditions,
		logic = :logic,
		actions = :actions,
		metadata = :metadata,
		execution_strategy = :execution_strategy,
		updated_at = :updated_at
	WHERE id = :id`

func (r *AutomationRepository) UpdateRule(ctx context.Context, rule *automation.Rule) error {
	rule.UpdatedAt = time.Now().UTC()
	dbModel, err := dbRuleFromDomain(rule)
	if err != nil {
		return err
	}

	res, err := sqlx.NamedExecContext(ctx, r.runner, updateRuleQuery, dbModel)
	if err != nil {
		return fmt.Errorf("update rule failed: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return automation.ErrRuleNotFound
	}
	return nil
}

func (r *AutomationRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("automation_rules").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete rule failed: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return automation.ErrRuleNotFound
	}
	return nil
}

func (r *AutomationRepository) ListRules(ctx context.Context, filter automation.RuleFilter) ([]automation.Rule, int64, error) {
	// Count
	countBuilder := r.sq.Select("COUNT(*)").From("automation_rules")
	countBuilder = r.applyRuleFilters(countBuilder, filter)
	countSQL, args, _ := countBuilder.ToSql()
	var total int64
	if err := sqlx.GetContext(ctx, r.runner, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count rules failed: %w", err)
	}
	if total == 0 {
		return []automation.Rule{}, 0, nil
	}

	// List
	dataBuilder := r.sq.Select("*").From("automation_rules")
	dataBuilder = r.applyRuleFilters(dataBuilder, filter)
	dataBuilder = dataBuilder.OrderBy("priority ASC, name ASC").Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	listSQL, args, _ := dataBuilder.ToSql()

	var dbRules []dbRule
	if err := sqlx.SelectContext(ctx, r.runner, &dbRules, listSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("list rules failed: %w", err)
	}

	rules := make([]automation.Rule, len(dbRules))
	for i, d := range dbRules {
		r, err := d.toDomain()
		if err != nil {
			return nil, 0, err
		}
		rules[i] = r
	}
	return rules, total, nil
}

func (r *AutomationRepository) applyRuleFilters(b squirrel.SelectBuilder, f automation.RuleFilter) squirrel.SelectBuilder {
	if f.Status != nil {
		b = b.Where(squirrel.Eq{"status": *f.Status})
	}
	if f.TriggerType != nil {
		// Use JSON operator as string
		b = b.Where("trigger->>'type' = ?", *f.TriggerType)
	}
	if f.Search != "" {
		pat := "%" + f.Search + "%"
		b = b.Where(squirrel.Or{squirrel.ILike{"name": pat}, squirrel.ILike{"description": pat}})
	}
	return b
}

func (r *AutomationRepository) GetActiveRulesByTrigger(ctx context.Context, triggerType string) ([]automation.Rule, error) {
	// Note: Explicitly selecting columns if needed, or * is fine if struct aligns
	query, args, _ := r.sq.Select("*").
		From("automation_rules").
		Where(squirrel.Eq{"status": automation.RuleStatusActive}).
		Where("trigger->>'type' = ?", triggerType).
		OrderBy("priority ASC").
		ToSql()

	var dbRules []dbRule
	if err := sqlx.SelectContext(ctx, r.runner, &dbRules, query, args...); err != nil {
		return nil, fmt.Errorf("get active rules failed: %w", err)
	}

	rules := make([]automation.Rule, len(dbRules))
	for i, d := range dbRules {
		r, err := d.toDomain()
		if err != nil {
			return nil, err
		}
		rules[i] = r
	}
	return rules, nil
}

func (r *AutomationRepository) IncrementRuleStats(ctx context.Context, ruleID uuid.UUID, success bool) error {
	sDelta := int64(0)
	if success {
		sDelta = 1
	}
	return r.IncrementRuleStatsBatch(ctx, ruleID, 1, sDelta)
}

func (r *AutomationRepository) IncrementRuleStatsBatch(ctx context.Context, ruleID uuid.UUID, totalDelta, successDelta int64) error {
	query := `
		UPDATE automation_rules 
		SET 
			total_executions = total_executions + $1,
			successful_executions = successful_executions + $2
		WHERE id = $3
	`
	_, err := r.runner.ExecContext(ctx, query, totalDelta, successDelta, ruleID)
	return err
}

// --- ExecutionRepository ---

type dbExecution struct {
	ID               uuid.UUID                  `db:"id"`
	RuleID           uuid.UUID                  `db:"rule_id"`
	EventID          uuid.UUID                  `db:"event_id"`
	Status           automation.ExecutionStatus `db:"status"`
	TriggerData      string                     `db:"trigger_data"`      // JSONB
	ActionsSnapshot  string                     `db:"actions_snapshot"`  // JSONB
	ConditionsResult string                     `db:"conditions_result"` // JSONB
	ActionsResult    string                     `db:"actions_result"`    // JSONB
	StartedAt        time.Time                  `db:"started_at"`
	UpdatedAt        time.Time                  `db:"updated_at"`
	CompletedAt      *time.Time                 `db:"completed_at"`
	Duration         int64                      `db:"duration"` // Nanoseconds
	Error            *string                    `db:"error"`
}

func (d *dbExecution) toDomain() (automation.Execution, error) {
	exec := automation.Execution{
		ID:          d.ID,
		RuleID:      d.RuleID,
		EventID:     d.EventID,
		Status:      d.Status,
		StartedAt:   d.StartedAt,
		UpdatedAt:   d.UpdatedAt,
		CompletedAt: d.CompletedAt,
		Duration:    time.Duration(d.Duration),
		Error:       d.Error,
	}

	// Safe Unmarshal
	exec.TriggerData = json.RawMessage(d.TriggerData)

	if d.ActionsSnapshot != "" && d.ActionsSnapshot != "null" {
		_ = json.Unmarshal([]byte(d.ActionsSnapshot), &exec.ActionsSnapshot)
	}
	if d.ConditionsResult != "" && d.ConditionsResult != "null" {
		_ = json.Unmarshal([]byte(d.ConditionsResult), &exec.ConditionsResult)
	}
	if d.ActionsResult != "" && d.ActionsResult != "null" {
		_ = json.Unmarshal([]byte(d.ActionsResult), &exec.ActionsResult)
	}
	return exec, nil
}

func dbExecutionFromDomain(e *automation.Execution) (*dbExecution, error) {
	// TriggerData is already json.RawMessage ([]byte), convert to string for DB
	triggerDataStr := string(e.TriggerData)
	if triggerDataStr == "" {
		triggerDataStr = "{}"
	}

	snapRes, err := json.Marshal(e.ActionsSnapshot)
	if err != nil {
		return nil, err
	}

	condRes, err := json.Marshal(e.ConditionsResult)
	if err != nil {
		return nil, err
	}

	actRes, err := json.Marshal(e.ActionsResult)
	if err != nil {
		return nil, err
	}

	return &dbExecution{
		ID:               e.ID,
		RuleID:           e.RuleID,
		EventID:          e.EventID,
		Status:           e.Status,
		TriggerData:      triggerDataStr,
		ActionsSnapshot:  string(snapRes),
		ConditionsResult: string(condRes),
		ActionsResult:    string(actRes),
		StartedAt:        e.StartedAt,
		UpdatedAt:        e.UpdatedAt,
		CompletedAt:      e.CompletedAt,
		Duration:         e.Duration.Nanoseconds(),
		Error:            e.Error,
	}, nil
}

const createExecutionQuery = `
	INSERT INTO automation_executions (
		id, rule_id, event_id, status, trigger_data, actions_snapshot, conditions_result,
		actions_result, started_at, updated_at, completed_at, duration, error
	) VALUES (
		:id, :rule_id, :event_id, :status, :trigger_data, :actions_snapshot, :conditions_result,
		:actions_result, :started_at, :started_at, :completed_at, :duration, :error
	)`

func (r *AutomationRepository) CreateExecution(ctx context.Context, execution *automation.Execution) error {
	dbModel, err := dbExecutionFromDomain(execution)
	if err != nil {
		return err
	}
	_, err = sqlx.NamedExecContext(ctx, r.runner, createExecutionQuery, dbModel)
	if err != nil {
		return fmt.Errorf("create execution failed: %w", err)
	}
	return nil
}

func (r *AutomationRepository) GetExecution(ctx context.Context, id uuid.UUID) (*automation.Execution, error) {
	query, args, _ := r.sq.Select("*").From("automation_executions").Where(squirrel.Eq{"id": id}).ToSql()
	var dbE dbExecution
	if err := sqlx.GetContext(ctx, r.runner, &dbE, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, automation.ErrExecutionNotFound
		}
		return nil, fmt.Errorf("get execution failed: %w", err)
	}
	exec, err := dbE.toDomain()
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

const updateExecutionQuery = `
	UPDATE automation_executions SET
		status = :status,
		conditions_result = :conditions_result,
		actions_result = :actions_result,
		updated_at = NOW(),
		completed_at = :completed_at,
		duration = :duration,
		error = :error
	WHERE id = :id`

func (r *AutomationRepository) UpdateExecution(ctx context.Context, execution *automation.Execution) error {
	dbModel, err := dbExecutionFromDomain(execution)
	if err != nil {
		return err
	}
	res, err := sqlx.NamedExecContext(ctx, r.runner, updateExecutionQuery, dbModel)
	if err != nil {
		return fmt.Errorf("update execution failed: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return automation.ErrExecutionNotFound
	}
	return nil
}

func (r *AutomationRepository) ListExecutions(ctx context.Context, filter automation.ExecutionFilter) ([]automation.Execution, int64, error) {
	countBuilder := r.sq.Select("COUNT(*)").From("automation_executions")
	countBuilder = r.applyExecutionFilters(countBuilder, filter)
	countSQL, args, _ := countBuilder.ToSql()
	var total int64
	if err := sqlx.GetContext(ctx, r.runner, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count executions failed: %w", err)
	}
	if total == 0 {
		return []automation.Execution{}, 0, nil
	}

	dataBuilder := r.sq.Select("*").From("automation_executions")
	dataBuilder = r.applyExecutionFilters(dataBuilder, filter)
	dataBuilder = dataBuilder.OrderBy("started_at DESC").Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	listSQL, args, _ := dataBuilder.ToSql()

	var dbExecs []dbExecution
	if err := sqlx.SelectContext(ctx, r.runner, &dbExecs, listSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("list executions failed: %w", err)
	}

	execs := make([]automation.Execution, len(dbExecs))
	for i, d := range dbExecs {
		e, err := d.toDomain()
		if err != nil {
			return nil, 0, err
		}
		execs[i] = e
	}
	return execs, total, nil
}

func (r *AutomationRepository) applyExecutionFilters(b squirrel.SelectBuilder, f automation.ExecutionFilter) squirrel.SelectBuilder {
	if f.RuleID != nil {
		b = b.Where(squirrel.Eq{"rule_id": *f.RuleID})
	}
	if f.Status != nil {
		b = b.Where(squirrel.Eq{"status": *f.Status})
	}
	if f.From != nil {
		b = b.Where(squirrel.GtOrEq{"started_at": *f.From})
	}
	if f.To != nil {
		b = b.Where(squirrel.LtOrEq{"started_at": *f.To})
	}
	return b
}

// FindStaleExecutions finds jobs stuck in RUNNING state older than threshold.
func (r *AutomationRepository) FindStaleExecutions(ctx context.Context, olderThan time.Time, limit int) ([]automation.Execution, error) {
	query, args, _ := r.sq.Select("*").
		From("automation_executions").
		Where(squirrel.Eq{"status": automation.ExecutionStatusRunning}).
		Where(squirrel.Lt{"updated_at": olderThan}).
		OrderBy("updated_at ASC").
		Limit(uint64(limit)).
		ToSql()

	var dbExecs []dbExecution
	if err := sqlx.SelectContext(ctx, r.runner, &dbExecs, query, args...); err != nil {
		return nil, fmt.Errorf("find stale executions failed: %w", err)
	}

	execs := make([]automation.Execution, 0, len(dbExecs))
	for _, d := range dbExecs {
		e, err := d.toDomain()
		if err != nil {
			continue // Skip corrupt rows logic handled by recovery service
		}
		execs = append(execs, e)
	}
	return execs, nil
}

// PruneExecutions efficiently deletes old executions.
func (r *AutomationRepository) PruneExecutions(ctx context.Context, olderThan time.Time, batchSize int) (int64, error) {
	query := `
		WITH deleted AS (
			DELETE FROM automation_executions
			WHERE id IN (
				SELECT id FROM automation_executions 
				WHERE started_at < $1
				LIMIT $2
			)
			RETURNING id
		)
		SELECT COUNT(*) FROM deleted
	`
	var count int64
	err := r.db.GetContext(ctx, &count, query, olderThan, batchSize)
	if err != nil {
		return 0, fmt.Errorf("prune executions failed: %w", err)
	}
	return count, nil
}

// Stats Helpers (Rule & System)
func (r *AutomationRepository) GetRuleExecutionStats(ctx context.Context, ruleID uuid.UUID, from, to time.Time) (*automation.RuleStatistics, error) {
	query := `
		SELECT
			$1::uuid AS rule_id,
			COUNT(*) AS total_executions,
			COUNT(*) FILTER (WHERE status = 'COMPLETED') AS success_count,
			COUNT(*) FILTER (WHERE status = 'FAILED') AS failure_count,
			COALESCE(AVG(duration), 0) / 1000000.0 AS avg_execution_time_ms
		FROM automation_executions
		WHERE rule_id = $1 AND started_at >= $2 AND started_at < $3
	`
	type dbStats struct {
		RuleID             uuid.UUID `db:"rule_id"`
		TotalExecutions    int64     `db:"total_executions"`
		SuccessCount       int64     `db:"success_count"`
		FailureCount       int64     `db:"failure_count"`
		AvgExecutionTimeMs float64   `db:"avg_execution_time_ms"`
	}

	var d dbStats
	if err := sqlx.GetContext(ctx, r.runner, &d, query, ruleID, from, to); err != nil {
		return nil, err
	}

	s := &automation.RuleStatistics{
		RuleID: d.RuleID, TotalExecutions: d.TotalExecutions,
		SuccessCount: d.SuccessCount, FailureCount: d.FailureCount,
		AvgExecutionTimeMs: d.AvgExecutionTimeMs, Period: to.Sub(from),
	}
	if s.TotalExecutions > 0 {
		s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalExecutions)
	}
	return s, nil
}

func (r *AutomationRepository) GetSystemStats(ctx context.Context, from, to time.Time) (*automation.SystemStatistics, error) {
	// 1. Get Overall System Stats
	const sysQuery = `
		SELECT
			COUNT(*) AS total_executions,
			COUNT(*) FILTER (WHERE status = 'COMPLETED') AS total_successes,
			COUNT(*) FILTER (WHERE status = 'FAILED') AS total_failures,
			COALESCE(AVG(duration), 0) / 1000000.0 AS avg_execution_time_ms
		FROM automation_executions
		WHERE started_at >= $1 AND started_at < $2
	`
	type dbSys struct {
		TotalExecutions int64   `db:"total_executions"`
		TotalSuccesses  int64   `db:"total_successes"`
		TotalFailures   int64   `db:"total_failures"`
		AvgMs           float32 `db:"avg_execution_time_ms"`
	}

	var ds dbSys
	if err := sqlx.GetContext(ctx, r.runner, &ds, sysQuery, from, to); err != nil {
		return nil, fmt.Errorf("failed to get system stats summary: %w", err)
	}

	// 2. Get Top Failing Rules with details
	const topFailingRulesQuery = `
		WITH top_failures AS (
			SELECT rule_id
			FROM automation_executions
			WHERE started_at >= $1 AND started_at < $2 AND status = 'FAILED'
			GROUP BY rule_id
			ORDER BY COUNT(*) DESC
			LIMIT 5
		)
		SELECT
			ae.rule_id,
			COUNT(*) AS failure_count, 
			COALESCE(s.total_executions, 0) as total_executions,
			COALESCE(s.success_count, 0) as success_count,
			COALESCE(s.avg_execution_time_ms, 0) as avg_execution_time_ms
		FROM automation_executions ae
		LEFT JOIN (
			SELECT
				rule_id,
				COUNT(*) AS total_executions,
				COUNT(*) FILTER (WHERE status = 'COMPLETED') AS success_count,
				COALESCE(AVG(duration), 0) / 1000000.0 AS avg_execution_time_ms
			FROM automation_executions
			WHERE started_at >= $1 AND started_at < $2
			GROUP BY rule_id
		) s ON ae.rule_id = s.rule_id
		WHERE ae.rule_id IN (SELECT rule_id FROM top_failures)
		  AND ae.started_at >= $1 AND ae.started_at < $2
		  AND ae.status = 'FAILED'
		GROUP BY ae.rule_id, s.total_executions, s.success_count, s.avg_execution_time_ms
		ORDER BY failure_count DESC
	`

	type dbRuleStats struct {
		RuleID             uuid.UUID `db:"rule_id"`
		TotalExecutions    int64     `db:"total_executions"`
		SuccessCount       int64     `db:"success_count"`
		FailureCount       int64     `db:"failure_count"`
		AvgExecutionTimeMs float64   `db:"avg_execution_time_ms"`
	}

	var dbRuleStatsList []dbRuleStats
	if err := sqlx.SelectContext(ctx, r.runner, &dbRuleStatsList, topFailingRulesQuery, from, to); err != nil {
		return nil, fmt.Errorf("failed to get top failing rules stats: %w", err)
	}

	stats := &automation.SystemStatistics{
		TotalExecutions:    ds.TotalExecutions,
		TotalSuccesses:     ds.TotalSuccesses,
		TotalFailures:      ds.TotalFailures,
		AvgExecutionTimeMs: ds.AvgMs,
		Period:             to.Sub(from),
		TopFailingRules:    make([]automation.RuleStatistics, len(dbRuleStatsList)),
	}

	for i, rs := range dbRuleStatsList {
		s := automation.RuleStatistics{
			RuleID:             rs.RuleID,
			TotalExecutions:    rs.TotalExecutions,
			SuccessCount:       rs.SuccessCount,
			FailureCount:       rs.FailureCount,
			AvgExecutionTimeMs: rs.AvgExecutionTimeMs,
		}
		if s.TotalExecutions > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalExecutions)
		}
		stats.TopFailingRules[i] = s
	}

	return stats, nil
}

func (r *AutomationRepository) GetUniqueActiveTriggers(ctx context.Context) ([]string, error) {
	// Use JSON arrow operator for text extraction
	query := `
		SELECT DISTINCT trigger->>'type' 
		FROM automation_rules 
		WHERE status = 'ACTIVE'
	`
	var triggers []string
	if err := r.db.SelectContext(ctx, &triggers, query); err != nil {
		return nil, fmt.Errorf("failed to get active triggers: %w", err)
	}
	return triggers, nil
}

func (r *AutomationRepository) AppendActionResult(ctx context.Context, executionID uuid.UUID, result automation.ActionResult) error {
	return r.AppendActionResultBatch(ctx, executionID, []automation.ActionResult{result})
}

func (r *AutomationRepository) AppendActionResultBatch(ctx context.Context, executionID uuid.UUID, results []automation.ActionResult) error {
	if len(results) == 0 {
		return nil
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal batch action results: %w", err)
	}

	// Postgres '||' operator concatenates two JSONB arrays.
	// We cast $1 to jsonb explicitly.
	query := `
		UPDATE automation_executions
		SET 
			actions_result = COALESCE(actions_result, '[]'::jsonb) || $1::jsonb,
			updated_at = NOW()
		WHERE id = $2
	`
	res, err := r.runner.ExecContext(ctx, query, string(resultsJSON), executionID)
	if err != nil {
		return fmt.Errorf("append batch results failed: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return automation.ErrExecutionNotFound
	}
	return nil
}
