package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Ensure implementation satisfies interface
var _ messaging.Repository = (*MessagingRepository)(nil)

type MessagingRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext // Supports both *sqlx.DB and *sqlx.Tx
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewMessagingRepository(db *sqlx.DB, logger log.Logger) *MessagingRepository {
	return &MessagingRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "messaging_repo")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// WithTx allows wrapping operations in a transaction.
func (r *MessagingRepository) WithTx(ctx context.Context, fn func(txRepo messaging.Repository) error) (err error) {
	// Check if already in a transaction to support nested calls (if driver supports savepoints, otherwise blocking)
	// For simplicity, we assume we are at the top level or the driver handles it.
	// Safe casting:
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return fn(r) // Already in Tx, just run it
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("messaging: begin tx failed: %w", err)
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

	txRepo := &MessagingRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	return fn(txRepo)
}

// --- Channels ---

func (r *MessagingRepository) CreateChannel(ctx context.Context, c *messaging.Channel) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.LastMessageAt.IsZero() {
		c.LastMessageAt = now
	}

	// The named parameters here (:avatar_url) MUST match the db struct tag `db:"avatar_url"`
	query := `
		INSERT INTO channels 
		(id, type, name, description, avatar_url, ref_type, ref_id, last_message_at, created_at, updated_at)
		VALUES 
		(:id, :type, :name, :description, :avatar_url, :ref_type, :ref_id, :last_message_at, :created_at, :updated_at)
	`
	_, err := sqlx.NamedExecContext(ctx, r.runner, query, c)
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	return nil
}

func (r *MessagingRepository) GetChannel(ctx context.Context, id uuid.UUID) (*messaging.Channel, error) {
	var c messaging.Channel
	query := `SELECT * FROM channels WHERE id = $1`
	if err := sqlx.GetContext(ctx, r.runner, &c, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, messaging.ErrChannelNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) FindDirectChannel(ctx context.Context, userA, userB uuid.UUID) (*messaging.Channel, error) {
	// Find a DIRECT channel where BOTH users are members.
	query := `
		SELECT c.* 
		FROM channels c
		JOIN channel_members cm1 ON c.id = cm1.channel_id
		JOIN channel_members cm2 ON c.id = cm2.channel_id
		WHERE c.type = $1 
		  AND cm1.user_id = $2 
		  AND cm2.user_id = $3
		LIMIT 1
	`
	var c messaging.Channel
	if err := sqlx.GetContext(ctx, r.runner, &c, query, messaging.TypeDirect, userA, userB); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, messaging.ErrChannelNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) ListChannelsForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]messaging.ChannelSummary, error) {
	// 1. Define a temporary struct that matches the FLAT query result exactly.
	// sqlx will map columns to db tags.
	type ChannelWithCount struct {
		messaging.Channel     // Embeds all channel fields
		UnreadCount       int `db:"unread_count"` // Explicitly map the computed column
	}

	query := `
		SELECT 
			c.*,
			(
				SELECT COUNT(*) 
				FROM messages m 
				WHERE m.channel_id = c.id 
				  AND m.created_at > cm.last_read_at
				  AND m.sender_id != $1
			) as unread_count
		FROM channels c
		JOIN channel_members cm ON c.id = cm.channel_id
		WHERE cm.user_id = $1
		ORDER BY c.last_message_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.runner.QueryxContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []messaging.ChannelSummary

	for rows.Next() {
		var raw ChannelWithCount
		// Now StructScan works because ChannelWithCount has both the embedded fields AND the unread_count field.
		if err := rows.StructScan(&raw); err != nil {
			return nil, err
		}

		// Map back to Domain DTO
		summary := messaging.ChannelSummary{
			Channel:     raw.Channel,
			UnreadCount: raw.UnreadCount,
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (r *MessagingRepository) UpdateChannelTimestamp(ctx context.Context, id uuid.UUID, t time.Time) error {
	_, err := r.runner.ExecContext(ctx, "UPDATE channels SET last_message_at = $1 WHERE id = $2", t, id)
	return err
}

// --- Members ---

func (r *MessagingRepository) AddMember(ctx context.Context, cm *messaging.ChannelMember) error {
	query := `
		INSERT INTO channel_members (channel_id, user_id, role, joined_at, last_read_at, notifications)
		VALUES (:channel_id, :user_id, :role, :joined_at, :last_read_at, :notifications)
		ON CONFLICT (channel_id, user_id) DO NOTHING
	`
	if cm.JoinedAt.IsZero() {
		cm.JoinedAt = time.Now().UTC()
	}
	if cm.LastReadAt.IsZero() {
		cm.LastReadAt = cm.JoinedAt
	}

	_, err := sqlx.NamedExecContext(ctx, r.runner, query, cm)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}
	return nil
}

func (r *MessagingRepository) RemoveMember(ctx context.Context, channelID, userID uuid.UUID) error {
	_, err := r.runner.ExecContext(ctx, "DELETE FROM channel_members WHERE channel_id = $1 AND user_id = $2", channelID, userID)
	return err
}

func (r *MessagingRepository) GetMember(ctx context.Context, channelID, userID uuid.UUID) (*messaging.ChannelMember, error) {
	var cm messaging.ChannelMember
	query := `SELECT * FROM channel_members WHERE channel_id = $1 AND user_id = $2`
	if err := sqlx.GetContext(ctx, r.runner, &cm, query, channelID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, messaging.ErrNotMember
		}
		return nil, err
	}
	return &cm, nil
}

func (r *MessagingRepository) GetChannelMembers(ctx context.Context, channelID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	query := `SELECT user_id FROM channel_members WHERE channel_id = $1`
	if err := sqlx.SelectContext(ctx, r.runner, &ids, query, channelID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *MessagingRepository) UpdateLastRead(ctx context.Context, channelID, userID uuid.UUID, t time.Time) error {
	query := `UPDATE channel_members SET last_read_at = $1 WHERE channel_id = $2 AND user_id = $3`
	_, err := r.runner.ExecContext(ctx, query, t, channelID, userID)
	return err
}

// --- Messages ---

func (r *MessagingRepository) CreateMessage(ctx context.Context, m *messaging.Message) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Ensure empty JSON
	if len(m.Metadata) == 0 {
		m.Metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO messages 
		(id, channel_id, sender_id, type, content, metadata, reply_to_id, created_at, updated_at)
		VALUES 
		(:id, :channel_id, :sender_id, :type, :content, :metadata, :reply_to_id, :created_at, :updated_at)
	`
	_, err := sqlx.NamedExecContext(ctx, r.runner, query, m)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Update Channel's Last Message Pointer
	_, _ = r.runner.ExecContext(ctx,
		"UPDATE channels SET last_message_id = $1, last_message_at = $2 WHERE id = $3",
		m.ID, m.CreatedAt, m.ChannelID)

	return nil
}

