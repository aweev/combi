// internal/persistence/postgres/queue_repository_internal_test.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIsAppError(t *testing.T, err error, expected *apperror.AppError) {
	t.Helper()
	require.Error(t, err, "Expected an error but got none")

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr), "Error should be of type *apperror.AppError")

	assert.Equal(t, expected.Code, appErr.Code, "AppError code mismatch")
	assert.Equal(t, expected.Type, appErr.Type, "AppError type mismatch")
}

func newTestJob(status queue.JobStatus) *queue.Job {
	return &queue.Job{
		ID:          uuid.New(),
		Type:        core.JobType("test.job"),
		Queue:       "default",
		Priority:    10,
		Payload:     json.RawMessage(`{"message":"hello"}`),
		Status:      status,
		MaxAttempts: 3,
		Timeout:     5 * time.Minute,
		AvailableAt: time.Now().UTC().Truncate(time.Microsecond),
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestQueueRepository_JobCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Create and Get Job, with Outbox Event", func(t *testing.T) {
		job := newTestJob(queue.StatusPending)

		// Create the job
		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		// Get the job and verify its fields
		got, err := repo.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, got.ID)
		assert.Equal(t, job.Type, got.Type)
		assert.Equal(t, job.Queue, got.Queue)
		assert.Equal(t, job.Status, got.Status)
		assert.JSONEq(t, string(job.Payload), string(got.Payload))
		assert.WithinDuration(t, job.AvailableAt, got.AvailableAt, time.Second)

		// CRITICAL: Verify the outbox event was also created
		var outboxEvent struct {
			JobID     uuid.UUID `db:"job_id"`
			QueueName string    `db:"queue_name"`
			Status    string    `db:"status"`
		}
		err = db.GetContext(ctx, &outboxEvent, "SELECT job_id, queue_name, status FROM outbox_events WHERE job_id=$1", job.ID)
		require.NoError(t, err, "outbox event should have been created for a PENDING job")
		assert.Equal(t, job.ID, outboxEvent.JobID)
		assert.Equal(t, job.Queue, outboxEvent.QueueName)
		assert.Equal(t, "PENDING", outboxEvent.Status)
	})

	t.Run("Create a non-pending job does not create an outbox event", func(t *testing.T) {
		job := newTestJob(queue.StatusBlocked)

		err := repo.CreateJob(ctx, job)
		require.NoError(t, err)

		var count int
		err = db.GetContext(ctx, &count, "SELECT COUNT(*) FROM outbox_events WHERE job_id=$1", job.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "outbox event should NOT be created for a non-PENDING job")
	})

	t.Run("GetJob returns ErrJobNotFound for non-existent ID", func(t *testing.T) {
		_, err := repo.GetJob(ctx, uuid.New())
		assertIsAppError(t, err, queue.ErrJobNotFound)
	})

	t.Run("UpdateJob successfully modifies a job", func(t *testing.T) {
		job := newTestJob(queue.StatusPending)
		require.NoError(t, repo.CreateJob(ctx, job))

		// Modify the job
		job.Status = queue.StatusProcessing
		job.Attempts = 1
		newError := "first attempt failed"
		job.Error = &newError
		job.AvailableAt = time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond)

		err := repo.UpdateJob(ctx, job)
		require.NoError(t, err)

		// Get and verify changes
		got, err := repo.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, queue.StatusProcessing, got.Status)
		assert.Equal(t, 1, got.Attempts)
		require.NotNil(t, got.Error)
		assert.Equal(t, "first attempt failed", *got.Error)
		assert.WithinDuration(t, job.AvailableAt, got.AvailableAt, time.Second)
	})

	t.Run("UpdateJob returns ErrJobNotFound for non-existent ID", func(t *testing.T) {
		job := newTestJob(queue.StatusPending) // This job doesn't exist in the DB
		err := repo.UpdateJob(ctx, job)
		assertIsAppError(t, err, queue.ErrJobNotFound)
	})
}

func TestQueueRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	// Use the package's constructor
	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("transaction commits on success", func(t *testing.T) {
		job1 := newTestJob(queue.StatusPending)
		job2 := newTestJob(queue.StatusPending)

		err := repo.WithTx(ctx, func(txRepo queue.Repository) error {
			// Cast to the concrete type to access the unexported method
			concreteTxRepo := txRepo.(*QueueRepository)

			// FIX: Call the unexported 'createJob' which does NOT start a new transaction
			if err := concreteTxRepo.createJob(ctx, job1); err != nil {
				return err
			}
			if err := concreteTxRepo.createJob(ctx, job2); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// Verification remains the same
		_, err = repo.GetJob(ctx, job1.ID)
		assert.NoError(t, err, "job1 should exist")
		_, err = repo.GetJob(ctx, job2.ID)
		assert.NoError(t, err, "job2 should exist")
	})

	t.Run("transaction rolls back on error", func(t *testing.T) {
		jobToCreate := newTestJob(queue.StatusPending)
		jobID := jobToCreate.ID

		err := repo.WithTx(ctx, func(txRepo queue.Repository) error {
			concreteTxRepo := txRepo.(*QueueRepository)

			// FIX: Call the unexported 'createJob'
			if err := concreteTxRepo.createJob(ctx, jobToCreate); err != nil {
				return err
			}
			return errors.New("intentional rollback")
		})

		// Now the error assertion will be correct
		require.Error(t, err)
		assert.Equal(t, "intentional rollback", err.Error())

		_, err = repo.GetJob(ctx, jobID)
		assertIsAppError(t, err, queue.ErrJobNotFound)
		t.Log("Verified job was rolled back")
	})

	t.Run("transaction handles panic and rolls back", func(t *testing.T) {
		jobToCreate := newTestJob(queue.StatusPending)
		jobID := jobToCreate.ID

		assert.Panics(t, func() {
			_ = repo.WithTx(ctx, func(txRepo queue.Repository) error {
				concreteTxRepo := txRepo.(*QueueRepository)
				// FIX: Call the unexported 'createJob'
				_ = concreteTxRepo.createJob(ctx, jobToCreate)
				panic("something went terribly wrong")
			})
		})

		_, err := repo.GetJob(ctx, jobID)
		assertIsAppError(t, err, queue.ErrJobNotFound)
		t.Log("Verified job was rolled back after panic")
	})

	// This test is now correct because the outer repo can start a TX, and the inner one cannot.
	t.Run("nested transaction returns error", func(t *testing.T) {
		err := repo.WithTx(ctx, func(txRepo queue.Repository) error {
			return txRepo.WithTx(ctx, func(nestedTxRepo queue.Repository) error {
				return nil
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot begin transaction from within another transaction")
	})
}

// newTestFailedJob is a helper to create a valid failed job for testing.
func newTestFailedJob(queueName string, jobType core.JobType) *queue.FailedJob {
	return &queue.FailedJob{
		ID:        uuid.New(),
		JobID:     uuid.New(),
		Type:      jobType,
		Queue:     queueName,
		Payload:   json.RawMessage(`{"error": "true"}`),
		Exception: "some error occurred during processing",
		Attempts:  3,
		FailedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestQueueRepository_FailedJobLifecycle(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Create and Get FailedJob", func(t *testing.T) {
		failedJob := newTestFailedJob("critical", "email.send")
		require.NoError(t, repo.CreateFailedJob(ctx, failedJob))

		got, err := repo.GetFailedJob(ctx, failedJob.ID)
		require.NoError(t, err)
		assert.Equal(t, failedJob.ID, got.ID)
		assert.Equal(t, failedJob.JobID, got.JobID)
		assert.Equal(t, failedJob.Queue, got.Queue)
		assert.Equal(t, failedJob.Exception, got.Exception)
	})

	t.Run("GetFailedJob returns ErrFailedJobNotFound", func(t *testing.T) {
		_, err := repo.GetFailedJob(ctx, uuid.New())
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound)
	})

	t.Run("DeleteFailedJob", func(t *testing.T) {
		failedJob := newTestFailedJob("default", "report.generate")
		require.NoError(t, repo.CreateFailedJob(ctx, failedJob))

		err := repo.DeleteFailedJob(ctx, failedJob.ID)
		require.NoError(t, err)

		_, err = repo.GetFailedJob(ctx, failedJob.ID)
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound)
	})

	t.Run("BulkDeleteFailedJobs by queue", func(t *testing.T) {
		// Seed data
		fj1 := newTestFailedJob("queue-a", "type.1")
		fj2 := newTestFailedJob("queue-a", "type.2")
		fj3 := newTestFailedJob("queue-b", "type.1")
		require.NoError(t, repo.CreateFailedJob(ctx, fj1))
		require.NoError(t, repo.CreateFailedJob(ctx, fj2))
		require.NoError(t, repo.CreateFailedJob(ctx, fj3))

		// Delete by queue
		queueA := "queue-a"
		deletedCount, err := repo.BulkDeleteFailedJobs(ctx, queue.FailedJobFilters{Queue: &queueA})
		require.NoError(t, err)
		assert.Equal(t, int64(2), deletedCount)

		// Verify
		_, err = repo.GetFailedJob(ctx, fj1.ID)
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound)
		_, err = repo.GetFailedJob(ctx, fj2.ID)
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound)
		_, err = repo.GetFailedJob(ctx, fj3.ID)
		assert.NoError(t, err, "job in queue-b should not be deleted")
	})
}

func TestQueueRepository_BulkRetryFailedJobs(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Seed data for the retry tests
	retryQueue := "retry-queue"
	otherQueue := "other-queue"
	fjToRetry1 := newTestFailedJob(retryQueue, "email.send")
	fjToRetry2 := newTestFailedJob(retryQueue, "image.process")
	fjToKeep := newTestFailedJob(otherQueue, "email.send")
	require.NoError(t, repo.CreateFailedJob(ctx, fjToRetry1))
	require.NoError(t, repo.CreateFailedJob(ctx, fjToRetry2))
	require.NoError(t, repo.CreateFailedJob(ctx, fjToKeep))

	t.Run("retries matching jobs and deletes them transactionally", func(t *testing.T) {
		var newJobs []*queue.Job
		var err error

		// The operation MUST be transactional
		txErr := repo.WithTx(ctx, func(txRepo queue.Repository) error {
			newJobs, err = txRepo.BulkRetryFailedJobs(ctx, queue.FailedJobFilters{Queue: &retryQueue})
			return err
		})

		require.NoError(t, txErr)
		require.Len(t, newJobs, 2, "should have returned two new jobs")

		// --- Verification ---
		// 1. Verify failed jobs were deleted
		_, err = repo.GetFailedJob(ctx, fjToRetry1.ID)
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound, "retried job 1 should be deleted")
		_, err = repo.GetFailedJob(ctx, fjToRetry2.ID)
		assert.ErrorIs(t, err, queue.ErrFailedJobNotFound, "retried job 2 should be deleted")

		// 2. Verify non-matching failed job still exists
		_, err = repo.GetFailedJob(ctx, fjToKeep.ID)
		assert.NoError(t, err, "job in other queue should not be touched")

		// 3. Verify new jobs were created in the 'jobs' table
		for _, newJob := range newJobs {
			got, getErr := repo.GetJob(ctx, newJob.ID)
			require.NoError(t, getErr)
			assert.Equal(t, queue.StatusPending, got.Status)
			assert.Equal(t, retryQueue, got.Queue)

			// 4. Verify new outbox events were created for each new job
			var outboxCount int
			countErr := db.GetContext(ctx, &outboxCount, "SELECT COUNT(*) FROM outbox_events WHERE job_id=$1", newJob.ID)
			require.NoError(t, countErr)
			assert.Equal(t, 1, outboxCount, "a new outbox event should exist for the retried job")
		}
	})

	t.Run("returns no error and empty slice when no jobs match filter", func(t *testing.T) {
		nonExistentQueue := "non-existent-queue"
		newJobs, err := repo.BulkRetryFailedJobs(ctx, queue.FailedJobFilters{Queue: &nonExistentQueue})

		require.NoError(t, err)
		assert.Empty(t, newJobs)
	})
}

