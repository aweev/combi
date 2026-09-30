package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync" // <--- Added
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"
)

// RecoveryService handles cleaning up stale/zombie executions.
type RecoveryService struct {
	repo          automation.Repository
	service       automation.Service
	logger        log.Logger
	interval      time.Duration
	shutdownCh    chan struct{}
	retentionDays int
	stopOnce      sync.Once // <--- Added for safe shutdown
}

func NewRecoveryService(repo automation.Repository, svc automation.Service, logger log.Logger) *RecoveryService {
	return &RecoveryService{
		repo:          repo,
		service:       svc,
		logger:        logger.With(log.String("component", "automation_recovery")),
		interval:      1 * time.Minute, // Run every minute
		shutdownCh:    make(chan struct{}),
		retentionDays: 30,
	}
}

// Update Start to run pruning less frequently (e.g., hourly)
func (r *RecoveryService) Start(ctx context.Context) {
	recoveryTicker := time.NewTicker(r.interval) // 1 minute
	pruneTicker := time.NewTicker(1 * time.Hour) // 1 hour

	r.logger.Info("automation maintenance service started")

	go func() {
		defer recoveryTicker.Stop()
		defer pruneTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.shutdownCh:
				return
			case <-recoveryTicker.C:
				r.RecoverStaleExecutions(ctx)
			case <-pruneTicker.C:
				r.PruneOldExecutions(ctx)
			}
		}
	}()
}

func (r *RecoveryService) Stop() {
	// [FIX] Ensure close happens only once to prevent panic
	r.stopOnce.Do(func() {
		close(r.shutdownCh)
	})
}

func (r *RecoveryService) PruneOldExecutions(ctx context.Context) {
	limit := time.Now().UTC().AddDate(0, 0, -r.retentionDays)
	batchSize := 1000 // Delete 1000 at a time to prevent locking

	r.logger.Info("starting execution pruning", log.Any("older_than", limit))

	totalDeleted := int64(0)
	for {
		deleted, err := r.repo.PruneExecutions(ctx, limit, batchSize)
		if err != nil {
			r.logger.Error("failed to prune executions", err)
			break
		}

		totalDeleted += deleted

		// If we deleted fewer than the batch size, we are done
		if deleted < int64(batchSize) {
			break
		}

		// Small sleep to let DB breathe between batches
		time.Sleep(100 * time.Millisecond)
	}

	if totalDeleted > 0 {
		r.logger.Info("pruned old executions", log.Int("count", int(totalDeleted)))
	}
}

func (r *RecoveryService) RecoverStaleExecutions(ctx context.Context) {
	// Define "Stale": No activity for 5 minutes.
	// This covers crashes (worker died) and stuck jobs.
	// However, long delays (e.g., 24h wait) will also appear here if we don't filter logic below.
	threshold := time.Now().UTC().Add(-5 * time.Minute)

	executions, err := r.repo.FindStaleExecutions(ctx, threshold, 50)
	if err != nil {
		r.logger.Error("failed to fetch stale executions", err)
		return
	}

	if len(executions) > 0 {
		r.logger.Info("found stale executions", log.Int("count", len(executions)))
	}

	for _, exec := range executions {
		r.processStaleExecution(ctx, exec)
	}
}

func (r *RecoveryService) processStaleExecution(ctx context.Context, exec automation.Execution) {
	logger := r.logger.With(log.String("execution_id", exec.ID.String()))

	// Check if the execution is purposefully sleeping (Delay action)
	if len(exec.ActionsResult) > 0 {
		lastResult := exec.ActionsResult[len(exec.ActionsResult)-1]

		// It was a pause/delay action
		if lastResult.Success && lastResult.Action.Type == "delay" {

			durationStr, _ := lastResult.Action.Config["duration"].(string)
			delayDuration, err := time.ParseDuration(durationStr)
			if err != nil {
				r.failExecution(ctx, exec, fmt.Sprintf("Recovery failed: invalid delay duration %v", err))
				return
			}

			// Calculate when it should wake up
			// We use exec.UpdatedAt because that's when the delay action was recorded (via UpdateExecution/AppendResult)
			wakeUpTime := exec.UpdatedAt.Add(delayDuration)
			now := time.Now().UTC()

			// Scenario 1: It is NOT stuck. It is just sleeping for a long time (e.g., 24 hours).
			// FindStaleExecutions picked it up because it hasn't updated in 5 mins,
			// but logically it is still valid.
			if now.Before(wakeUpTime) {
				// Do nothing. It is sleeping peacefully.
				// logger.Debug("execution is sleeping, skipping recovery")
				return
			}

			// Scenario 2: The "Dual Write" Zombie.
			// We are PAST the wakeUpTime, but the status is still RUNNING.
			// This means the queue message to resume it was lost or the worker crashed immediately after waking.
			logger.Warn("recovering stuck paused execution: re-dispatching continuation",
				log.String("wake_up_time", wakeUpTime.String()),
				log.String("now", now.String()),
			)

			if err := r.resumeExecution(ctx, exec); err != nil {
				logger.Error("failed to resume stuck execution", err)
			}
			return
		}
	}

	// Case B: Crash Recovery
	// The last action was NOT a delay, or there were no actions yet.
	// The worker stopped updating the DB > 5 mins ago. It likely crashed.
	logger.Warn("execution timed out (system crash or stall detected)")
	r.failExecution(ctx, exec, "Execution timed out (system crash or stall detected)")
}

func (r *RecoveryService) resumeExecution(ctx context.Context, exec automation.Execution) error {
	// 1. Fetch the Rule to know what actions are left
	rule, err := r.repo.GetRule(ctx, exec.RuleID)
	if err != nil {
		r.failExecution(ctx, exec, "Recovery failed: Rule not found")
		return err
	}

	totalActions := len(rule.Actions)
	executedActions := len(exec.ActionsResult)

	// If we somehow finished all actions but stuck in RUNNING, just complete it.
	if executedActions >= totalActions {
		r.completeExecution(ctx, exec)
		return nil
	}

	remaining := rule.Actions[executedActions:]

	// 2. Reconstruct Event Data
	var eventData map[string]any
	if err := json.Unmarshal(exec.TriggerData, &eventData); err != nil {
		r.failExecution(ctx, exec, "Recovery failed: Corrupt trigger data")
		return err
	}

	// 3. Dispatch Continuation
	// We call the service method. Ideally, this dispatches to the queue.
	// Since ContinueExecution in the service logic handles dispatching/executing,
	// calling it here is the correct way to "Kick" it back into gear.
	return r.service.ContinueExecution(ctx, exec.ID, eventData, remaining)
}

func (r *RecoveryService) failExecution(ctx context.Context, exec automation.Execution, reason string) {
	now := time.Now().UTC()
	exec.Status = automation.ExecutionStatusFailed
	exec.Error = &reason
	exec.CompletedAt = &now
	exec.UpdatedAt = now // Ensure we update heartbeat so we don't pick it up again
	exec.Duration = now.Sub(exec.StartedAt)

	if err := r.repo.UpdateExecution(ctx, &exec); err != nil {
		r.logger.Error("failed to mark execution as failed", err)
	}
}

func (r *RecoveryService) completeExecution(ctx context.Context, exec automation.Execution) {
	now := time.Now().UTC()
	exec.Status = automation.ExecutionStatusCompleted
	exec.CompletedAt = &now
	exec.UpdatedAt = now
	exec.Duration = now.Sub(exec.StartedAt)

	if err := r.repo.UpdateExecution(ctx, &exec); err != nil {
		r.logger.Error("failed to mark execution as completed", err)
	}
}
