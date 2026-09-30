package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ notification.Repository = (*NotificationRepository)(nil)

type NotificationRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewNotificationRepository(db *sqlx.DB, logger log.Logger) *NotificationRepository {
	return &NotificationRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "notification_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *NotificationRepository) WithTx(ctx context.Context, fn func(txRepo notification.Repository) error) (err error) {
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
	txRepo := &NotificationRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	return fn(txRepo)
}

// --- Notification Operations ---

func (r *NotificationRepository) CreateNotification(ctx context.Context, n *notification.Notification) error {
	dbModel, err := dbNotificationFromDomain(n)
	if err != nil {
		return err
	}
	query, args, err := r.sq.Insert("notifications").
		Columns("id", "recipient_id", "channel", "status", "template_id", "subject", "body", "data", "to_address", "category", "reference_entity_type", "reference_entity_id", "sent_at", "delivered_at", "read_at", "attempts", "last_error", "provider_name", "provider_id", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.RecipientID, dbModel.Channel, dbModel.Status, dbModel.TemplateID, dbModel.Subject, dbModel.Body, dbModel.Data, dbModel.ToAddress, dbModel.Category, dbModel.RefEntityType, dbModel.RefEntityID, dbModel.SentAt, dbModel.DeliveredAt, dbModel.ReadAt, dbModel.Attempts, dbModel.LastError, dbModel.ProviderName, dbModel.ProviderID, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *NotificationRepository) GetNotification(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	query, args, _ := r.sq.Select("*").From("notifications").Where(squirrel.Eq{"id": id}).ToSql()
	var dbn dbNotification
	if err := sqlx.GetContext(ctx, r.runner, &dbn, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrNotificationNotFound
		}
		return nil, err
	}
	return dbn.toDomain()
}

func (r *NotificationRepository) ListUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]notification.Notification, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("notifications").Where(squirrel.Eq{"recipient_id": userID})
	query, args, err := builder.OrderBy("created_at DESC").Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var results []dbNotificationWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []notification.Notification{}, 0, nil
	}

	notes := make([]notification.Notification, len(results))
	for i, res := range results {
		n, err := res.toDomain()
		if err != nil {
			return nil, 0, err
		}
		notes[i] = *n
	}
	return notes, results[0].Total, nil
}

func (r *NotificationRepository) UpdateNotification(ctx context.Context, n *notification.Notification) error {
	n.UpdatedAt = time.Now().UTC()
	dbModel, err := dbNotificationFromDomain(n)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("notifications").
		Set("status", dbModel.Status).
		Set("sent_at", dbModel.SentAt).
		Set("delivered_at", dbModel.DeliveredAt).
		Set("read_at", dbModel.ReadAt).
		Set("attempts", dbModel.Attempts).
		Set("last_error", dbModel.LastError).
		Set("provider_name", dbModel.ProviderName).
		Set("provider_id", dbModel.ProviderID).
		Set("updated_at", dbModel.UpdatedAt).
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
		return notification.ErrNotificationNotFound
	}
	return nil
}

func (r *NotificationRepository) GetNotificationByProviderID(ctx context.Context, providerName, providerID string) (*notification.Notification, error) {
	query, args, _ := r.sq.Select("*").From("notifications").
		Where(squirrel.Eq{"provider_name": providerName, "provider_id": providerID}).ToSql()

	var dbn dbNotification
	if err := sqlx.GetContext(ctx, r.runner, &dbn, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrNotificationNotFound
		}
		return nil, err
	}
	return dbn.toDomain()
}

