package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// Ensure implementation
var _ queue.JobProcessor = (*PartitionWorker)(nil)

type PartitionWorker struct {
	db     *sqlx.DB
	logger log.Logger
}

func NewPartitionWorker(db *sqlx.DB, logger log.Logger) *PartitionWorker {
	return &PartitionWorker{
		db:     db,
		logger: logger.With(log.String("component", "analytics_partition_worker")),
	}
}

func (w *PartitionWorker) JobType() core.JobType {
	return analytics.JobMaintainPartitions
}

// ProcessJob calls the Postgres function to create future partitions.
func (w *PartitionWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// Look ahead 2 months by default.
	// This provides a safety buffer. If the worker fails once, we still have time.
	monthsAhead := 2

	w.logger.Info("starting partition maintenance", log.Int("months_ahead", monthsAhead))

	// We call the PL/pgSQL function defined in the schema.
	// This function is idempotent (IF NOT EXISTS).
	query := `SELECT create_analytics_partitions($1)`

	if _, err := w.db.ExecContext(ctx, query, monthsAhead); err != nil {
		// DB Errors here are critical but retryable
		return nil, fmt.Errorf("failed to execute partition maintenance: %w", err)
	}

	w.logger.Info("partition maintenance completed successfully")

	return json.Marshal(map[string]string{
		"status": "success",
		"action": "partitions_verified",
	})
}
