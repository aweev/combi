package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// --- Token Cleanup Worker ---

var _ queue.JobProcessor = (*TokenCleanupWorker)(nil)

type TokenCleanupWorker struct {
	repo   auth.Repository
	logger log.Logger
}

func NewTokenCleanupWorker(repo auth.Repository, logger log.Logger) *TokenCleanupWorker {
	return &TokenCleanupWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "auth_token_cleanup")),
	}
}

func (w *TokenCleanupWorker) JobType() core.JobType {
	return auth.JobAuthCleanupExpiredTokens
}

func (w *TokenCleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	count, err := w.repo.DeleteExpiredTokens(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	w.logger.Info("cleaned up expired auth tokens", log.Int("deleted_count", int(count)))
	data, err := json.Marshal(map[string]int64{"deleted": count})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// --- Session Cleanup Worker ---

var _ queue.JobProcessor = (*SessionCleanupWorker)(nil)

type SessionCleanupWorker struct {
	repo   auth.Repository
	logger log.Logger
}

func NewSessionCleanupWorker(repo auth.Repository, logger log.Logger) *SessionCleanupWorker {
	return &SessionCleanupWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "auth_session_cleanup")),
	}
}

func (w *SessionCleanupWorker) JobType() core.JobType {
	return auth.JobAuthCleanupExpiredSessions
}

func (w *SessionCleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	count, err := w.repo.DeleteExpiredSessions(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	w.logger.Info("cleaned up expired sessions", log.Int("deleted_count", int(count)))
	data, err := json.Marshal(map[string]int64{"deleted": count})
	if err != nil {
		return nil, err
	}
	return data, nil
}
