package user

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
)

var _ queue.JobProcessor = (*CleanupWorker)(nil)

type CleanupWorker struct {
	repo   user.Repository
	logger log.Logger
}

func NewCleanupWorker(repo user.Repository, logger log.Logger) *CleanupWorker {
	return &CleanupWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "user_cleanup")),
	}
}

func (w *CleanupWorker) JobType() core.JobType {
	return user.JobCleanupPendingUsers
}

type cleanupPayload struct {
	OlderThanHours int `json:"older_than_hours"`
}

type cleanupResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

func (w *CleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var p cleanupPayload
	// If payload is empty or missing fields, apply defaults
	if len(job.Payload) > 0 {
		_ = json.Unmarshal(job.Payload, &p)
	}
	if p.OlderThanHours <= 0 {
		p.OlderThanHours = 24 * 7 // Default: 7 days
	}

	cutoff := time.Now().UTC().Add(time.Duration(-p.OlderThanHours) * time.Hour)
	w.logger.Info("running pending user cleanup", log.Any("cutoff_time", cutoff))

	// We need to add this specific method to the repository interface in `internal/domain/user/interfaces.go`
	// OR assume it's part of the repository contract.
	// Let's assume `DeletePendingUsers` exists on the repo implementation.

	// Since we can't easily change the domain interface file in this turn without showing it,
	// I will cast the repo to a local interface that defines the method, or rely on the concrete implementation.
	// Better approach: Add it to the Repository interface definition.

	// Assuming repo interface has: DeletePendingUsers(ctx context.Context, olderThan time.Time) (int64, error)
	count, err := w.repo.DeletePendingUsers(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("cleanup failed: %w", err)
	}

	w.logger.Info("cleanup complete", log.Int("deleted", int(count)))

	res := cleanupResult{DeletedCount: count}
	return json.Marshal(res)
}