func (r *NotificationRepository) CreateEvent(ctx context.Context, event *notification.NotificationEvent) error {
	dbModel, err := dbNotificationEventFromDomain(event)
	if err != nil {
		return err
	}
	query, args, err := r.sq.Insert("notification_events").
		Columns("id", "notification_id", "source", "event_type", "message", "details", "created_at").
		Values(dbModel.ID, dbModel.NotificationID, dbModel.Source, dbModel.EventType, dbModel.Message, dbModel.Details, dbModel.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *NotificationRepository) GetNotificationEvents(ctx context.Context, notificationID uuid.UUID) ([]notification.NotificationEvent, error) {
	query, args, _ := r.sq.Select("*").From("notification_events").
		Where(squirrel.Eq{"notification_id": notificationID}).
		OrderBy("created_at ASC").ToSql()

	var rows []dbNotificationEvent
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	events := make([]notification.NotificationEvent, len(rows))
	for i, row := range rows {
		evt, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		events[i] = *evt
	}
	return events, nil
}

func (r *NotificationRepository) GetScheduledNotifications(ctx context.Context, dueTime time.Time, limit int) ([]notification.Notification, error) {
	// Currently schema doesn't have `scheduled_for`. Assuming standard PENDING check or updated schema.
	// For now, let's assume we query PENDING notifications created before dueTime (simple queue).
	// If `scheduled_for` existed, we'd query `WHERE status = 'PENDING' AND scheduled_for <= dueTime`.

	// Assuming basic immediate send queue behavior:
	query, args, _ := r.sq.Select("*").From("notifications").
		Where(squirrel.Eq{"status": notification.StatusPending}).
		Where(squirrel.LtOrEq{"created_at": dueTime}).
		OrderBy("created_at ASC").
		Limit(uint64(limit)).
		ToSql()

	var rows []dbNotification
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	result := make([]notification.Notification, len(rows))
	for i, r := range rows {
		d, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		result[i] = *d
	}
	return result, nil
}

func (r *NotificationRepository) DeleteOldNotifications(ctx context.Context, olderThan time.Time) (int64, error) {
	query, args, _ := r.sq.Delete("notifications").Where(squirrel.Lt{"created_at": olderThan}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *NotificationRepository) DeleteOldEvents(ctx context.Context, olderThan time.Time) (int64, error) {
	query, args, _ := r.sq.Delete("notification_events").Where(squirrel.Lt{"created_at": olderThan}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *NotificationRepository) GetNotificationForUpdate(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	query, args, _ := r.sq.Select("*").From("notifications").
		Where(squirrel.Eq{"id": id}).
		Suffix("FOR UPDATE"). // Row locking
		ToSql()

	var dbn dbNotification
	if err := sqlx.GetContext(ctx, r.runner, &dbn, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrNotificationNotFound
		}
		return nil, err
	}
	return dbn.toDomain()
}

// --- Template Operations ---

func (r *NotificationRepository) CreateTemplate(ctx context.Context, t *notification.NotificationTemplate) error {
	dbModel := dbTemplateFromDomain(t)
	query, args, err := r.sq.Insert("notification_templates").
		Columns("id", "name", "slug", "description", "type", "category", "channel", "is_active", "locale", "subject", "body", "created_by", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Name, dbModel.Slug, dbModel.Description, dbModel.Type, dbModel.Category, dbModel.Channel, dbModel.IsActive, dbModel.Locale, dbModel.Subject, dbModel.Body, dbModel.CreatedBy, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return notification.ErrTemplateAlreadyExists
		}
		return err
	}
	return nil
}

func (r *NotificationRepository) GetTemplate(ctx context.Context, id uuid.UUID) (*notification.NotificationTemplate, error) {
	query, args, _ := r.sq.Select("*").From("notification_templates").Where(squirrel.Eq{"id": id}).ToSql()
	var dbt dbTemplate
	if err := sqlx.GetContext(ctx, r.runner, &dbt, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrTemplateNotFound
		}
		return nil, err
	}
	return dbt.toDomain(), nil
}

func (r *NotificationRepository) GetTemplateBySlug(ctx context.Context, slug string) (*notification.NotificationTemplate, error) {
	query, args, _ := r.sq.Select("*").From("notification_templates").Where(squirrel.Eq{"slug": slug}).ToSql()
	var dbt dbTemplate
	if err := sqlx.GetContext(ctx, r.runner, &dbt, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrTemplateNotFound
		}
		return nil, err
	}
	return dbt.toDomain(), nil
}

