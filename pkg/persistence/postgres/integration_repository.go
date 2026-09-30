package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ integration.Repository = (*IntegrationRepository)(nil)

type IntegrationRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewIntegrationRepository(db *sqlx.DB, logger log.Logger) *IntegrationRepository {
	return &IntegrationRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "integration_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx implementation (standard across repos)
func (r *IntegrationRepository) WithTx(ctx context.Context, fn func(txRepo integration.Repository) error) (err error) {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return errors.New("cannot begin transaction from within another transaction")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
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
	txRepo := &IntegrationRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	return fn(txRepo)
}

// --- Integration CRUD ---

func (r *IntegrationRepository) CreateIntegration(ctx context.Context, i *integration.Integration) error {
	dbModel, err := dbIntegrationFromDomain(i)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("integrations").
		Columns("id", "provider", "name", "status", "credentials_encrypted", "encryption_context", "configuration", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Provider, dbModel.Name, dbModel.Status, dbModel.CredentialsEncrypted, dbModel.EncryptionContext, dbModel.Configuration, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}
	return nil
}

func (r *IntegrationRepository) GetIntegration(ctx context.Context, id uuid.UUID) (*integration.Integration, error) {
	query, args, _ := r.sq.Select("*").From("integrations").Where(squirrel.Eq{"id": id}).ToSql()

	var dbi dbIntegration
	if err := sqlx.GetContext(ctx, r.runner, &dbi, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, integration.ErrIntegrationNotFound
		}
		return nil, err
	}
	return dbi.toDomain()
}

func (r *IntegrationRepository) GetActiveIntegrationByProvider(ctx context.Context, provider core.ProviderType) (*integration.Integration, error) {
	// Retrieves the most recently updated active integration for a specific provider
	query, args, _ := r.sq.Select("*").From("integrations").
		Where(squirrel.Eq{
			"provider": provider,
			"status":   integration.StatusActive,
		}).
		OrderBy("updated_at DESC").
		Limit(1).
		ToSql()

	var dbi dbIntegration
	if err := sqlx.GetContext(ctx, r.runner, &dbi, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, integration.ErrIntegrationNotFound
		}
		return nil, err
	}
	return dbi.toDomain()
}

func (r *IntegrationRepository) ListIntegrations(ctx context.Context, filter integration.IntegrationFilter) ([]*integration.Integration, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("integrations")
	builder = applyIntegrationFilters(builder, filter)
	query, args, err := builder.
		OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, err
	}

	var results []dbIntegrationWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []*integration.Integration{}, 0, nil
	}

	integrations := make([]*integration.Integration, len(results))
	for i, res := range results {
		integ, err := res.toDomain()
		if err != nil {
			return nil, 0, err
		}
		integrations[i] = integ
	}

	return integrations, results[0].Total, nil
}

func (r *IntegrationRepository) UpdateIntegration(ctx context.Context, i *integration.Integration) error {
	i.UpdatedAt = time.Now().UTC()
	dbModel, err := dbIntegrationFromDomain(i)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("integrations").
		Set("name", dbModel.Name).
		Set("status", dbModel.Status).
		Set("credentials_encrypted", dbModel.CredentialsEncrypted).
		Set("encryption_context", dbModel.EncryptionContext).
		Set("configuration", dbModel.Configuration).
		Set("last_error", dbModel.LastError).
		Set("last_error_at", dbModel.LastErrorAt).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update integration: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return integration.ErrIntegrationNotFound
	}
	return nil
}

func (r *IntegrationRepository) DeleteIntegration(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("integrations").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return integration.ErrIntegrationNotFound
	}
	return nil
}

// --- Webhook CRUD ---

func (r *IntegrationRepository) CreateWebhook(ctx context.Context, w *integration.Webhook) error {
	dbModel, err := dbWebhookFromDomain(w)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("webhooks").
		Columns("id", "integration_id", "provider", "provider_event_id", "event_type", "status", "payload", "error_message", "retry_count", "received_at", "processed_at").
		Values(dbModel.ID, dbModel.IntegrationID, dbModel.Provider, dbModel.ProviderEventID, dbModel.EventType, dbModel.Status, dbModel.Payload, dbModel.ErrorMessage, dbModel.RetryCount, dbModel.ReceivedAt, dbModel.ProcessedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return integration.ErrDuplicateWebhook
		}
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	return nil
}

func (r *IntegrationRepository) GetWebhook(ctx context.Context, id uuid.UUID) (*integration.Webhook, error) {
	query, args, _ := r.sq.Select("*").From("webhooks").Where(squirrel.Eq{"id": id}).ToSql()

	var dbw dbWebhook
	if err := sqlx.GetContext(ctx, r.runner, &dbw, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, integration.ErrWebhookNotFound
		}
		return nil, err
	}
	return dbw.toDomain()
}

func (r *IntegrationRepository) GetWebhookByProviderID(ctx context.Context, provider core.ProviderType, eventID string) (*integration.Webhook, error) {
	query, args, _ := r.sq.Select("*").From("webhooks").
		Where(squirrel.Eq{"provider": provider, "provider_event_id": eventID}).
		ToSql()

	var dbw dbWebhook
	if err := sqlx.GetContext(ctx, r.runner, &dbw, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, integration.ErrWebhookNotFound
		}
		return nil, err
	}
	return dbw.toDomain()
}

func (r *IntegrationRepository) UpdateWebhook(ctx context.Context, w *integration.Webhook) error {
	dbModel, err := dbWebhookFromDomain(w)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("webhooks").
		Set("status", dbModel.Status).
		Set("error_message", dbModel.ErrorMessage).
		Set("retry_count", dbModel.RetryCount).
		Set("processed_at", dbModel.ProcessedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return integration.ErrWebhookNotFound
	}
	return nil
}