func (r *MessagingRepository) GetMessage(ctx context.Context, id uuid.UUID) (*messaging.Message, error) {
	var m messaging.Message
	query := `SELECT * FROM messages WHERE id = $1 AND deleted_at IS NULL`
	if err := sqlx.GetContext(ctx, r.runner, &m, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, messaging.ErrMessageNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *MessagingRepository) ListMessages(ctx context.Context, channelID uuid.UUID, beforeID *uuid.UUID, limit int) ([]messaging.Message, error) {
	// Tuple comparison for strict stable pagination: (created_at, id) < (cursor_time, cursor_id)

	args := []interface{}{channelID, limit}
	// Base query
	sqlStr := "SELECT * FROM messages WHERE channel_id = $1 AND deleted_at IS NULL"

	if beforeID != nil {
		// 1. Fetch cursor details
		var cursor struct {
			CreatedAt time.Time `db:"created_at"`
		}
		err := r.runner.QueryRowxContext(ctx, "SELECT created_at FROM messages WHERE id = $1", beforeID).StructScan(&cursor)
		if err == nil {
			// 2. Add cursor condition
			// We use $3 and $4 for the tuple comparison
			sqlStr += " AND (created_at, id) < ($3, $4)"
			args = append(args, cursor.CreatedAt, *beforeID)
		}
	}

	sqlStr += " ORDER BY created_at DESC, id DESC LIMIT $2"

	var msgs []messaging.Message
	if err := sqlx.SelectContext(ctx, r.runner, &msgs, sqlStr, args...); err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	return msgs, nil
}

func (r *MessagingRepository) SoftDeleteMessage(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE messages SET deleted_at = NOW() WHERE id = $1`
	res, err := r.runner.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return messaging.ErrMessageNotFound
	}
	return nil
}

func (r *MessagingRepository) ListMessagesSince(ctx context.Context, channelID uuid.UUID, since time.Time) ([]messaging.Message, error) {
	// Select messages created after 'since'
	// Ordered by creation time ascending (to replay history in order)
	query := `
		SELECT * FROM messages 
		WHERE channel_id = $1 
		  AND created_at > $2 
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	var msgs []messaging.Message
	if err := sqlx.SelectContext(ctx, r.runner, &msgs, query, channelID, since); err != nil {
		return nil, fmt.Errorf("failed to list messages since: %w", err)
	}
	return msgs, nil
}