func (r *NotificationRepository) ListTemplates(ctx context.Context, limit, offset int) ([]notification.NotificationTemplate, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("notification_templates")
	query, args, err := builder.OrderBy("name ASC").Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var results []dbTemplateWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []notification.NotificationTemplate{}, 0, nil
	}

	temps := make([]notification.NotificationTemplate, len(results))
	for i, res := range results {
		temps[i] = *res.toDomain()
	}
	return temps, results[0].Total, nil
}

func (r *NotificationRepository) UpdateTemplate(ctx context.Context, t *notification.NotificationTemplate) error {
	t.UpdatedAt = time.Now().UTC()
	dbModel := dbTemplateFromDomain(t)
	query, args, err := r.sq.Update("notification_templates").
		Set("name", dbModel.Name).
		Set("slug", dbModel.Slug).
		Set("description", dbModel.Description).
		Set("is_active", dbModel.IsActive).
		Set("subject", dbModel.Subject).
		Set("body", dbModel.Body).
		Set("updated_at", dbModel.UpdatedAt).
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
		return notification.ErrTemplateNotFound
	}
	return nil
}

func (r *NotificationRepository) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("notification_templates").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		// If used by notifications, might violate FK.
		// Check error code if you want to return ErrTemplateInUse
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return notification.ErrTemplateNotFound
	}
	return nil
}

func (r *NotificationRepository) CreateTemplateTranslation(ctx context.Context, tt *notification.TemplateTranslation) error {
	query, args, _ := r.sq.Insert("template_translations").
		Columns("id", "template_id", "locale", "subject", "body", "created_at", "updated_at").
		Values(tt.ID, tt.TemplateID, tt.Locale, tt.Subject, tt.Body, tt.CreatedAt, tt.UpdatedAt).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *NotificationRepository) GetTemplateTranslations(ctx context.Context, templateID uuid.UUID) ([]notification.TemplateTranslation, error) {
	query, args, _ := r.sq.Select("*").From("template_translations").Where(squirrel.Eq{"template_id": templateID}).ToSql()
	var rows []dbTranslation
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	trans := make([]notification.TemplateTranslation, len(rows))
	for i, row := range rows {
		trans[i] = *row.toDomain()
	}
	return trans, nil
}

func (r *NotificationRepository) IsTemplateInUse(ctx context.Context, id uuid.UUID) (*notification.NotificationTemplate, error) {
	// Logic to check if template has associated active notifications or preferences.
	// For simplicity, assuming "In Use" implies "Exists" for now, or custom check query.
	return r.GetTemplate(ctx, id)
}

// --- Provider Operations ---

func (r *NotificationRepository) CreateProvider(ctx context.Context, p *notification.NotificationProvider) error {
	dbModel, err := dbProviderFromDomain(p)
	if err != nil {
		return err
	}
	query, args, err := r.sq.Insert("notification_providers").
		Columns("id", "name", "type", "channel", "is_active", "config_encrypted", "encryption_context", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Name, dbModel.Type, dbModel.Channel, dbModel.IsActive, dbModel.ConfigEncrypted, dbModel.EncryptionContext, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *NotificationRepository) GetProvider(ctx context.Context, id uuid.UUID) (*notification.NotificationProvider, error) {
	query, args, _ := r.sq.Select("*").From("notification_providers").Where(squirrel.Eq{"id": id}).ToSql()
	var dbp dbProvider
	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrProviderNotFound
		}
		return nil, err
	}
	return dbp.toDomain()
}

func (r *NotificationRepository) GetProviderByName(ctx context.Context, name string) (*notification.NotificationProvider, error) {
	query, args, _ := r.sq.Select("*").From("notification_providers").Where(squirrel.Eq{"name": name}).ToSql()
	var dbp dbProvider
	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrProviderNotFound
		}
		return nil, err
	}
	return dbp.toDomain()
}