func TestQueueRepository_SearchJobs(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// --- Seed a diverse set of test data ---
	queueA := "queue-a"
	queueB := "queue-b"
	type1 := core.JobType("type.1")
	type2 := core.JobType("type.2")
	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	jobs := []*queue.Job{
		{ // Job 0
			ID: uuid.New(), Type: type1, Queue: queueA, Status: queue.StatusPending,
			CreatedAt: baseTime, AvailableAt: baseTime, Payload: json.RawMessage(`{}`), Tags: []string{"important"},
		},
		{ // Job 1
			ID: uuid.New(), Type: type2, Queue: queueA, Status: queue.StatusCompleted,
			CreatedAt: baseTime.Add(-1 * time.Hour), AvailableAt: baseTime, Payload: json.RawMessage(`{}`), Tags: []string{"important", "daily"},
		},
		{ // Job 2
			ID: uuid.New(), Type: type1, Queue: queueB, Status: queue.StatusProcessing,
			CreatedAt: baseTime.Add(-2 * time.Hour), AvailableAt: baseTime, Payload: json.RawMessage(`{}`), Tags: []string{"report"},
		},
		{ // Job 3
			ID: uuid.New(), Type: type1, Queue: queueA, Status: queue.StatusPending,
			CreatedAt: baseTime.Add(-3 * time.Hour), AvailableAt: baseTime, Payload: json.RawMessage(`{}`), Tags: []string{"report"},
		},
	}
	for _, job := range jobs {
		require.NoError(t, repo.CreateJob(ctx, job))
	}
	// --- End Seeding ---

	t.Run("search with no filters returns all", func(t *testing.T) {
		list, total, err := repo.SearchJobs(ctx, queue.JobSearchFilters{})
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, list, 4)
	})

	t.Run("filter by queue", func(t *testing.T) {
		filters := queue.JobSearchFilters{Queue: &queueA}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)
		for _, j := range list {
			assert.Equal(t, queueA, j.Queue)
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		filters := queue.JobSearchFilters{Type: &type1}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)
	})

	t.Run("filter by status", func(t *testing.T) {
		status := queue.StatusPending
		filters := queue.JobSearchFilters{Status: &status}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
	})

	t.Run("filter by a single tag", func(t *testing.T) {
		filters := queue.JobSearchFilters{Tags: []string{"important"}}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
	})

	t.Run("filter by multiple tags", func(t *testing.T) {
		filters := queue.JobSearchFilters{Tags: []string{"important", "daily"}}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, jobs[1].ID, list[0].ID)
	})

	t.Run("filter by date range", func(t *testing.T) {
		startDate := baseTime.Add(-2*time.Hour - 30*time.Minute)
		endDate := baseTime.Add(-1*time.Hour - 30*time.Minute)
		filters := queue.JobSearchFilters{StartDate: &startDate, EndDate: &endDate}

		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, jobs[2].ID, list[0].ID)
	})

	t.Run("pagination with limit and offset", func(t *testing.T) {
		filters := queue.JobSearchFilters{Limit: 2, Offset: 0}
		list1, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, list1, 2)

		filters.Offset = 2
		list2, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Len(t, list2, 2)

		// Ensure pages are distinct
		assert.NotEqual(t, list1[0].ID, list2[0].ID)
		assert.NotEqual(t, list1[1].ID, list2[1].ID)
	})

	t.Run("default sort order is created_at DESC", func(t *testing.T) {
		list, _, err := repo.SearchJobs(ctx, queue.JobSearchFilters{})
		require.NoError(t, err)
		assert.Len(t, list, 4)
		assert.Equal(t, jobs[0].ID, list[0].ID) // Newest
		assert.Equal(t, jobs[1].ID, list[1].ID)
		assert.Equal(t, jobs[2].ID, list[2].ID)
		assert.Equal(t, jobs[3].ID, list[3].ID) // Oldest
	})

	t.Run("combined filters", func(t *testing.T) {
		status := queue.StatusPending
		filters := queue.JobSearchFilters{Queue: &queueA, Status: &status}
		list, total, err := repo.SearchJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
	})
}

