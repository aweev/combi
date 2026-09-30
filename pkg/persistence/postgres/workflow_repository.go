package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/workflow"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Statically verify implementation.
var _ workflow.Repository = (*WorkflowRepository)(nil)

type WorkflowRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewWorkflowRepository(db *sqlx.DB, logger log.Logger) *WorkflowRepository {
	return &WorkflowRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "workflow_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *WorkflowRepository) WithTx(ctx context.Context, fn func(txRepo workflow.Repository) error) (err error) {
	_, isTx := r.runner.(*sqlx.Tx)
	if isTx {
		return errors.New("cannot begin transaction from within another transaction")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow repo: failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
			}
		} else {
			err = tx.Commit()
			if err != nil {
				err = fmt.Errorf("workflow repo: failed to commit transaction: %w", err)
			}
		}
	}()

	txRepo := &WorkflowRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	return fn(txRepo)
}

// =================================================================================
// DEFINITIONS
// =================================================================================

const createDefQuery = `
	INSERT INTO workflow_definitions (
		id, name, type, version, description, is_active, config, created_at
	) VALUES (
		:id, :name, :type, :version, :description, :is_active, :config, :created_at
	)`

func (r *WorkflowRepository) CreateDefinition(ctx context.Context, d *workflow.WorkflowDefinition) error {
	dbModel, err := dbDefinitionFromDomain(d)
	if err != nil {
		return err
	}
	_, err = sqlx.NamedExecContext(ctx, r.runner, createDefQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create definition: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) GetDefinition(ctx context.Context, id uuid.UUID) (*workflow.WorkflowDefinition, error) {
	query, args, _ := r.sq.Select("*").From("workflow_definitions").Where(squirrel.Eq{"id": id}).ToSql()
	var dbDef dbDefinition
	if err := sqlx.GetContext(ctx, r.runner, &dbDef, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, workflow.ErrDefinitionNotFound
		}
		return nil, fmt.Errorf("failed to get definition: %w", err)
	}
	return dbDef.toDomain()
}

func (r *WorkflowRepository) ListDefinitions(ctx context.Context, activeOnly bool) ([]workflow.WorkflowDefinition, error) {
	builder := r.sq.Select("*").From("workflow_definitions")
	if activeOnly {
		builder = builder.Where(squirrel.Eq{"is_active": true})
	}
	query, args, _ := builder.OrderBy("name ASC").ToSql()

	var dbDefs []dbDefinition
	if err := sqlx.SelectContext(ctx, r.runner, &dbDefs, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list definitions: %w", err)
	}

	defs := make([]workflow.WorkflowDefinition, len(dbDefs))
	for i, dbDef := range dbDefs {
		d, err := dbDef.toDomain()
		if err != nil {
			return nil, err
		}
		defs[i] = *d
	}
	return defs, nil
}

// =================================================================================
// INSTANCES
// =================================================================================

const createInstanceQuery = `
	INSERT INTO workflow_instances (
		id, workflow_id, temporal_id, run_id, subject_id, status, current_step,
		input_data, result_data, started_at, completed_at
	) VALUES (
		:id, :workflow_id, :temporal_id, :run_id, :subject_id, :status, :current_step,
		:input_data, :result_data, :started_at, :completed_at
	)`

func (r *WorkflowRepository) CreateInstance(ctx context.Context, i *workflow.WorkflowInstance) error {
	dbModel, err := dbInstanceFromDomain(i)
	if err != nil {
		return err
	}
	_, err = sqlx.NamedExecContext(ctx, r.runner, createInstanceQuery, dbModel)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) GetInstance(ctx context.Context, id uuid.UUID) (*workflow.WorkflowInstance, error) {
	query, args, _ := r.sq.Select("*").From("workflow_instances").Where(squirrel.Eq{"id": id}).ToSql()
	var dbInst dbInstance
	if err := sqlx.GetContext(ctx, r.runner, &dbInst, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, workflow.ErrInstanceNotFound
		}
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	return dbInst.toDomain()
}

func (r *WorkflowRepository) UpdateInstanceStatus(ctx context.Context, id uuid.UUID, status workflow.InstanceStatus, currentStep string) error {
	builder := r.sq.Update("workflow_instances").
		Set("status", status).
		Set("current_step", currentStep)

	// If finished, set timestamp
	if status == workflow.StatusCompleted || status == workflow.StatusFailed || status == workflow.StatusCancelled {
		builder = builder.Set("completed_at", time.Now().UTC())
	}

	query, args, _ := builder.Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return workflow.ErrInstanceNotFound
	}
	return nil
}