func (r *NotificationRepository) ListProviders(ctx context.Context, channel *notification.NotificationChannel) ([]notification.NotificationProvider, error) {
	builder := r.sq.Select("*").From("notification_providers")
	if channel != nil {
		builder = builder.Where(squirrel.Eq{"channel": *channel})
	}
	query, args, _ := builder.OrderBy("name ASC").ToSql()

	var rows []dbProvider
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	provs := make([]notification.NotificationProvider, len(rows))
	for i, row := range rows {
		p, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		provs[i] = *p
	}
	return provs, nil
}

func (r *NotificationRepository) UpdateProvider(ctx context.Context, p *notification.NotificationProvider) error {
	p.UpdatedAt = time.Now().UTC()
	dbModel, err := dbProviderFromDomain(p)
	if err != nil {
		return err
	}
	query, args, err := r.sq.Update("notification_providers").
		Set("is_active", dbModel.IsActive).
		Set("config_encrypted", dbModel.ConfigEncrypted).
		Set("encryption_context", dbModel.EncryptionContext).
		Set("updated_at", dbModel.UpdatedAt).
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
		return notification.ErrProviderNotFound
	}
	return nil
}

func (r *NotificationRepository) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("notification_providers").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return notification.ErrProviderNotFound
	}
	return nil
}

func (r *NotificationRepository) FindProviderForDispatch(ctx context.Context, channel notification.NotificationChannel) (*notification.NotificationProvider, error) {
	// Logic: find first active provider for channel. Could be enhanced with priority/round-robin.
	query, args, _ := r.sq.Select("*").From("notification_providers").
		Where(squirrel.Eq{"channel": channel, "is_active": true}).
		Limit(1).
		ToSql()

	var dbp dbProvider
	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notification.ErrNoProviderConfigured
		}
		return nil, err
	}
	return dbp.toDomain()
}

func (r *NotificationRepository) IsProviderInUse(ctx context.Context, id uuid.UUID) (*notification.NotificationProvider, error) {
	return r.GetProvider(ctx, id)
}

// --- Preference Operations ---

func (r *NotificationRepository) GetUserPreferences(ctx context.Context, userID uuid.UUID) ([]notification.NotificationPreference, error) {
	query, args, _ := r.sq.Select("*").From("notification_preferences").Where(squirrel.Eq{"user_id": userID}).ToSql()
	var rows []dbPreference
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	prefs := make([]notification.NotificationPreference, len(rows))
	for i, row := range rows {
		prefs[i] = *row.toDomain()
	}
	return prefs, nil
}

