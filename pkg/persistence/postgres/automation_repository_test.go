package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

// newTestRule creates a valid rule for testing purposes.
func newTestRule(t *testing.T, name, triggerType string, status automation.RuleStatus) *automation.Rule {
	t.Helper()
	return &automation.Rule{
		ID:          uuid.New(),
		Name:        name,
		Description: "Description for " + name,
		Status:      status,
		Priority:    10,
		Trigger: automation.Trigger{
			Type: triggerType,
		},
		// Initialize slices to empty, not nil, to avoid JSON issues with the DB driver.
		Conditions: []automation.Condition{},
		Logic:      automation.ConditionLogicAND,
		Actions:    []automation.Action{},
		Metadata:   make(map[string]any), // Use make() for an empty, non-nil map.
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
}
func newTestExecution(t *testing.T, ruleID uuid.UUID, status automation.ExecutionStatus, startedAt time.Time) *automation.Execution {
	t.Helper()
	return &automation.Execution{
		ID:      uuid.New(),
		RuleID:  ruleID,
		EventID: uuid.New(),
		Status:  status,
		// These fields MUST be initialized to valid JSON to prevent DB errors.
		TriggerData:      json.RawMessage(`{"key":"value"}`),
		ConditionsResult: []automation.ConditionResult{},
		ActionsResult:    []automation.ActionResult{},
		StartedAt:        startedAt.UTC().Truncate(time.Microsecond),
	}
}

// --- Test Suites ---

func TestAutomationRepository_RuleCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAutomationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Create and Get Rule", func(t *testing.T) {
		rule := newTestRule(t, "Test Rule", "user.created", automation.RuleStatusActive)
		rule.Conditions = []automation.Condition{{Field: "data.email", Operator: "contains", Value: "@example.com"}}

		err := repo.CreateRule(ctx, rule)
		require.NoError(t, err)

		got, err := repo.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, rule.ID, got.ID)
		assert.Equal(t, rule.Name, got.Name)
		assert.Equal(t, rule.Trigger.Type, got.Trigger.Type)
		assert.Equal(t, rule.Conditions[0].Field, got.Conditions[0].Field)
	})

	t.Run("Get non-existent Rule", func(t *testing.T) {
		_, err := repo.GetRule(ctx, uuid.New())
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})

	t.Run("Update Rule", func(t *testing.T) {
		rule := newTestRule(t, "Original Name", "user.updated", automation.RuleStatusDraft)
		require.NoError(t, repo.CreateRule(ctx, rule))

		originalUpdatedAt := rule.UpdatedAt
		rule.Name = "Updated Name"
		rule.Status = automation.RuleStatusActive
		time.Sleep(1 * time.Millisecond) // Ensure updated_at changes

		err := repo.UpdateRule(ctx, rule)
		require.NoError(t, err)

		got, err := repo.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", got.Name)
		assert.Equal(t, automation.RuleStatusActive, got.Status)
		assert.True(t, got.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("Update non-existent Rule", func(t *testing.T) {
		rule := newTestRule(t, "Non-existent", "user.deleted", automation.RuleStatusActive)
		err := repo.UpdateRule(ctx, rule)
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})

	t.Run("Delete Rule", func(t *testing.T) {
		rule := newTestRule(t, "To Be Deleted", "user.deleted", automation.RuleStatusInactive)
		require.NoError(t, repo.CreateRule(ctx, rule))

		err := repo.DeleteRule(ctx, rule.ID)
		require.NoError(t, err)

		_, err = repo.GetRule(ctx, rule.ID)
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})
}

func TestAutomationRepository_ListAndFilterRules(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAutomationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Seed data
	rule1 := newTestRule(t, "Active User Rule", "user.created", automation.RuleStatusActive)
	rule2 := newTestRule(t, "Draft User Rule", "user.created", automation.RuleStatusDraft)
	rule3 := newTestRule(t, "Active Donation Rule", "donation.succeeded", automation.RuleStatusActive)
	rule4 := newTestRule(t, "Inactive Donation Rule", "donation.succeeded", automation.RuleStatusInactive)
	require.NoError(t, repo.CreateRule(ctx, rule1))
	require.NoError(t, repo.CreateRule(ctx, rule2))
	require.NoError(t, repo.CreateRule(ctx, rule3))
	require.NoError(t, repo.CreateRule(ctx, rule4))

	t.Run("List with no filters", func(t *testing.T) {
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.EqualValues(t, 4, total)
		assert.Len(t, rules, 4)
	})

	t.Run("Filter by status", func(t *testing.T) {
		status := automation.RuleStatusActive
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{Status: &status, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, rules, 2)
	})

	t.Run("Filter by trigger type", func(t *testing.T) {
		triggerType := "donation.succeeded"
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{TriggerType: &triggerType, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, rules, 2)
	})

	t.Run("Filter by search term", func(t *testing.T) {
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{Search: "User", Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, rules, 2)
	})

	t.Run("Combined filters", func(t *testing.T) {
		status := automation.RuleStatusActive
		triggerType := "user.created"
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{Status: &status, TriggerType: &triggerType, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Len(t, rules, 1)
		assert.Equal(t, rule1.ID, rules[0].ID)
	})

	t.Run("Pagination", func(t *testing.T) {
		rules, total, err := repo.ListRules(ctx, automation.RuleFilter{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.EqualValues(t, 4, total)
		assert.Len(t, rules, 2)

		rules, total, err = repo.ListRules(ctx, automation.RuleFilter{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.EqualValues(t, 4, total)
		assert.Len(t, rules, 2)
	})

	t.Run("Get active rules by trigger", func(t *testing.T) {
		rules, err := repo.GetActiveRulesByTrigger(ctx, "user.created")
		require.NoError(t, err)
		assert.Len(t, rules, 1)
		assert.Equal(t, rule1.ID, rules[0].ID)
	})

	t.Run("Increment rule stats", func(t *testing.T) {
		// Create some executions for the rule
		exec1 := newTestExecution(t, rule1.ID, automation.ExecutionStatusCompleted, time.Now())
		exec2 := newTestExecution(t, rule1.ID, automation.ExecutionStatusFailed, time.Now())
		require.NoError(t, repo.CreateExecution(ctx, exec1))
		require.NoError(t, repo.CreateExecution(ctx, exec2))

		// Now call IncrementRuleStats
		err := repo.IncrementRuleStats(ctx, rule1.ID, true) // Success
		require.NoError(t, err)
		err = repo.IncrementRuleStats(ctx, rule1.ID, false) // Failure
		require.NoError(t, err)

		// Query the stats from the execution records (not the rule table)
		from := time.Now().Add(-1 * time.Hour)
		to := time.Now().Add(1 * time.Hour)
		stats, err := repo.GetRuleExecutionStats(ctx, rule1.ID, from, to)
		require.NoError(t, err)

		// We created 2 executions: 1 completed, 1 failed
		assert.EqualValues(t, 2, stats.TotalExecutions, "Should have 2 total executions from the created execution records")
		assert.EqualValues(t, 1, stats.SuccessCount, "Should have 1 successful execution")
		assert.EqualValues(t, 1, stats.FailureCount, "Should have 1 failed execution")
	})
}

func TestAutomationRepository_ExecutionCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAutomationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Prerequisite: Create a rule
	rule := newTestRule(t, "Execution Test Rule", "test.event", automation.RuleStatusActive)
	require.NoError(t, repo.CreateRule(ctx, rule))

	t.Run("Create and Get Execution", func(t *testing.T) {
		exec := newTestExecution(t, rule.ID, automation.ExecutionStatusPending, time.Now())

		err := repo.CreateExecution(ctx, exec)
		require.NoError(t, err)

		got, err := repo.GetExecution(ctx, exec.ID)
		require.NoError(t, err)
		assert.Equal(t, exec.ID, got.ID)
		assert.Equal(t, exec.RuleID, got.RuleID)
		assert.Equal(t, exec.Status, got.Status)
		assert.JSONEq(t, string(exec.TriggerData), string(got.TriggerData))
	})

	t.Run("Update and Append To Execution", func(t *testing.T) {
		exec := newTestExecution(t, rule.ID, automation.ExecutionStatusRunning, time.Now())
		require.NoError(t, repo.CreateExecution(ctx, exec))

		// Append an action result
		actionResult1 := automation.ActionResult{Success: true, Output: map[string]any{"message": "done"}}
		err := repo.AppendActionResult(ctx, exec.ID, actionResult1)
		require.NoError(t, err)

		got, err := repo.GetExecution(ctx, exec.ID)
		require.NoError(t, err)
		require.Len(t, got.ActionsResult, 1)
		assert.Equal(t, actionResult1.Output, got.ActionsResult[0].Output)

		// Update the final status
		completedTime := time.Now().UTC()
		errorMsg := "Something failed"
		exec.Status = automation.ExecutionStatusFailed
		exec.CompletedAt = &completedTime
		exec.Error = &errorMsg
		exec.Duration = time.Second

		err = repo.UpdateExecution(ctx, exec)
		require.NoError(t, err)

		got, err = repo.GetExecution(ctx, exec.ID)
		require.NoError(t, err)
		assert.Equal(t, automation.ExecutionStatusFailed, got.Status)
		assert.Equal(t, errorMsg, *got.Error)
		assert.WithinDuration(t, completedTime, *got.CompletedAt, time.Millisecond)
	})
	t.Run("Append Batch Action Results", func(t *testing.T) {
		exec := newTestExecution(t, rule.ID, automation.ExecutionStatusRunning, time.Now())
		require.NoError(t, repo.CreateExecution(ctx, exec))

		results := []automation.ActionResult{
			{Success: true, Output: map[string]any{"idx": 1}},
			{Success: true, Output: map[string]any{"idx": 2}},
		}

		err := repo.AppendActionResultBatch(ctx, exec.ID, results)
		require.NoError(t, err)

		got, err := repo.GetExecution(ctx, exec.ID)
		require.NoError(t, err)
		require.Len(t, got.ActionsResult, 2)

		// Verify order and content
		assert.Equal(t, float64(1), got.ActionsResult[0].Output["idx"])
		assert.Equal(t, float64(2), got.ActionsResult[1].Output["idx"])
	})
}

func TestAutomationRepository_Statistics(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAutomationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Create two rules
	rule1 := newTestRule(t, "Rule 1", "test.event", automation.RuleStatusActive)
	rule2 := newTestRule(t, "Rule 2", "test.event", automation.RuleStatusActive)
	require.NoError(t, repo.CreateRule(ctx, rule1))
	require.NoError(t, repo.CreateRule(ctx, rule2))

	baseTime := time.Now().UTC()

	// Create executions for the rules
	// For Rule 1: 1 success, 1 failure
	exec1 := newTestExecution(t, rule1.ID, automation.ExecutionStatusCompleted, baseTime.Add(-10*time.Minute))
	exec1.Duration = 200 * time.Millisecond
	exec2 := newTestExecution(t, rule1.ID, automation.ExecutionStatusFailed, baseTime.Add(-5*time.Minute))
	exec2.Duration = 300 * time.Millisecond

	// For Rule 2: 1 failure
	exec3 := newTestExecution(t, rule2.ID, automation.ExecutionStatusFailed, baseTime.Add(-2*time.Minute))
	exec3.Duration = 500 * time.Millisecond

	// This one is outside the time window for the test
	exec4 := newTestExecution(t, rule1.ID, automation.ExecutionStatusCompleted, baseTime.Add(-30*time.Minute))

	require.NoError(t, repo.CreateExecution(ctx, exec1))
	require.NoError(t, repo.CreateExecution(ctx, exec2))
	require.NoError(t, repo.CreateExecution(ctx, exec3))
	require.NoError(t, repo.CreateExecution(ctx, exec4))

	from := baseTime.Add(-15 * time.Minute)
	to := baseTime.Add(1 * time.Minute)

	t.Run("Get Rule Execution Stats", func(t *testing.T) {
		stats, err := repo.GetRuleExecutionStats(ctx, rule1.ID, from, to)
		require.NoError(t, err)

		assert.EqualValues(t, 2, stats.TotalExecutions)
		assert.EqualValues(t, 1, stats.SuccessCount)
		assert.EqualValues(t, 1, stats.FailureCount)
		assert.InDelta(t, 0.5, stats.SuccessRate, 0.01)
		// avg is (200+300)/2 = 250ms
		assert.InDelta(t, 250.0, stats.AvgExecutionTimeMs, 0.01)
	})

	t.Run("Get System Stats", func(t *testing.T) {
		stats, err := repo.GetSystemStats(ctx, from, to)
		require.NoError(t, err)

		// Assert overall system stats
		assert.EqualValues(t, 3, stats.TotalExecutions)
		assert.EqualValues(t, 1, stats.TotalSuccesses)
		assert.EqualValues(t, 2, stats.TotalFailures)
		// avg is (200+300+500)/3 = 333.33ms
		assert.InDelta(t, 333.33, stats.AvgExecutionTimeMs, 0.01)

		// --- UPDATED PART ---
		// Verify top failing rules without relying on a specific order.
		require.Len(t, stats.TopFailingRules, 2)

		// Convert the slice to a map for easy lookup
		statsByRuleID := make(map[uuid.UUID]automation.RuleStatistics)
		for _, s := range stats.TopFailingRules {
			statsByRuleID[s.RuleID] = s
		}

		// Check that both failing rules are in the map
		require.Contains(t, statsByRuleID, rule1.ID, "Rule 1 should be in the top failing list")
		require.Contains(t, statsByRuleID, rule2.ID, "Rule 2 should be in the top failing list")

		// Verify stats for Rule 1
		rule1Stats := statsByRuleID[rule1.ID]
		assert.EqualValues(t, 1, rule1Stats.FailureCount, "Rule 1 should have 1 failure")
		assert.EqualValues(t, 2, rule1Stats.TotalExecutions, "Rule 1 should have 2 total executions in the period")
		assert.EqualValues(t, 1, rule1Stats.SuccessCount, "Rule 1 should have 1 success")

		// Verify stats for Rule 2
		rule2Stats := statsByRuleID[rule2.ID]
		assert.EqualValues(t, 1, rule2Stats.FailureCount, "Rule 2 should have 1 failure")
		assert.EqualValues(t, 1, rule2Stats.TotalExecutions, "Rule 2 should have 1 total execution in the period")
		assert.EqualValues(t, 0, rule2Stats.SuccessCount, "Rule 2 should have 0 successes")
	})
}

func TestAutomationRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAutomationRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("transaction commits on success", func(t *testing.T) {
		rule := newTestRule(t, "TX Success", "tx.test", automation.RuleStatusActive)

		err := repo.WithTx(ctx, func(txRepo automation.Repository) error {
			return txRepo.CreateRule(ctx, rule)
		})
		require.NoError(t, err)

		// Verify the rule exists
		got, err := repo.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.Equal(t, rule.ID, got.ID)
	})

	t.Run("transaction rolls back on error", func(t *testing.T) {
		rule := newTestRule(t, "TX Rollback", "tx.test", automation.RuleStatusDraft)
		customErr := errors.New("intentional error")

		err := repo.WithTx(ctx, func(txRepo automation.Repository) error {
			if err := txRepo.CreateRule(ctx, rule); err != nil {
				return err
			}
			return customErr
		})
		require.ErrorIs(t, err, customErr)

		// Verify the rule does NOT exist
		_, err = repo.GetRule(ctx, rule.ID)
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})

	t.Run("transaction handles panic and rolls back", func(t *testing.T) {
		rule := newTestRule(t, "TX Panic", "tx.test", automation.RuleStatusDraft)

		assert.Panics(t, func() {
			_ = repo.WithTx(ctx, func(txRepo automation.Repository) error {
				_ = txRepo.CreateRule(ctx, rule)
				panic("something went very wrong")
			})
		})

		// Verify the rule does NOT exist
		_, err := repo.GetRule(ctx, rule.ID)
		assert.ErrorIs(t, err, automation.ErrRuleNotFound)
	})
}