func (r *WorkflowRepository) ListInstances(ctx context.Context, filter workflow.InstanceFilter) ([]workflow.WorkflowInstance, int, error) {
	// 1. Count
	countBuilder := r.sq.Select("COUNT(*)").From("workflow_instances")
	countBuilder = r.applyInstanceFilter(countBuilder, filter)
	countQuery, args, _ := countBuilder.ToSql()

	var total int
	if err := sqlx.GetContext(ctx, r.runner, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to count instances: %w", err)
	}
	if total == 0 {
		return []workflow.WorkflowInstance{}, 0, nil
	}

	// 2. Select
	selBuilder := r.sq.Select("*").From("workflow_instances")
	selBuilder = r.applyInstanceFilter(selBuilder, filter)
	selBuilder = selBuilder.OrderBy("started_at DESC").Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	query, args, _ := selBuilder.ToSql()

	var dbInsts []dbInstance
	if err := sqlx.SelectContext(ctx, r.runner, &dbInsts, query, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to list instances: %w", err)
	}

	insts := make([]workflow.WorkflowInstance, len(dbInsts))
	for i, dbInst := range dbInsts {
		inst, err := dbInst.toDomain()
		if err != nil {
			return nil, 0, err
		}
		insts[i] = *inst
	}

	return insts, total, nil
}

func (r *WorkflowRepository) applyInstanceFilter(b squirrel.SelectBuilder, f workflow.InstanceFilter) squirrel.SelectBuilder {
	if f.Status != nil {
		b = b.Where(squirrel.Eq{"status": *f.Status})
	}
	if f.SubjectID != nil {
		b = b.Where(squirrel.Eq{"subject_id": *f.SubjectID})
	}
	if f.Type != nil {
		// Subquery filter to find instances belonging to definitions of the specific type
		b = b.Where("workflow_id IN (SELECT id FROM workflow_definitions WHERE type = ?)", *f.Type)
	}
	return b
}

// =================================================================================
// APPROVALS
// =================================================================================

const createApprovalQuery = `
	INSERT INTO approval_requests (
		id, instance_id, assignee_role, assignee_id, message, status,
		decision_note, decided_by, created_at, decided_at
	) VALUES (
		:id, :instance_id, :assignee_role, :assignee_id, :message, :status,
		:decision_note, :decided_by, :created_at, :decided_at
	)`