func (r *NotificationRepository) UpsertPreference(ctx context.Context, p *notification.NotificationPreference) (*notification.NotificationPreference, error) {
	// Using Postgres ON CONFLICT for upsert
	dbModel := dbPreferenceFromDomain(p)

	// Note: UNIQUE constraint on (user_id, channel, scope, category, template_id) handles conflict.
	// Since category/template_id can be NULL, unique index handling in PG requires coalescing or partial indexes if strictness needed.
	// Assuming the migration handles uniqueness correctly (Postgres treats NULLs as distinct in unique indexes by default unless version 15+ with NULLS NOT DISTINCT, or partial indexes used).
	// For this snippet, relying on ID or assuming conflict handling strategy.

	// Better approach: Check existence or use ID if updating specific pref.
	// If creating/overwriting based on user/scope keys:
	query, args, err := r.sq.Insert("notification_preferences").
		Columns("id", "user_id", "channel", "scope", "category", "template_id", "is_enabled", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.UserID, dbModel.Channel, dbModel.Scope, dbModel.Category, dbModel.TemplateID, dbModel.IsEnabled, dbModel.CreatedAt, dbModel.UpdatedAt).
		Suffix("ON CONFLICT (id) DO UPDATE SET is_enabled = EXCLUDED.is_enabled, updated_at = EXCLUDED.updated_at").
		ToSql()
	if err != nil {
		return nil, err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// --- Stats ---

func (r *NotificationRepository) GetSystemStats(ctx context.Context, start, end time.Time) (*notification.NotificationStats, error) {
	query, args, _ := r.sq.Select("status, COUNT(*)").
		From("notifications").
		Where(squirrel.GtOrEq{"created_at": start}).
		Where(squirrel.LtOrEq{"created_at": end}).
		GroupBy("status").
		ToSql()

	rows, err := r.runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &notification.NotificationStats{ByStatus: make(map[notification.NotificationStatus]int64)}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		st := notification.NotificationStatus(status)
		stats.ByStatus[st] = count

		switch st {
		case notification.StatusSent:
			stats.TotalSent += count
		case notification.StatusDelivered:
			stats.TotalDelivered += count
			stats.TotalSent += count // Delivered implies sent
		case notification.StatusFailed, notification.StatusBounced:
			stats.TotalFailed += count
		case notification.StatusRead:
			stats.TotalRead += count
			stats.TotalDelivered += count
			stats.TotalSent += count
		}
	}
	return stats, nil
}

func (r *NotificationRepository) CreateNotificationAndOutboxJob(ctx context.Context, n *notification.Notification, job *notification.OutboxJobPayload, jobQueue string, priority int) error {
	return r.WithTx(ctx, func(txRepo notification.Repository) error {
		// 1. Create Notification
		if err := txRepo.CreateNotification(ctx, n); err != nil {
			return err
		}

		// 2. Create Job AND Outbox Event
		if job != nil {
			jobID := uuid.New()
			now := time.Now().UTC()

			if jobQueue == "" {
				jobQueue = "default"
			}

			// A. Insert into JOBS table
			jobQuery, args, err := r.sq.Insert("jobs").
				Columns(
					"id", "type", "queue", "priority", "payload", "status",
					"max_attempts", "timeout_seconds", "created_at", "updated_at",
				).
				Values(
					jobID,
					job.JobType,
					jobQueue,
					priority,
					job.Payload,
					"PENDING",
					3,
					300,
					now,
					now,
				).
				ToSql()

			if err != nil {
				return fmt.Errorf("failed to build job insert query: %w", err)
			}
			if _, err := r.runner.ExecContext(ctx, jobQuery, args...); err != nil {
				return fmt.Errorf("failed to enqueue notification job: %w", err)
			}

			// B. Insert into OUTBOX_EVENTS table (THIS WAS MISSING)
			outboxID := uuid.New()
			outboxQuery, args, err := r.sq.Insert("outbox_events").
				Columns(
					"id", "job_id", "queue_name", "priority", "status", 
					"attempts", "created_at",
				).
				Values(
					outboxID,
					jobID,
					jobQueue,
					priority,
					"PENDING", // Status
					0,         // Attempts
					now,
				).
				ToSql()

			if err != nil {
				return fmt.Errorf("failed to build outbox insert query: %w", err)
			}
			if _, err := r.runner.ExecContext(ctx, outboxQuery, args...); err != nil {
				return fmt.Errorf("failed to create outbox event: %w", err)
			}
		}
		return nil
	})
}

// CountUnread returns the number of notifications for a user that have not been read.
func (r *NotificationRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query, args, _ := r.sq.Select("COUNT(*)").
		From("notifications").
		Where(squirrel.Eq{
			"recipient_id": userID,
			"read_at":      nil,
		}).
		ToSql()

		// Use sqlx's GetContext which handles scanning a single row into a destination variable
	if err := sqlx.GetContext(ctx, r.runner, &count, query, args...); err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

// MarkAllAsRead updates all unread notifications for a user to READ status.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()

	// We update 'status', 'read_at', and 'updated_at'
	query, args, _ := r.sq.Update("notifications").
		Set("status", "READ").
		Set("read_at", now).
		Set("updated_at", now).
		Where(squirrel.Eq{
			"recipient_id": userID,
			"read_at":      nil, // Only update unread ones
		}).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}
	return nil
}

