package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/security"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ audit.Repository = (*AuditRepository)(nil)

type AuditRepository struct {
	db             *sqlx.DB
	runner         sqlx.ExtContext
	squirrelRunner squirrel.BaseRunner
}

func NewAuditRepository(db *sqlx.DB) *AuditRepository {
	return &AuditRepository{
		db:             db,
		runner:         db,
		squirrelRunner: db,
	}
}

func (r *AuditRepository) builder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(r.squirrelRunner)
}

// WithTx uses a named return parameter 'err' so the defer block can inspect it.
func (r *AuditRepository) WithTx(ctx context.Context, fn func(audit.Repository) error) (err error) {
	if _, isTx := r.runner.(*sqlx.Tx); isTx {
		return errors.New("cannot begin transaction from within another transaction")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return &audit.RepositoryError{Operation: "BeginTx", Err: err}
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			// If the function returned an error, rollback
			if rbErr := tx.Rollback(); rbErr != nil {
				// Log this in a real app
				err = fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
			}
		} else {
			// If success, commit
			if commitErr := tx.Commit(); commitErr != nil {
				err = &audit.RepositoryError{Operation: "Commit", Err: commitErr}
			}
		}
	}()

	txRepo := &AuditRepository{
		db:             r.db,
		runner:         tx,
		squirrelRunner: tx,
	}

	// Assign to the named return parameter
	err = fn(txRepo)
	return err
}

// --- EventRepository ---

func (r *AuditRepository) CreateEvent(ctx context.Context, event *audit.Event) error {
	return r.CreateBatchEvents(ctx, []*audit.Event{event})
}

func (r *AuditRepository) CreateBatchEvents(ctx context.Context, events []*audit.Event) error {
	if len(events) == 0 {
		return nil
	}

	builder := r.builder().Insert("audit_events").Columns(
		"id", "created_at", "actor_id", "actor_type", "action", "outcome", "outcome_reason",
		"target_id", "target_type", "ip_address", "user_agent", "trace_id",
		"encrypted_data", "encryption_context",
	)

	for _, event := range events {
		encContextBytes, err := json.Marshal(event.EncryptionContext)
		if err != nil {
			return &audit.RepositoryError{Operation: "MarshalContext", Err: err}
		}
		builder = builder.Values(
			event.ID, event.CreatedAt, event.ActorID, event.ActorType, event.Action, event.Outcome, event.OutcomeReason,
			event.TargetID, event.TargetType, event.IPAddress, event.UserAgent, event.TraceID,
			event.EncryptedData, encContextBytes,
		)
	}

	if _, err := builder.ExecContext(ctx); err != nil {
		return &audit.RepositoryError{Operation: "CreateBatchEvents", Err: err}
	}
	return nil
}

func (r *AuditRepository) GetEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error) {
	var dbe dbAuditEvent
	query, args, _ := r.builder().Select("*").From("audit_events").Where(squirrel.Eq{"id": eventID}).ToSql()

	if err := sqlx.GetContext(ctx, r.runner, &dbe, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, audit.ErrEventNotFound
		}
		return nil, &audit.RepositoryError{Operation: "GetEvent", Err: err}
	}
	return dbe.ToDomain()
}

func (r *AuditRepository) SearchEvents(ctx context.Context, filters audit.EventFilters) ([]*audit.Event, int, error) {
	countBuilder := r.builder().Select("COUNT(*)").From("audit_events")
	countBuilder = applyEventFilters(countBuilder, filters)
	countQuery, countArgs, _ := countBuilder.ToSql()

	var total int
	if err := sqlx.GetContext(ctx, r.runner, &total, countQuery, countArgs...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "SearchEvents.Count", Err: err}
	}

	if total == 0 {
		return []*audit.Event{}, 0, nil
	}

	dataBuilder := r.builder().Select("*").From("audit_events")
	dataBuilder = applyEventFilters(dataBuilder, filters)

	// FIX: Dynamic sorting
	sortOrder := "DESC"
	if strings.ToUpper(filters.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	dataBuilder = dataBuilder.OrderBy(fmt.Sprintf("created_at %s", sortOrder))

	if filters.Limit > 0 {
		dataBuilder = dataBuilder.Limit(uint64(filters.Limit))
	}
	if filters.Offset > 0 {
		dataBuilder = dataBuilder.Offset(uint64(filters.Offset))
	}

	dataQuery, dataArgs, _ := dataBuilder.ToSql()
	var dbEvents []dbAuditEvent
	if err := sqlx.SelectContext(ctx, r.runner, &dbEvents, dataQuery, dataArgs...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "SearchEvents.Select", Err: err}
	}

	events := make([]*audit.Event, len(dbEvents))
	for i, dbe := range dbEvents {
		e, err := dbe.ToDomain()
		if err != nil {
			return nil, 0, err
		}
		events[i] = e
	}
	return events, total, nil
}

