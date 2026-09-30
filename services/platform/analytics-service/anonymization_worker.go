package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

var _ queue.JobProcessor = (*AnonymizationWorker)(nil)

type AnonymizationWorker struct {
	db     *sqlx.DB
	logger log.Logger
	secret string
}

func NewAnonymizationWorker(db *sqlx.DB, logger log.Logger, secret string) *AnonymizationWorker {
	if secret == "" {
		logger.Warn("ANALYTICS: No anonymization secret provided! Using insecure default.")
		secret = "insecure_default"
	}
	return &AnonymizationWorker{
		db:     db,
		logger: logger.With(log.String("component", "analytics_anonymization_worker")),
		secret: secret,
	}
}

func (w *AnonymizationWorker) JobType() core.JobType {
	return analytics.JobAnonymizeOldData
}

type AnonymizePayload struct {
	OlderThanDays int `json:"older_than_days"`
}

func (w *AnonymizationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p AnonymizePayload
	if len(job.Payload) > 0 {
		_ = json.Unmarshal(job.Payload, &p)
	}

	if p.OlderThanDays <= 0 {
		p.OlderThanDays = 365
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -p.OlderThanDays)
	w.logger.Info("starting analytics anonymization", log.String("cutoff", cutoff.Format(time.RFC3339)))

	// 1. Set IP to 0.0.0.0
	// 2. Unlink UserID
	// 3. HMAC-SHA256 the SessionID using the worker's secret ($2)
	query := `
		UPDATE analytics_events
		SET 
			ip_address = '0.0.0.0',
			user_id = NULL,
			session_id = encode(hmac(session_id, $2, 'sha256'), 'hex')
		WHERE created_at < $1
		AND ip_address != '0.0.0.0'
	`

	// FIX: Pass BOTH 'cutoff' ($1) AND 'w.secret' ($2)
	res, err := w.db.ExecContext(ctx, query, cutoff, w.secret)
	if err != nil {
		return nil, fmt.Errorf("anonymization query failed (ensure pgcrypto enabled): %w", err)
	}

	affected, _ := res.RowsAffected()
	w.logger.Info("anonymization complete", log.Int("rows_anonymized", int(affected)))

	return json.Marshal(map[string]any{
		"status": "success",
		"rows":   affected,
	})
}