func (r *WorkflowRepository) CreateApprovalRequest(ctx context.Context, req *workflow.ApprovalRequest) error {
	_, err := sqlx.NamedExecContext(ctx, r.runner, createApprovalQuery, req)
	if err != nil {
		return fmt.Errorf("failed to create approval request: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) GetApprovalRequest(ctx context.Context, id uuid.UUID) (*workflow.ApprovalRequest, error) {
	query, args, _ := r.sq.Select("*").From("approval_requests").Where(squirrel.Eq{"id": id}).ToSql()
	var req workflow.ApprovalRequest // Struct scan works directly here as fields match standard types
	if err := sqlx.GetContext(ctx, r.runner, &req, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, workflow.ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get approval request: %w", err)
	}
	return &req, nil
}

func (r *WorkflowRepository) UpdateApprovalRequest(ctx context.Context, req *workflow.ApprovalRequest) error {
	query := `
		UPDATE approval_requests SET
			status = :status,
			decision_note = :decision_note,
			decided_by = :decided_by,
			decided_at = :decided_at
		WHERE id = :id`

	res, err := sqlx.NamedExecContext(ctx, r.runner, query, req)
	if err != nil {
		return fmt.Errorf("failed to update approval request: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return workflow.ErrRequestNotFound
	}
	return nil
}

func (r *WorkflowRepository) ListPendingApprovals(ctx context.Context, assigneeID *uuid.UUID, role string) ([]workflow.ApprovalRequest, error) {
	// Logic: Show requests where Status=PENDING AND (AssignedToMe OR (AssignedToNoOne AND RoleMatches))
	builder := r.sq.Select("*").From("approval_requests").
		Where(squirrel.Eq{"status": "PENDING"})

	if assigneeID != nil {
		builder = builder.Where(squirrel.Or{
			squirrel.Eq{"assignee_id": *assigneeID},
			squirrel.And{
				squirrel.Eq{"assignee_id": nil},
				squirrel.Eq{"assignee_role": role},
			},
		})
	} else {
		// Just filter by role if no user ID provided
		builder = builder.Where(squirrel.Eq{"assignee_role": role})
	}

	query, args, _ := builder.OrderBy("created_at ASC").ToSql()

	var reqs []workflow.ApprovalRequest
	if err := sqlx.SelectContext(ctx, r.runner, &reqs, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list approvals: %w", err)
	}
	return reqs, nil
}

// =================================================================================
// DB MODELS & MAPPERS
// =================================================================================

type dbDefinition struct {
	ID          uuid.UUID             `db:"id"`
	Name        string                `db:"name"`
	Type        workflow.WorkflowType `db:"type"`
	Version     int                   `db:"version"`
	Description string                `db:"description"`
	IsActive    bool                  `db:"is_active"`
	Config      []byte                `db:"config"` // JSONB
	CreatedAt   time.Time             `db:"created_at"`
}

func (d *dbDefinition) toDomain() (*workflow.WorkflowDefinition, error) {
	def := &workflow.WorkflowDefinition{
		ID:          d.ID,
		Name:        d.Name,
		Type:        d.Type,
		Version:     d.Version,
		Description: d.Description,
		IsActive:    d.IsActive,
		CreatedAt:   d.CreatedAt,
	}
	if len(d.Config) > 0 {
		if err := json.Unmarshal(d.Config, &def.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config for definition %s: %w", d.ID, err)
		}
	}
	return def, nil
}

func dbDefinitionFromDomain(d *workflow.WorkflowDefinition) (*dbDefinition, error) {
	cfg, err := json.Marshal(d.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return &dbDefinition{
		ID:          d.ID,
		Name:        d.Name,
		Type:        d.Type,
		Version:     d.Version,
		Description: d.Description,
		IsActive:    d.IsActive,
		Config:      cfg,
		CreatedAt:   d.CreatedAt,
	}, nil
}

type dbInstance struct {
	ID          uuid.UUID               `db:"id"`
	WorkflowID  uuid.UUID               `db:"workflow_id"`
	TemporalID  string                  `db:"temporal_id"`
	RunID       string                  `db:"run_id"`
	SubjectID   uuid.UUID               `db:"subject_id"`
	Status      workflow.InstanceStatus `db:"status"`
	CurrentStep string                  `db:"current_step"`
	InputData   []byte                  `db:"input_data"`  // JSONB
	ResultData  []byte                  `db:"result_data"` // JSONB
	StartedAt   time.Time               `db:"started_at"`
	CompletedAt *time.Time              `db:"completed_at"`
}

func (d *dbInstance) toDomain() (*workflow.WorkflowInstance, error) {
	i := &workflow.WorkflowInstance{
		ID:          d.ID,
		WorkflowID:  d.WorkflowID,
		TemporalID:  d.TemporalID,
		RunID:       d.RunID,
		SubjectID:   d.SubjectID,
		Status:      d.Status,
		CurrentStep: d.CurrentStep,
		StartedAt:   d.StartedAt,
		CompletedAt: d.CompletedAt,
	}
	// Defensive unmarshaling for nullable JSON
	if len(d.InputData) > 0 {
		if err := json.Unmarshal(d.InputData, &i.InputData); err != nil {
			return nil, err
		}
	}
	if len(d.ResultData) > 0 {
		if err := json.Unmarshal(d.ResultData, &i.ResultData); err != nil {
			return nil, err
		}
	}
	return i, nil
}

func dbInstanceFromDomain(i *workflow.WorkflowInstance) (*dbInstance, error) {
	input, err := json.Marshal(i.InputData)
	if err != nil {
		return nil, err
	}
	result, err := json.Marshal(i.ResultData)
	if err != nil {
		return nil, err
	}
	return &dbInstance{
		ID:          i.ID,
		WorkflowID:  i.WorkflowID,
		TemporalID:  i.TemporalID,
		RunID:       i.RunID,
		SubjectID:   i.SubjectID,
		Status:      i.Status,
		CurrentStep: i.CurrentStep,
		InputData:   input,
		ResultData:  result,
		StartedAt:   i.StartedAt,
		CompletedAt: i.CompletedAt,
	}, nil
}