// --- DB Models & Mappers ---

type dbNotification struct {
	ID            uuid.UUID      `db:"id"`
	RecipientID   uuid.UUID      `db:"recipient_id"`
	Channel       string         `db:"channel"`
	Status        string         `db:"status"`
	TemplateID    uuid.NullUUID  `db:"template_id"`
	Subject       sql.NullString `db:"subject"`
	Body          string         `db:"body"`
	Data          []byte         `db:"data"`
	ToAddress     string         `db:"to_address"`
	Category      string         `db:"category"`
	RefEntityType sql.NullString `db:"reference_entity_type"`
	RefEntityID   uuid.NullUUID  `db:"reference_entity_id"`
	SentAt        sql.NullTime   `db:"sent_at"`
	DeliveredAt   sql.NullTime   `db:"delivered_at"`
	ReadAt        sql.NullTime   `db:"read_at"`
	Attempts      int            `db:"attempts"`
	LastError     sql.NullString `db:"last_error"`
	ProviderName  sql.NullString `db:"provider_name"`
	ProviderID    sql.NullString `db:"provider_id"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

type dbNotificationWithTotal struct {
	dbNotification
	Total int `db:"total_count"`
}

func dbNotificationFromDomain(n *notification.Notification) (*dbNotification, error) {
	dataBytes, err := json.Marshal(n.Data)
	if err != nil {
		return nil, err
	}

	var tmplID uuid.NullUUID
	if n.TemplateID != nil {
		tmplID.UUID = *n.TemplateID
		tmplID.Valid = true
	}

	var refType sql.NullString
	var refID uuid.NullUUID
	if n.Reference != nil {
		refType.String = n.Reference.EntityType
		refType.Valid = true
		refID.UUID = n.Reference.EntityID
		refID.Valid = true
	}

	return &dbNotification{
		ID:            n.ID,
		RecipientID:   n.RecipientID,
		Channel:       string(n.Channel),
		Status:        string(n.Status),
		TemplateID:    tmplID,
		Subject:       toNullString(n.Subject),
		Body:          n.Body,
		Data:          dataBytes,
		ToAddress:     n.ToAddress.String(),
		Category:      n.Category,
		RefEntityType: refType,
		RefEntityID:   refID,
		SentAt:        toNullTime(n.SentAt),
		DeliveredAt:   toNullTime(n.DeliveredAt),
		ReadAt:        toNullTime(n.ReadAt),
		Attempts:      n.Attempts,
		LastError:     toNullString(n.LastError),
		ProviderName:  sql.NullString{String: n.ProviderName, Valid: n.ProviderName != ""},
		ProviderID:    toNullString(n.ProviderID),
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
	}, nil
}

func (d *dbNotification) toDomain() (*notification.Notification, error) {
	var data map[string]any
	if len(d.Data) > 0 {
		if err := json.Unmarshal(d.Data, &data); err != nil {
			return nil, err
		}
	}

	toEmail, err := core.NewEmail(d.ToAddress)
	if err != nil {
		return nil, err
	}

	var tmplID *uuid.UUID
	if d.TemplateID.Valid {
		tmplID = &d.TemplateID.UUID
	}

	var ref *notification.NotificationRef
	if d.RefEntityType.Valid && d.RefEntityID.Valid {
		ref = &notification.NotificationRef{
			EntityType: d.RefEntityType.String,
			EntityID:   d.RefEntityID.UUID,
		}
	}

	var provID *string
	if d.ProviderID.Valid {
		provID = &d.ProviderID.String
	}
	var provName string
	if d.ProviderName.Valid {
		provName = d.ProviderName.String
	}

	var lastErr *string
	if d.LastError.Valid {
		lastErr = &d.LastError.String
	}

	var subj *string
	if d.Subject.Valid {
		subj = &d.Subject.String
	}

	var sentAt *time.Time
	if d.SentAt.Valid {
		sentAt = &d.SentAt.Time
	}
	var delAt *time.Time
	if d.DeliveredAt.Valid {
		delAt = &d.DeliveredAt.Time
	}
	var readAt *time.Time
	if d.ReadAt.Valid {
		readAt = &d.ReadAt.Time
	}

	return &notification.Notification{
		ID:           d.ID,
		RecipientID:  d.RecipientID,
		Channel:      notification.NotificationChannel(d.Channel),
		Status:       notification.NotificationStatus(d.Status),
		TemplateID:   tmplID,
		Subject:      subj,
		Body:         d.Body,
		Data:         data,
		ToAddress:    toEmail,
		Category:     d.Category,
		Reference:    ref,
		SentAt:       sentAt,
		DeliveredAt:  delAt,
		ReadAt:       readAt,
		Attempts:     d.Attempts,
		LastError:    lastErr,
		ProviderName: provName,
		ProviderID:   provID,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}, nil
}

type dbProvider struct {
	ID                uuid.UUID `db:"id"`
	Name              string    `db:"name"`
	Type              string    `db:"type"`
	Channel           string    `db:"channel"`
	IsActive          bool      `db:"is_active"`
	ConfigEncrypted   []byte    `db:"config_encrypted"`
	EncryptionContext []byte    `db:"encryption_context"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

func dbProviderFromDomain(p *notification.NotificationProvider) (*dbProvider, error) {
	encCtxBytes, err := json.Marshal(p.Config.EncryptionContext)
	if err != nil {
		return nil, err
	}
	return &dbProvider{
		ID:                p.ID,
		Name:              p.Name,
		Type:              string(p.Type),
		Channel:           string(p.Channel),
		IsActive:          p.IsActive,
		ConfigEncrypted:   p.Config.EncryptedData,
		EncryptionContext: encCtxBytes,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}, nil
}

func (d *dbProvider) toDomain() (*notification.NotificationProvider, error) {
	var encCtx security.EncryptionContext
	if err := json.Unmarshal(d.EncryptionContext, &encCtx); err != nil {
		return nil, err
	}
	return &notification.NotificationProvider{
		ID:       d.ID,
		Name:     d.Name,
		Type:     notification.ProviderType(d.Type),
		Channel:  notification.NotificationChannel(d.Channel),
		IsActive: d.IsActive,
		Config: notification.EncryptedConfig{
			EncryptedData:     d.ConfigEncrypted,
			EncryptionContext: &encCtx,
		},
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}

type dbTemplate struct {
	ID          uuid.UUID      `db:"id"`
	Name        string         `db:"name"`
	Slug        string         `db:"slug"`
	Description sql.NullString `db:"description"`
	Type        string         `db:"type"`
	Category    string         `db:"category"`
	Channel     string         `db:"channel"`
	IsActive    bool           `db:"is_active"`
	Locale      string         `db:"locale"`
	Subject     sql.NullString `db:"subject"`
	Body        string         `db:"body"`
	Version     int            `db:"version"`
	CreatedBy   uuid.UUID      `db:"created_by"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

type dbTemplateWithTotal struct {
	dbTemplate
	Total int `db:"total_count"`
}

func dbTemplateFromDomain(t *notification.NotificationTemplate) *dbTemplate {
	return &dbTemplate{
		ID:          t.ID,
		Name:        t.Name,
		Slug:        t.Slug,
		Description: toNullString(&t.Description),
		Type:        string(t.Type),
		Category:    t.Category,
		Channel:     string(t.Channel),
		IsActive:    t.IsActive,
		Locale:      t.Locale,
		Subject:     toNullString(t.Subject),
		Version:     t.Version,
		Body:        t.Body,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (d *dbTemplate) toDomain() *notification.NotificationTemplate {
	return &notification.NotificationTemplate{
		ID:          d.ID,
		Name:        d.Name,
		Slug:        d.Slug,
		Description: d.Description.String,
		Type:        notification.TemplateType(d.Type),
		Category:    d.Category,
		Channel:     notification.NotificationChannel(d.Channel),
		IsActive:    d.IsActive,
		Locale:      d.Locale,
		Subject:     &d.Subject.String,
		Body:        d.Body,
		Version:     d.Version,
		CreatedBy:   d.CreatedBy,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

type dbNotificationEvent struct {
	ID             uuid.UUID `db:"id"`
	NotificationID uuid.UUID `db:"notification_id"`
	Source         string    `db:"source"`
	EventType      string    `db:"event_type"`
	Message        string    `db:"message"`
	Details        []byte    `db:"details"`
	CreatedAt      time.Time `db:"created_at"`
}

func dbNotificationEventFromDomain(e *notification.NotificationEvent) (*dbNotificationEvent, error) {
	detailsBytes, err := json.Marshal(e.Details)
	if err != nil {
		return nil, err
	}
	return &dbNotificationEvent{
		ID:             e.ID,
		NotificationID: e.NotificationID,
		Source:         string(e.Source),
		EventType:      e.EventType,
		Message:        e.Message,
		Details:        detailsBytes,
		CreatedAt:      e.CreatedAt,
	}, nil
}

func (d *dbNotificationEvent) toDomain() (*notification.NotificationEvent, error) {
	var details map[string]any
	if err := json.Unmarshal(d.Details, &details); err != nil {
		return nil, err
	}
	return &notification.NotificationEvent{
		ID:             d.ID,
		NotificationID: d.NotificationID,
		Source:         notification.EventSource(d.Source),
		EventType:      d.EventType,
		Message:        d.Message,
		Details:        details,
		CreatedAt:      d.CreatedAt,
	}, nil
}

type dbTranslation struct {
	ID         uuid.UUID      `db:"id"`
	TemplateID uuid.UUID      `db:"template_id"`
	Locale     string         `db:"locale"`
	Subject    sql.NullString `db:"subject"`
	Body       string         `db:"body"`
	CreatedAt  time.Time      `db:"created_at"`
	UpdatedAt  time.Time      `db:"updated_at"`
}

func (d *dbTranslation) toDomain() *notification.TemplateTranslation {
	var subj *string
	if d.Subject.Valid {
		subj = &d.Subject.String
	}
	return &notification.TemplateTranslation{
		ID:         d.ID,
		TemplateID: d.TemplateID,
		Locale:     d.Locale,
		Subject:    subj,
		Body:       d.Body,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

type dbPreference struct {
	ID         uuid.UUID      `db:"id"`
	UserID     uuid.UUID      `db:"user_id"`
	Channel    string         `db:"channel"`
	Scope      string         `db:"scope"`
	Category   sql.NullString `db:"category"`
	TemplateID uuid.NullUUID  `db:"template_id"`
	IsEnabled  bool           `db:"is_enabled"`
	CreatedAt  time.Time      `db:"created_at"`
	UpdatedAt  time.Time      `db:"updated_at"`
}

func dbPreferenceFromDomain(p *notification.NotificationPreference) *dbPreference {
	return &dbPreference{
		ID:         p.ID,
		UserID:     p.UserID,
		Channel:    string(p.Channel),
		Scope:      string(p.Scope),
		Category:   toNullString(p.Category),
		TemplateID: toNullUUID(p.TemplateID),
		IsEnabled:  p.IsEnabled,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}

func (d *dbPreference) toDomain() *notification.NotificationPreference {
	var cat *string
	if d.Category.Valid {
		cat = &d.Category.String
	}
	var tid *uuid.UUID
	if d.TemplateID.Valid {
		tid = &d.TemplateID.UUID
	}
	return &notification.NotificationPreference{
		ID:         d.ID,
		UserID:     d.UserID,
		Channel:    notification.NotificationChannel(d.Channel),
		Scope:      notification.PreferenceScope(d.Scope),
		Category:   cat,
		TemplateID: tid,
		IsEnabled:  d.IsEnabled,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// Helpers
func toNullUUID(u *uuid.UUID) uuid.NullUUID {
	if u == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *u, Valid: true}
}