// --- DB Models & Mappers ---

type dbIntegration struct {
	ID                   uuid.UUID      `db:"id"`
	Provider             string         `db:"provider"`
	Name                 string         `db:"name"`
	Status               string         `db:"status"`
	CredentialsEncrypted []byte         `db:"credentials_encrypted"`
	EncryptionContext    []byte         `db:"encryption_context"` // JSONB
	Configuration        []byte         `db:"configuration"`      // JSONB
	LastError            sql.NullString `db:"last_error"`
	LastErrorAt          sql.NullTime   `db:"last_error_at"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
}

type dbIntegrationWithTotal struct {
	dbIntegration
	Total int `db:"total_count"`
}

func dbIntegrationFromDomain(i *integration.Integration) (*dbIntegration, error) {
	var encCtxBytes []byte
	var err error
	if i.EncryptionContext != nil {
		encCtxBytes, err = json.Marshal(i.EncryptionContext)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal encryption context: %w", err)
		}
	} else {
		encCtxBytes = []byte("{}")
	}

	configBytes := i.Configuration
	if configBytes == nil {
		configBytes = []byte("{}")
	}

	return &dbIntegration{
		ID:                   i.ID,
		Provider:             string(i.Provider),
		Name:                 i.Name,
		Status:               string(i.Status),
		CredentialsEncrypted: i.CredentialsEncrypted,
		EncryptionContext:    encCtxBytes,
		Configuration:        configBytes,
		LastError:            toNullString(i.LastError),
		LastErrorAt:          toNullTime(i.LastErrorAt),
		CreatedAt:            i.CreatedAt,
		UpdatedAt:            i.UpdatedAt,
	}, nil
}

func (d *dbIntegration) toDomain() (*integration.Integration, error) {
	var encCtx security.EncryptionContext
	if len(d.EncryptionContext) > 0 {
		if err := json.Unmarshal(d.EncryptionContext, &encCtx); err != nil {
			return nil, fmt.Errorf("failed to unmarshal encryption context: %w", err)
		}
	}

	var lastErr *string
	if d.LastError.Valid {
		lastErr = &d.LastError.String
	}

	var lastErrAt *time.Time
	if d.LastErrorAt.Valid {
		lastErrAt = &d.LastErrorAt.Time
	}

	return &integration.Integration{
		ID:                   d.ID,
		Provider:             core.ProviderType(d.Provider),
		Name:                 d.Name,
		Status:               integration.IntegrationStatus(d.Status),
		CredentialsEncrypted: d.CredentialsEncrypted,
		EncryptionContext:    &encCtx,
		Configuration:        d.Configuration,
		LastError:            lastErr,
		LastErrorAt:          lastErrAt,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
	}, nil
}

type dbWebhook struct {
	ID              uuid.UUID      `db:"id"`
	IntegrationID   uuid.NullUUID  `db:"integration_id"`
	Provider        string         `db:"provider"`
	ProviderEventID string         `db:"provider_event_id"`
	EventType       string         `db:"event_type"`
	Status          string         `db:"status"`
	Payload         []byte         `db:"payload"`
	ErrorMessage    sql.NullString `db:"error_message"`
	RetryCount      int            `db:"retry_count"`
	ReceivedAt      time.Time      `db:"received_at"`
	ProcessedAt     sql.NullTime   `db:"processed_at"`
}

func dbWebhookFromDomain(w *integration.Webhook) (*dbWebhook, error) {
	payloadBytes := w.Payload
	if payloadBytes == nil {
		payloadBytes = []byte("{}")
	}

	var intID uuid.NullUUID
	if w.IntegrationID != uuid.Nil {
		intID.UUID = w.IntegrationID
		intID.Valid = true
	}

	return &dbWebhook{
		ID:              w.ID,
		IntegrationID:   intID,
		Provider:        string(w.Provider),
		ProviderEventID: w.ProviderEventID,
		EventType:       w.EventType,
		Status:          string(w.Status),
		Payload:         payloadBytes,
		ErrorMessage:    toNullString(w.ErrorMessage),
		RetryCount:      w.RetryCount,
		ReceivedAt:      w.ReceivedAt,
		ProcessedAt:     toNullTime(w.ProcessedAt),
	}, nil
}

func (d *dbWebhook) toDomain() (*integration.Webhook, error) {
	var errMsg *string
	if d.ErrorMessage.Valid {
		errMsg = &d.ErrorMessage.String
	}
	var procAt *time.Time
	if d.ProcessedAt.Valid {
		procAt = &d.ProcessedAt.Time
	}

	return &integration.Webhook{
		ID:              d.ID,
		IntegrationID:   d.IntegrationID.UUID,
		Provider:        core.ProviderType(d.Provider),
		ProviderEventID: d.ProviderEventID,
		EventType:       d.EventType,
		Status:          integration.WebhookStatus(d.Status),
		Payload:         d.Payload,
		ErrorMessage:    errMsg,
		RetryCount:      d.RetryCount,
		ReceivedAt:      d.ReceivedAt,
		ProcessedAt:     procAt,
	}, nil
}

// Helpers
func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func applyIntegrationFilters(builder squirrel.SelectBuilder, filter integration.IntegrationFilter) squirrel.SelectBuilder {
	if filter.Provider != nil {
		builder = builder.Where(squirrel.Eq{"provider": *filter.Provider})
	}
	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
	}
	return builder
}