func TestQueueRepository_ListFailedJobs(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// --- Seed a diverse set of test data ---
	queueA := "failed-a"
	queueB := "failed-b"
	type1 := core.JobType("failed.type.1")
	type2 := core.JobType("failed.type.2")
	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	failedJobs := []*queue.FailedJob{
		{ID: uuid.New(), JobID: uuid.New(), Queue: queueA, Type: type1, Exception: "e", FailedAt: baseTime},
		{ID: uuid.New(), JobID: uuid.New(), Queue: queueB, Type: type1, Exception: "e", FailedAt: baseTime.Add(-1 * time.Hour)},
		{ID: uuid.New(), JobID: uuid.New(), Queue: queueA, Type: type2, Exception: "e", FailedAt: baseTime.Add(-2 * time.Hour)},
	}
	for _, fj := range failedJobs {
		require.NoError(t, repo.CreateFailedJob(ctx, fj))
	}
	// --- End Seeding ---

	t.Run("list with no filters returns all", func(t *testing.T) {
		list, total, err := repo.ListFailedJobs(ctx, queue.FailedJobFilters{})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 3)
	})

	t.Run("filter by queue", func(t *testing.T) {
		filters := queue.FailedJobFilters{Queue: &queueA}
		list, total, err := repo.ListFailedJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
	})

	t.Run("pagination with limit and offset", func(t *testing.T) {
		filters := queue.FailedJobFilters{Limit: 2, Offset: 0}
		list, total, err := repo.ListFailedJobs(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, list, 2)
	})

	t.Run("default sort order is failed_at DESC", func(t *testing.T) {
		list, _, err := repo.ListFailedJobs(ctx, queue.FailedJobFilters{})
		require.NoError(t, err)
		assert.Equal(t, failedJobs[0].ID, list[0].ID) // Newest
		assert.Equal(t, failedJobs[1].ID, list[1].ID)
		assert.Equal(t, failedJobs[2].ID, list[2].ID) // Oldest
	})
}

