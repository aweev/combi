package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

var _ queue.JobProcessor = (*AggregationWorker)(nil)

type AggregationWorker struct {
	repo   analytics.Repository
	logger log.Logger
}

func NewAggregationWorker(repo analytics.Repository, logger log.Logger) *AggregationWorker {
	return &AggregationWorker{
		repo:   repo,
		logger: logger.With(log.String("component", "analytics_aggregation_worker")),
	}
}

func (w *AggregationWorker) JobType() core.JobType {
	return analytics.JobAnalyticsAggregate
}

type AggregationPayload struct {
	// TargetDateStr allows re-running aggregations for specific past dates.
	// Format: "YYYY-MM-DD"
	TargetDateStr string `json:"target_date,omitempty"`
}

func (w *AggregationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p AggregationPayload
	if len(job.Payload) > 0 {
		_ = json.Unmarshal(job.Payload, &p)
	}

	// Default: Aggregate "Yesterday" (Since today is still active and incomplete)
	targetDate := time.Now().UTC().AddDate(0, 0, -1)

	if p.TargetDateStr != "" {
		parsed, err := time.Parse("2006-01-02", p.TargetDateStr)
		if err != nil {
			return nil, queue.NewNonRetryableError(fmt.Errorf("invalid target_date format (expected YYYY-MM-DD): %w", err))
		}
		targetDate = parsed
	}

	w.logger.Info("starting analytics aggregation", log.String("date", targetDate.Format("2006-01-02")))

	// The repository handles the complex SQL "INSERT INTO ... SELECT ..."
	if err := w.repo.AggregateStats(ctx, targetDate); err != nil {
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}

	w.logger.Info("analytics aggregation completed successfully")

	return json.Marshal(map[string]string{
		"status": "success",
		"date":   targetDate.Format("2006-01-02"),
	})
}