func (r *AuditRepository) AnonymizeEventsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		UPDATE audit_events 
		SET 
			actor_id = '00000000-0000-0000-0000-000000000000', 
			encrypted_data = NULL,
			encryption_context = NULL,
			ip_address = NULL
		WHERE actor_id = $1
	`
	res, err := r.runner.ExecContext(ctx, query, userID)
	if err != nil {
		return 0, &audit.RepositoryError{Operation: "AnonymizeEvents", Err: err}
	}
	return res.RowsAffected()
}

// --- LegalHoldRepository ---

func (r *AuditRepository) CreateLegalHold(ctx context.Context, hold *audit.LegalHold) error {
	filterBytes, err := json.Marshal(hold.Filter)
	if err != nil {
		return &audit.RepositoryError{Operation: "MarshalFilter", Err: err}
	}

	_, err = r.builder().Insert("legal_holds").
		Columns("id", "name", "description", "hold_subject", "reason", "filter", "active", "created_by", "issued_at").
		Values(hold.ID, hold.Name, hold.Description, hold.HoldSubject, hold.Reason, filterBytes, hold.Active, hold.CreatedBy, hold.IssuedAt).
		ExecContext(ctx)

	if err != nil {
		return &audit.RepositoryError{Operation: "CreateLegalHold", Err: err}
	}
	return nil
}

func (r *AuditRepository) GetLegalHold(ctx context.Context, id uuid.UUID) (*audit.LegalHold, error) {
	var dbHold dbLegalHold
	query, args, _ := r.builder().Select("*").From("legal_holds").Where(squirrel.Eq{"id": id}).ToSql()

	if err := sqlx.GetContext(ctx, r.runner, &dbHold, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Correctly wrapping so assertIsAppError works
			return nil, audit.ErrLegalHoldNotFound
		}
		return nil, &audit.RepositoryError{Operation: "GetLegalHold", Err: err}
	}
	return dbHold.ToDomain()
}

func (r *AuditRepository) UpdateLegalHold(ctx context.Context, hold *audit.LegalHold) error {
	filterBytes, err := json.Marshal(hold.Filter)
	if err != nil {
		return &audit.RepositoryError{Operation: "MarshalFilter", Err: err}
	}

	_, err = r.builder().Update("legal_holds").
		Set("name", hold.Name).
		Set("description", hold.Description).
		Set("filter", filterBytes).
		Set("active", hold.Active).
		Set("released_at", hold.ReleasedAt).
		Set("released_by", hold.ReleasedBy).
		Where(squirrel.Eq{"id": hold.ID}).
		ExecContext(ctx)

	if err != nil {
		return &audit.RepositoryError{Operation: "UpdateLegalHold", Err: err}
	}
	return nil
}

func (r *AuditRepository) ListLegalHolds(ctx context.Context, limit, offset int) ([]*audit.LegalHold, int, error) {
	var total int
	countQuery, countArgs, _ := r.builder().Select("COUNT(*)").From("legal_holds").ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &total, countQuery, countArgs...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "ListLegalHolds.Count", Err: err}
	}

	queryBuilder := r.builder().Select("*").From("legal_holds").OrderBy("issued_at DESC")
	if limit > 0 {
		queryBuilder = queryBuilder.Limit(uint64(limit))
	}
	if offset > 0 {
		queryBuilder = queryBuilder.Offset(uint64(offset))
	}

	query, args, _ := queryBuilder.ToSql()
	var dbHolds []dbLegalHold
	if err := sqlx.SelectContext(ctx, r.runner, &dbHolds, query, args...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "ListLegalHolds.Select", Err: err}
	}

	holds := make([]*audit.LegalHold, len(dbHolds))
	for i, h := range dbHolds {
		domainHold, err := h.ToDomain()
		if err != nil {
			return nil, 0, err
		}
		holds[i] = domainHold
	}
	return holds, total, nil
}

// --- RetentionPolicyRepository ---

func (r *AuditRepository) CreateRetentionPolicy(ctx context.Context, policy *audit.RetentionPolicy) error {
	_, err := r.builder().Insert("retention_policies").
		Columns("id", "name", "description", "data_type", "duration_days", "is_active", "created_by", "created_at", "updated_at").
		Values(policy.ID, policy.Name, policy.Description, policy.DataType, int(policy.Duration.Hours()/24), policy.IsActive, policy.CreatedBy, policy.CreatedAt, policy.UpdatedAt).
		ExecContext(ctx)

	if err != nil {
		return &audit.RepositoryError{Operation: "CreateRetentionPolicy", Err: err}
	}
	return nil
}

func (r *AuditRepository) GetRetentionPolicy(ctx context.Context, id uuid.UUID) (*audit.RetentionPolicy, error) {
	var dbp dbRetentionPolicy
	query, args, _ := r.builder().Select("*").From("retention_policies").Where(squirrel.Eq{"id": id}).ToSql()

	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, audit.ErrRetentionPolicyNotFound
		}
		return nil, &audit.RepositoryError{Operation: "GetRetentionPolicy", Err: err}
	}
	return dbp.ToDomain(), nil
}

func (r *AuditRepository) UpdateRetentionPolicy(ctx context.Context, policy *audit.RetentionPolicy) error {
	res, err := r.builder().Update("retention_policies").
		Set("name", policy.Name).
		Set("description", policy.Description).
		Set("data_type", policy.DataType).
		Set("duration_days", int(policy.Duration.Hours()/24)).
		Set("is_active", policy.IsActive).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": policy.ID}).
		ExecContext(ctx)

	if err != nil {
		return &audit.RepositoryError{Operation: "UpdateRetentionPolicy", Err: err}
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return audit.ErrRetentionPolicyNotFound
	}
	return nil
}

func (r *AuditRepository) DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error {
	res, err := r.builder().Delete("retention_policies").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return &audit.RepositoryError{Operation: "DeleteRetentionPolicy", Err: err}
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return audit.ErrRetentionPolicyNotFound
	}
	return nil
}

func (r *AuditRepository) ListRetentionPolicies(ctx context.Context, limit, offset int) ([]*audit.RetentionPolicy, int, error) {
	var total int
	countQuery, countArgs, _ := r.builder().Select("COUNT(*)").From("retention_policies").ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &total, countQuery, countArgs...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "ListRetentionPolicies.Count", Err: err}
	}

	queryBuilder := r.builder().Select("*").From("retention_policies").OrderBy("name ASC")
	if limit > 0 {
		queryBuilder = queryBuilder.Limit(uint64(limit))
	}
	if offset > 0 {
		queryBuilder = queryBuilder.Offset(uint64(offset))
	}

	query, args, _ := queryBuilder.ToSql()
	var dbPolicies []dbRetentionPolicy
	if err := sqlx.SelectContext(ctx, r.runner, &dbPolicies, query, args...); err != nil {
		return nil, 0, &audit.RepositoryError{Operation: "ListRetentionPolicies.Select", Err: err}
	}

	policies := make([]*audit.RetentionPolicy, len(dbPolicies))
	for i, p := range dbPolicies {
		policies[i] = p.ToDomain()
	}
	return policies, total, nil
}

// --- DB DTOs ---

type dbLegalHold struct {
	ID          uuid.UUID      `db:"id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	HoldSubject string         `db:"hold_subject"`
	Reason      sql.NullString `db:"reason"`
	Filter      []byte         `db:"filter"` // JSONB
	Active      bool           `db:"active"`
	CreatedBy   uuid.UUID      `db:"created_by"`
	IssuedAt    time.Time      `db:"issued_at"`
	ReleasedAt  *time.Time     `db:"released_at"`
	ReleasedBy  *uuid.UUID     `db:"released_by"`
}