// newTestScheduledJob is a helper to create a valid scheduled job for testing.
func newTestScheduledJob(name string, active bool) *queue.ScheduledJob {
	nextRun := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	return &queue.ScheduledJob{
		ID:             uuid.New(),
		Name:           name,
		Type:           core.JobType("scheduled.report"),
		Payload:        json.RawMessage(`{"report_type":"weekly"}`),
		Queue:          "reports",
		CronExpression: "0 0 * * 0", // Every Sunday at midnight
		Timezone:       "UTC",
		IsActive:       active,
		NextRunAt:      &nextRun,
		CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestQueueRepository_ScheduledJobLifecycle(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Create and Get ScheduledJob", func(t *testing.T) {
		schedule := newTestScheduledJob("weekly-report-gen", true)

		err := repo.CreateSchedule(ctx, schedule)
		require.NoError(t, err)

		got, err := repo.GetSchedule(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, schedule.ID, got.ID)
		assert.Equal(t, "weekly-report-gen", got.Name)
		assert.Equal(t, "reports", got.Queue)
		assert.True(t, got.IsActive)
		require.NotNil(t, got.NextRunAt)
		assert.WithinDuration(t, *schedule.NextRunAt, *got.NextRunAt, time.Second)
	})

	t.Run("Create Schedule with duplicate name fails", func(t *testing.T) {
		schedule1 := newTestScheduledJob("unique-name", true)
		schedule2 := newTestScheduledJob("unique-name", true) // Same name
		schedule2.ID = uuid.New()                             // Different ID

		require.NoError(t, repo.CreateSchedule(ctx, schedule1))
		err := repo.CreateSchedule(ctx, schedule2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("GetSchedule returns ErrScheduleNotFound", func(t *testing.T) {
		_, err := repo.GetSchedule(ctx, uuid.New())
		assertIsAppError(t, err, queue.ErrScheduleNotFound)
	})

	t.Run("UpdateSchedule", func(t *testing.T) {
		schedule := newTestScheduledJob("updatable-schedule", true)
		require.NoError(t, repo.CreateSchedule(ctx, schedule))

		// Modify the schedule
		schedule.CronExpression = "0 12 * * *" // Change to noon daily
		schedule.IsActive = false
		lastError := "ran out of memory"
		schedule.LastError = &lastError

		err := repo.UpdateSchedule(ctx, schedule)
		require.NoError(t, err)

		got, err := repo.GetSchedule(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, "0 12 * * *", got.CronExpression)
		assert.False(t, got.IsActive)
		require.NotNil(t, got.LastError)
		assert.Equal(t, lastError, *got.LastError)
	})

	t.Run("DeleteSchedule", func(t *testing.T) {
		schedule := newTestScheduledJob("deletable-schedule", true)
		require.NoError(t, repo.CreateSchedule(ctx, schedule))

		err := repo.DeleteSchedule(ctx, schedule.ID)
		require.NoError(t, err)

		_, err = repo.GetSchedule(ctx, schedule.ID)
		assertIsAppError(t, err, queue.ErrScheduleNotFound)
	})

	t.Run("ListSchedules and GetActiveSchedules", func(t *testing.T) {
		// Clean up previous schedules to have a clean slate for counting
		_, err := db.ExecContext(ctx, "DELETE FROM scheduled_jobs")
		require.NoError(t, err)

		// Seed data
		require.NoError(t, repo.CreateSchedule(ctx, newTestScheduledJob("active-schedule-1", true)))
		require.NoError(t, repo.CreateSchedule(ctx, newTestScheduledJob("active-schedule-2", true)))
		require.NoError(t, repo.CreateSchedule(ctx, newTestScheduledJob("inactive-schedule", false)))

		// Test GetActiveSchedules
		active, err := repo.GetActiveSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, active, 2, "should only find active schedules")

		// Test ListSchedules (pagination)
		all, total, err := repo.ListSchedules(ctx, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total, "should list all schedules regardless of active status")
		assert.Len(t, all, 3)

		// Test pagination limit
		page, total, err := repo.ListSchedules(ctx, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, page, 2)
	})
}

func TestQueueRepository_JobDependencies(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CreateJobWithDependencies creates a blocked job and links", func(t *testing.T) {
		depJob := newTestJob(queue.StatusPending)
		require.NoError(t, repo.CreateJob(ctx, depJob))

		mainJob := newTestJob(queue.StatusBlocked) // Start with an intention, but the method forces it
		err := repo.CreateJobWithDependencies(ctx, mainJob, []uuid.UUID{depJob.ID})
		require.NoError(t, err)

		// 1. Verify the main job's status is BLOCKED
		got, err := repo.GetJob(ctx, mainJob.ID)
		require.NoError(t, err)
		assert.Equal(t, queue.StatusBlocked, got.Status)

		// 2. Verify the dependency link exists in the join table
		var count int
		err = db.GetContext(ctx, &count, "SELECT COUNT(*) FROM job_dependencies WHERE job_id=$1 AND depends_on_job_id=$2", mainJob.ID, depJob.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("GetJobDependencies and GetJobDependents", func(t *testing.T) {
		// Setup: B depends on A
		jobA := newTestJob(queue.StatusPending)
		jobB := newTestJob(queue.StatusBlocked)
		require.NoError(t, repo.CreateJob(ctx, jobA))
		require.NoError(t, repo.CreateJobWithDependencies(ctx, jobB, []uuid.UUID{jobA.ID}))

		// Test GetJobDependencies
		dependencies, err := repo.GetJobDependencies(ctx, jobB.ID)
		require.NoError(t, err)
		require.Len(t, dependencies, 1)
		assert.Equal(t, jobA.ID, dependencies[0].ID)

		// Test GetJobDependents
		dependents, err := repo.GetJobDependents(ctx, jobA.ID)
		require.NoError(t, err)
		require.Len(t, dependents, 1)
		assert.Equal(t, jobB.ID, dependents[0].ID)
	})

	t.Run("CheckAndUnblockDependents handles fan-in and fan-out", func(t *testing.T) {
		// --- Test Setup: A complex dependency graph ---
		// Job A (Completed)
		// Job B (Depends on A) -> Should be unblocked first
		// Job C (Depends on A) -> Should be unblocked first (fan-out)
		// Job D (Completed)
		// Job E (Depends on A and D) -> Should be unblocked last (fan-in)
		jobA := newTestJob(queue.StatusPending)
		jobD := newTestJob(queue.StatusPending)
		jobB := newTestJob(queue.StatusBlocked)
		jobC := newTestJob(queue.StatusBlocked)
		jobE := newTestJob(queue.StatusBlocked)
		require.NoError(t, repo.CreateJob(ctx, jobA))
		require.NoError(t, repo.CreateJob(ctx, jobD))
		require.NoError(t, repo.CreateJobWithDependencies(ctx, jobB, []uuid.UUID{jobA.ID}))
		require.NoError(t, repo.CreateJobWithDependencies(ctx, jobC, []uuid.UUID{jobA.ID}))
		require.NoError(t, repo.CreateJobWithDependencies(ctx, jobE, []uuid.UUID{jobA.ID, jobD.ID}))
		// --- End Setup ---

		// --- Step 1: Complete Job A and check for unblocks ---
		jobA.Status = queue.StatusCompleted
		require.NoError(t, repo.UpdateJob(ctx, jobA))

		var unblockedJobs []*queue.Job
		var err error
		txErr := repo.WithTx(ctx, func(txRepo queue.Repository) error {
			unblockedJobs, err = txRepo.CheckAndUnblockDependents(ctx, jobA.ID)
			return err
		})
		require.NoError(t, txErr)

		// Verification after A completes
		require.Len(t, unblockedJobs, 2, "Jobs B and C should have been unblocked")
		// Make a map for easy lookup
		unblockedIDs := make(map[uuid.UUID]bool)
		for _, j := range unblockedJobs {
			unblockedIDs[j.ID] = true
		}
		assert.True(t, unblockedIDs[jobB.ID], "Job B should be in the unblocked list")
		assert.True(t, unblockedIDs[jobC.ID], "Job C should be in the unblocked list")

		gotB, _ := repo.GetJob(ctx, jobB.ID)
		assert.Equal(t, queue.StatusPending, gotB.Status, "Job B should now be PENDING")
		gotC, _ := repo.GetJob(ctx, jobC.ID)
		assert.Equal(t, queue.StatusPending, gotC.Status, "Job C should now be PENDING")

		gotE, _ := repo.GetJob(ctx, jobE.ID)
		assert.Equal(t, queue.StatusBlocked, gotE.Status, "Job E should still be BLOCKED because Job D is not complete")

		// --- Step 2: Complete Job D and check for unblocks ---
		jobD.Status = queue.StatusCompleted
		require.NoError(t, repo.UpdateJob(ctx, jobD))

		txErr = repo.WithTx(ctx, func(txRepo queue.Repository) error {
			unblockedJobs, err = txRepo.CheckAndUnblockDependents(ctx, jobD.ID)
			return err
		})
		require.NoError(t, txErr)

		// Verification after D completes
		require.Len(t, unblockedJobs, 1, "Only Job E should have been unblocked")
		assert.Equal(t, jobE.ID, unblockedJobs[0].ID)

		gotE, _ = repo.GetJob(ctx, jobE.ID)
		assert.Equal(t, queue.StatusPending, gotE.Status, "Job E should now be PENDING")

		// Verify outbox event was created for the finally unblocked job E
		var outboxCount int
		err = db.GetContext(ctx, &outboxCount, "SELECT COUNT(*) FROM outbox_events WHERE job_id=$1", jobE.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, outboxCount, "an outbox event should be created when a job is unblocked")
	})
}

func TestQueueRepository_MonitoringAndStats(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("GetAggregatedStats and GetQueueStats", func(t *testing.T) {
		// --- Seed Data ---
		// Queue A: 1 pending, 1 processing
		jobA1 := newTestJob(queue.StatusPending)
		jobA1.Queue = "queue-a"
		jobA2 := newTestJob(queue.StatusProcessing)
		jobA2.Queue = "queue-a"
		// Queue B: 1 pending, 1 completed
		jobB1 := newTestJob(queue.StatusPending)
		jobB1.Queue = "queue-b"
		jobB2 := newTestJob(queue.StatusCompleted)
		jobB2.Queue = "queue-b"
		// Failed Jobs: 1 in queue-a, 2 in queue-b
		failedA1 := newTestFailedJob("queue-a", "type.1")
		failedB1 := newTestFailedJob("queue-b", "type.2")
		failedB2 := newTestFailedJob("queue-b", "type.3")
		// Schedules: 1 active, 1 inactive
		schedule1 := newTestScheduledJob("active", true)
		schedule2 := newTestScheduledJob("inactive", false)

		require.NoError(t, repo.CreateJob(ctx, jobA1))
		require.NoError(t, repo.CreateJob(ctx, jobA2))
		require.NoError(t, repo.CreateJob(ctx, jobB1))
		require.NoError(t, repo.CreateJob(ctx, jobB2))
		require.NoError(t, repo.CreateFailedJob(ctx, failedA1))
		require.NoError(t, repo.CreateFailedJob(ctx, failedB1))
		require.NoError(t, repo.CreateFailedJob(ctx, failedB2))
		require.NoError(t, repo.CreateSchedule(ctx, schedule1))
		require.NoError(t, repo.CreateSchedule(ctx, schedule2))
		// --- End Seed Data ---

		// Test GetAggregatedStats
		aggStats, err := repo.GetAggregatedStats(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), aggStats.TotalPending)    // jobA1, jobB1
		assert.Equal(t, int64(1), aggStats.TotalProcessing) // jobA2
		assert.Equal(t, int64(3), aggStats.TotalFailed)
		assert.Equal(t, int64(1), int64(aggStats.TotalSchedules))
		// Test GetQueueStats
		queueStats, err := repo.GetQueueStats(ctx)
		require.NoError(t, err)
		require.Len(t, queueStats, 2)

		statsMap := make(map[string]queue.QueueStats)
		for _, s := range queueStats {
			statsMap[s.Name] = s
		}
		assert.Equal(t, int64(1), statsMap["queue-a"].Pending)
		assert.Equal(t, int64(1), statsMap["queue-a"].Processing)
		assert.Equal(t, int64(1), statsMap["queue-a"].Failed)
		assert.Equal(t, int64(1), statsMap["queue-b"].Pending)
		assert.Equal(t, int64(0), statsMap["queue-b"].Processing)
		assert.Equal(t, int64(2), statsMap["queue-b"].Failed)
	})

	t.Run("RecordHeartbeat and GetActiveWorkers", func(t *testing.T) {
		activeWorker := &queue.WorkerMetrics{
			ID: "worker-1", Host: "host-a", Queues: []string{"q1"}, Status: "active",
			JobsProcessed: 10, LastHeartbeat: time.Now().UTC(), StartedAt: time.Now().UTC(),
		}
		staleWorker := &queue.WorkerMetrics{
			ID: "worker-2", Host: "host-b", Queues: []string{"q2"}, Status: "active",
			JobsProcessed: 20, LastHeartbeat: time.Now().UTC().Add(-10 * time.Minute), StartedAt: time.Now().UTC(),
		}
		require.NoError(t, repo.RecordHeartbeat(ctx, activeWorker))
		require.NoError(t, repo.RecordHeartbeat(ctx, staleWorker))

		// Test upsert functionality
		activeWorker.JobsProcessed = 15
		activeWorker.LastHeartbeat = time.Now().UTC()
		require.NoError(t, repo.RecordHeartbeat(ctx, activeWorker))

		var updatedJobsProcessed int64
		err := db.GetContext(ctx, &updatedJobsProcessed, "SELECT jobs_processed FROM worker_metrics WHERE id='worker-1'")
		require.NoError(t, err)
		assert.Equal(t, int64(15), updatedJobsProcessed)

		// Test GetActiveWorkers
		activeWorkers, err := repo.GetActiveWorkers(ctx, 5*time.Minute)
		require.NoError(t, err)
		require.Len(t, activeWorkers, 1)
		assert.Equal(t, "worker-1", activeWorkers[0].ID)
	})
}

func TestQueueRepository_Maintenance(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Seed data
	oldCompletedJob := newTestJob(queue.StatusCompleted)
	oldCompletedJob.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour)
	newCompletedJob := newTestJob(queue.StatusCompleted)
	require.NoError(t, repo.CreateJob(ctx, oldCompletedJob))
	require.NoError(t, repo.CreateJob(ctx, newCompletedJob))

	oldFailedJob := newTestFailedJob("default", "type.1")
	oldFailedJob.FailedAt = time.Now().UTC().Add(-48 * time.Hour)
	newFailedJob := newTestFailedJob("default", "type.1")
	require.NoError(t, repo.CreateFailedJob(ctx, oldFailedJob))
	require.NoError(t, repo.CreateFailedJob(ctx, newFailedJob))

	t.Run("DeleteJobsOlderThan", func(t *testing.T) {
		deletedCount, err := repo.DeleteJobsOlderThan(ctx, queue.StatusCompleted, time.Now().UTC().Add(-24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), deletedCount)

		_, err = repo.GetJob(ctx, oldCompletedJob.ID)
		assertIsAppError(t, err, queue.ErrJobNotFound)
		_, err = repo.GetJob(ctx, newCompletedJob.ID)
		assert.NoError(t, err)
	})

	t.Run("DeleteFailedJobsOlderThan", func(t *testing.T) {
		deletedCount, err := repo.DeleteFailedJobsOlderThan(ctx, time.Now().UTC().Add(-24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), deletedCount)

		_, err = repo.GetFailedJob(ctx, oldFailedJob.ID)
		assertIsAppError(t, err, queue.ErrFailedJobNotFound)

		_, err = repo.GetFailedJob(ctx, newFailedJob.ID)
		assert.NoError(t, err)
	})
}

func TestQueueRepository_Outbox(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := NewQueueRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Seed outbox events directly for isolated testing
	event1 := queue.OutboxEvent{ID: uuid.New(), JobID: uuid.New(), QueueName: "q1", Status: queue.OutboxStatusPending, CreatedAt: time.Now().UTC().Add(-10 * time.Minute)}
	event2 := queue.OutboxEvent{ID: uuid.New(), JobID: uuid.New(), QueueName: "q2", Status: queue.OutboxStatusPending, CreatedAt: time.Now().UTC().Add(-5 * time.Minute)}
	event3 := queue.OutboxEvent{ID: uuid.New(), JobID: uuid.New(), QueueName: "q1", Status: queue.OutboxStatusProcessed, CreatedAt: time.Now().UTC().Add(-15 * time.Minute)}
	_, err := db.NamedExecContext(ctx, `INSERT INTO outbox_events (id, job_id, queue_name, status, created_at, priority) VALUES (:id, :job_id, :queue_name, :status, :created_at, 0)`, &event1)
	require.NoError(t, err)
	_, err = db.NamedExecContext(ctx, `INSERT INTO outbox_events (id, job_id, queue_name, status, created_at, priority) VALUES (:id, :job_id, :queue_name, :status, :created_at, 0)`, &event2)
	require.NoError(t, err)
	_, err = db.NamedExecContext(ctx, `INSERT INTO outbox_events (id, job_id, queue_name, status, created_at, priority) VALUES (:id, :job_id, :queue_name, :status, :created_at, 0)`, &event3)
	require.NoError(t, err)

	t.Run("FetchAndLockOutboxEvents", func(t *testing.T) {
		events, err := repo.FetchAndLockOutboxEvents(ctx, 5)
		require.NoError(t, err)
		// Should fetch the two pending events, ordered by creation time
		require.Len(t, events, 2)
		assert.Equal(t, event1.ID, events[0].ID, "should fetch the oldest event first")
		assert.Equal(t, event2.ID, events[1].ID)
	})

	t.Run("UpdateOutboxEvent", func(t *testing.T) {
		eventToUpdate := event2
		now := time.Now().UTC()
		eventToUpdate.Status = queue.OutboxStatusProcessed
		eventToUpdate.ProcessedAt = &now
		eventToUpdate.Attempts = 1

		err := repo.UpdateOutboxEvent(ctx, &eventToUpdate)
		require.NoError(t, err)

		var got queue.OutboxEvent
		err = db.GetContext(ctx, &got, "SELECT * FROM outbox_events WHERE id=$1", eventToUpdate.ID)
		require.NoError(t, err)
		assert.Equal(t, queue.OutboxStatusProcessed, got.Status)
		require.NotNil(t, got.ProcessedAt)
		assert.WithinDuration(t, now, *got.ProcessedAt, time.Second)
		assert.Equal(t, 1, got.Attempts)
	})
}
