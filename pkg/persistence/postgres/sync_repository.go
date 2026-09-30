package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ sync_domain.Repository = (*SyncRepository)(nil)

type SyncRepository struct {
	db     *sqlx.DB
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewSyncRepository(db *sqlx.DB, logger log.Logger) *SyncRepository {
	return &SyncRepository{
		db:     db,
		logger: logger,
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// EnsureDevice registers a device or updates its last_sync_at time.
// Returns error if device is revoked.
func (r *SyncRepository) EnsureDevice(ctx context.Context, userID uuid.UUID, deviceID string) error {
	// 1. Check if exists
	var isRevoked bool
	queryCheck := `SELECT is_revoked FROM sync_devices WHERE user_id = $1 AND device_id = $2`
	err := r.db.GetContext(ctx, &isRevoked, queryCheck, userID, deviceID)

	if err == nil {
		// Device exists
		if isRevoked {
			return errors.New("device is revoked")
		}
		// Update Timestamp
		_, err := r.db.ExecContext(ctx,
			`UPDATE sync_devices SET last_sync_at = NOW() WHERE user_id = $1 AND device_id = $2`,
			userID, deviceID)
		return err
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err // Real DB error
	}

	// 2. Create if not exists
	id := uuid.New()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO sync_devices (id, user_id, device_id, last_sync_at)
		VALUES ($1, $2, $3, NOW())
	`, id, userID, deviceID)
	return err
}

func (r *SyncRepository) LogSession(ctx context.Context, userID uuid.UUID, deviceID, sType, status string, count int, errMsg *string) error {
	id := uuid.New()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sync_logs (id, user_id, device_id, sync_type, status, ops_count, error_message, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, id, userID, deviceID, sType, status, count, errMsg)
	return err
}