func (d *dbLegalHold) ToDomain() (*audit.LegalHold, error) {
	var filter audit.LogFilter
	if len(d.Filter) > 0 {
		if err := json.Unmarshal(d.Filter, &filter); err != nil {
			return nil, fmt.Errorf("unmarshal filter failed: %w", err)
		}
	}

	return &audit.LegalHold{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description.String,
		HoldSubject: d.HoldSubject,
		Reason:      d.Reason.String,
		Filter:      filter,
		Active:      d.Active,
		CreatedBy:   d.CreatedBy,
		IssuedAt:    d.IssuedAt,
		ReleasedAt:  d.ReleasedAt,
		ReleasedBy:  d.ReleasedBy,
	}, nil
}

type dbRetentionPolicy struct {
	ID           uuid.UUID      `db:"id"`
	Name         string         `db:"name"`
	Description  sql.NullString `db:"description"`
	DataType     string         `db:"data_type"`
	DurationDays int            `db:"duration_days"`
	IsActive     bool           `db:"is_active"`
	CreatedBy    uuid.UUID      `db:"created_by"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

func (d *dbRetentionPolicy) ToDomain() *audit.RetentionPolicy {
	return &audit.RetentionPolicy{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description.String,
		DataType:    d.DataType,
		Duration:    time.Duration(d.DurationDays) * 24 * time.Hour,
		IsActive:    d.IsActive,
		CreatedBy:   d.CreatedBy,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

type dbAuditEvent struct {
	ID                uuid.UUID      `db:"id"`
	CreatedAt         time.Time      `db:"created_at"`
	ActorID           uuid.UUID      `db:"actor_id"`
	ActorType         string         `db:"actor_type"`
	Action            string         `db:"action"`
	Outcome           string         `db:"outcome"`
	OutcomeReason     sql.NullString `db:"outcome_reason"`
	TargetID          uuid.NullUUID  `db:"target_id"`
	TargetType        sql.NullString `db:"target_type"`
	EncryptedData     []byte         `db:"encrypted_data"`
	EncryptionContext []byte         `db:"encryption_context"`
	IPAddress         sql.NullString `db:"ip_address"`
	UserAgent         sql.NullString `db:"user_agent"`
	TraceID           sql.NullString `db:"trace_id"`
}

func (d *dbAuditEvent) ToDomain() (*audit.Event, error) {
	var encContext security.EncryptionContext
	if len(d.EncryptionContext) > 0 {
		if err := json.Unmarshal(d.EncryptionContext, &encContext); err != nil {
			return nil, err
		}
	}

	evt := &audit.Event{
		ID:                d.ID,
		CreatedAt:         d.CreatedAt,
		ActorID:           d.ActorID,
		ActorType:         d.ActorType,
		Action:            core.EventAction(d.Action),
		Outcome:           audit.Outcome(d.Outcome),
		EncryptedData:     d.EncryptedData,
		EncryptionContext: &encContext,
	}
	if d.OutcomeReason.Valid {
		evt.OutcomeReason = &d.OutcomeReason.String
	}
	if d.TargetID.Valid {
		evt.TargetID = &d.TargetID.UUID
	}
	if d.TargetType.Valid {
		evt.TargetType = &d.TargetType.String
	}
	if d.IPAddress.Valid {
		evt.IPAddress = &d.IPAddress.String
	}
	if d.UserAgent.Valid {
		evt.UserAgent = &d.UserAgent.String
	}
	if d.TraceID.Valid {
		evt.TraceID = &d.TraceID.String
	}
	return evt, nil
}

func applyEventFilters(builder squirrel.SelectBuilder, filters audit.EventFilters) squirrel.SelectBuilder {
	if filters.ActorID != nil {
		builder = builder.Where(squirrel.Eq{"actor_id": *filters.ActorID})
	}
	if filters.TargetID != nil {
		builder = builder.Where(squirrel.Eq{"target_id": *filters.TargetID})
	}
	if filters.Action != nil {
		builder = builder.Where(squirrel.Eq{"action": *filters.Action})
	}
	if filters.Outcome != nil {
		builder = builder.Where(squirrel.Eq{"outcome": *filters.Outcome})
	}
	if filters.StartDate != nil {
		builder = builder.Where(squirrel.GtOrEq{"created_at": *filters.StartDate})
	}
	if filters.EndDate != nil {
		builder = builder.Where(squirrel.LtOrEq{"created_at": *filters.EndDate})
	}
	return builder
}
