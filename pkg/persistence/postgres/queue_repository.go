package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ queue.Repository = (*QueueRepository)(nil)

type QueueRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	dbOrTx squirrel.BaseRunner
	logger log.Logger
}

func NewQueueRepository(db *sqlx.DB, logger log.Logger) *QueueRepository {
	return &QueueRepository{
		db:     db,
		runner: db,
		dbOrTx: db,
		logger: logger.With(log.String("component", "queue_repository")),
	}
}

func (r *QueueRepository) WithTx(ctx context.Context, fn func(txRepo queue.Repository) error) (err error) {
	// Strict Check: If r.db is nil, we are already inside a transaction wrapper.
	if r.db == nil {
		return errors.New("cannot begin transaction from within another transaction")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			if cErr := tx.Commit(); cErr != nil {
				r.logger.Error("transaction commit failed", cErr)
				err = cErr
			}
		}
	}()

	txRepo := &QueueRepository{
		runner: tx,
		dbOrTx: tx,
		logger: r.logger,
		db:     nil, // Signal active transaction
	}

	return fn(txRepo)
}

func (r *QueueRepository) builder() squirrel.StatementBuilderType {
	return squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(r.dbOrTx)
}

func (r *QueueRepository) CreateJob(ctx context.Context, job *queue.Job) error {
	if r.db == nil {
		return r.createJob(ctx, job)
	}
	return r.WithTx(ctx, func(txRepo queue.Repository) error {
		return txRepo.(*QueueRepository).createJob(ctx, job)
	})
}

func (r *QueueRepository) createJob(ctx context.Context, job *queue.Job) error {
	if err := r.createJobInTx(ctx, job); err != nil {
		return err
	}
	if job.Status == queue.StatusPending {
		outboxEvent := &queue.OutboxEvent{
			ID:        uuid.New(),
			JobID:     job.ID,
			QueueName: job.Queue,
			Priority:  job.Priority,
			Status:    queue.OutboxStatusPending,
			CreatedAt: time.Now().UTC(),
		}
		_, err := r.builder().Insert("outbox_events").SetMap(outboxEventToMap(outboxEvent)).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}
	}
	return nil
}

func (r *QueueRepository) CreateJobWithDependencies(ctx context.Context, job *queue.Job, dependencyIDs []uuid.UUID) error {
	if r.db == nil {
		return r.createJobWithDependenciesInternal(ctx, job, dependencyIDs)
	}
	return r.WithTx(ctx, func(txRepo queue.Repository) error {
		return txRepo.(*QueueRepository).createJobWithDependenciesInternal(ctx, job, dependencyIDs)
	})
}

func (r *QueueRepository) createJobWithDependenciesInternal(ctx context.Context, job *queue.Job, dependencyIDs []uuid.UUID) error {
	job.Status = queue.StatusBlocked
	if err := r.createJobInTx(ctx, job); err != nil {
		return err
	}
	if len(dependencyIDs) > 0 {
		insertBuilder := r.builder().Insert("job_dependencies").Columns("job_id", "depends_on_job_id")
		for _, depID := range dependencyIDs {
			insertBuilder = insertBuilder.Values(job.ID, depID)
		}
		_, err := insertBuilder.ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to insert job dependencies for job %s: %w", job.ID, err)
		}
	}
	return nil
}

func (r *QueueRepository) createJobInTx(ctx context.Context, job *queue.Job) error {
	payloadMap, err := jobToMap(job)
	if err != nil {
		return err
	}
	_, err = r.builder().Insert("jobs").SetMap(payloadMap).ExecContext(ctx)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return apperror.New(http.StatusBadRequest, "FOREIGN_KEY_VIOLATION", "foreign key violation", err)
		}
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}

func (r *QueueRepository) GetJob(ctx context.Context, id uuid.UUID) (*queue.Job, error) {
	query, args, _ := r.builder().Select("*").From("jobs").Where(squirrel.Eq{"id": id}).ToSql()
	var dbJob dbJob
	if err := sqlx.GetContext(ctx, r.runner, &dbJob, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, queue.ErrJobNotFound.WithUnderlying(err)
		}
		return nil, fmt.Errorf("failed to get job %s: %w", id, err)
	}
	return dbJob.toDomain()
}

func (r *QueueRepository) UpdateJob(ctx context.Context, job *queue.Job) error {
	job.UpdatedAt = time.Now().UTC()

	updateMap := map[string]interface{}{
		"status":       job.Status,
		"attempts":     job.Attempts,
		"available_at": job.AvailableAt,
		"error":        job.Error,
		"updated_at":   job.UpdatedAt,
	}
	if job.Result != nil {
		updateMap["result"] = string(job.Result)
	} else {
		updateMap["result"] = nil
	}

	res, err := r.builder().Update("jobs").
		SetMap(updateMap).
		Where(squirrel.Eq{"id": job.ID}).
		ExecContext(ctx)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "invalid_text_representation" {
			return apperror.New(http.StatusBadRequest, "INVALID_JSON_PAYLOAD", "failed to update job due to JSON syntax error in 'result' field", err)
		}
		return fmt.Errorf("failed to update job %s: %w", job.ID, err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return queue.ErrJobNotFound
	}
	return nil
}

// --- Failed Job Methods ---

func (r *QueueRepository) CreateFailedJob(ctx context.Context, failedJob *queue.FailedJob) error {
	_, err := r.builder().Insert("failed_jobs").SetMap(failedJobToMap(failedJob)).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert failed job: %w", err)
	}
	return nil
}

func (r *QueueRepository) GetFailedJob(ctx context.Context, id uuid.UUID) (*queue.FailedJob, error) {
	query, args, _ := r.builder().Select("*").From("failed_jobs").Where(squirrel.Eq{"id": id}).ToSql()
	var failedJob queue.FailedJob
	if err := sqlx.GetContext(ctx, r.runner, &failedJob, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, queue.ErrFailedJobNotFound
		}
		return nil, fmt.Errorf("failed to get failed job %s: %w", id, err)
	}
	return &failedJob, nil
}

func (r *QueueRepository) DeleteFailedJob(ctx context.Context, id uuid.UUID) error {
	res, err := r.builder().Delete("failed_jobs").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete failed job %s: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return queue.ErrFailedJobNotFound
	}
	return nil
}

func (r *QueueRepository) BulkDeleteFailedJobs(ctx context.Context, filters queue.FailedJobFilters) (int64, error) {
	builder := r.builder().Delete("failed_jobs")
	builder = buildFailedJobFilterQuery(builder, filters)

	res, err := builder.ExecContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to execute bulk delete: %w", err)
	}
	return res.RowsAffected()
}

func (r *QueueRepository) BulkRetryFailedJobs(ctx context.Context, filters queue.FailedJobFilters) ([]*queue.Job, error) {
	sq := r.builder()
	var newJobs []*queue.Job

	selectBuilder := sq.Select("*").From("failed_jobs")
	selectBuilder = buildFailedJobFilterQuery(selectBuilder, filters)
	selectQuery, args, err := selectBuilder.Suffix("FOR UPDATE SKIP LOCKED").ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build select query for bulk retry: %w", err)
	}

	var failedJobs []queue.FailedJob
	if err := sqlx.SelectContext(ctx, r.runner, &failedJobs, selectQuery, args...); err != nil {
		return nil, fmt.Errorf("failed to fetch jobs for bulk retry: %w", err)
	}
	if len(failedJobs) == 0 {
		return nil, nil
	}

	failedJobIDs := make([]uuid.UUID, len(failedJobs))
	jobInsertBuilder := sq.Insert("jobs").Columns(
		"id", "type", "queue", "payload", "status", "actor_id", "actor_type",
	)
	outboxInsertBuilder := sq.Insert("outbox_events").Columns(
		"id", "job_id", "queue_name", "priority", "status",
	)

	for i, fj := range failedJobs {
		failedJobIDs[i] = fj.ID
		newJob := &queue.Job{
			ID:        uuid.New(),
			Type:      fj.Type,
			Queue:     fj.Queue,
			Payload:   fj.Payload,
			Status:    queue.StatusPending,
			ActorType: "system",
			ActorID:   &system.SystemActorID,
		}
		newJobs = append(newJobs, newJob)
		jobInsertBuilder = jobInsertBuilder.Values(
			newJob.ID, newJob.Type, newJob.Queue, string(newJob.Payload), newJob.Status,
			newJob.ActorID, newJob.ActorType,
		)
		outboxInsertBuilder = outboxInsertBuilder.Values(
			uuid.New(), newJob.ID, newJob.Queue, 5, queue.OutboxStatusPending,
		)
	}

	if _, err := jobInsertBuilder.ExecContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to bulk insert new jobs for retry: %w", err)
	}
	if _, err := outboxInsertBuilder.ExecContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to bulk insert new outbox events for retry: %w", err)
	}

	_, err = sq.Delete("failed_jobs").Where(squirrel.Eq{"id": failedJobIDs}).ExecContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to delete retried jobs: %w", err)
	}

	return newJobs, nil
}

// --- Search & List Methods ---

func (r *QueueRepository) SearchJobs(ctx context.Context, filters queue.JobSearchFilters) ([]*queue.Job, int, error) {
	sq := r.builder()
	countQuery := sq.Select("COUNT(*)").From("jobs")
	countQuery = buildJobFilterQuery(countQuery, filters)

	var total int
	countSql, args, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count query for search jobs: %w", err)
	}
	if err := sqlx.GetContext(ctx, r.runner, &total, countSql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to execute count query for search jobs: %w", err)
	}

	if total == 0 {
		return []*queue.Job{}, 0, nil
	}

	baseQuery := sq.Select("*").From("jobs")
	baseQuery = buildJobFilterQuery(baseQuery, filters)
	baseQuery = baseQuery.OrderBy("created_at DESC").Offset(uint64(filters.Offset))
	if filters.Limit > 0 {
		baseQuery = baseQuery.Limit(uint64(filters.Limit))
	}

	var dbJobs []dbJob
	baseSql, args, err := baseQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build search query for jobs: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &dbJobs, baseSql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to execute search query for jobs: %w", err)
	}

	jobs := make([]*queue.Job, len(dbJobs))
	for i, j := range dbJobs {
		job, err := j.toDomain()
		if err != nil {
			r.logger.Error("failed to convert db job to domain", err, log.String("job_id", j.ID.String()))
			continue
		}
		jobs[i] = job
	}
	return jobs, total, nil
}

func (r *QueueRepository) ListFailedJobs(ctx context.Context, filters queue.FailedJobFilters) ([]*queue.FailedJob, int, error) {
	countQuery := r.builder().Select("COUNT(*)").From("failed_jobs")
	countQuery = buildFailedJobFilterQuery(countQuery, filters)

	var total int
	countSql, args, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count query for failed jobs: %w", err)
	}
	if err := sqlx.GetContext(ctx, r.runner, &total, countSql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to execute count query for failed jobs: %w", err)
	}
	baseQuery := r.builder().Select("*").From("failed_jobs")
	baseQuery = buildFailedJobFilterQuery(baseQuery, filters)
	baseQuery = baseQuery.OrderBy("failed_at DESC").Offset(uint64(filters.Offset))
	if filters.Limit > 0 {
		baseQuery = baseQuery.Limit(uint64(filters.Limit))
	}

	var failedJobs []*queue.FailedJob
	baseSql, args, err := baseQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build list query for failed jobs: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &failedJobs, baseSql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to execute list query for failed jobs: %w", err)
	}
	return failedJobs, total, nil
}

// --- Schedule Methods ---
func (r *QueueRepository) CreateSchedule(ctx context.Context, schedule *queue.ScheduledJob) error {
	_, err := r.builder().Insert("scheduled_jobs").SetMap(scheduledJobToMap(schedule)).ExecContext(ctx)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			msg := fmt.Sprintf("a schedule with name '%s' already exists", schedule.Name)
			return apperror.New(http.StatusConflict, "SCHEDULE_ALREADY_EXISTS", msg, err)
		}
		return fmt.Errorf("failed to create schedule: %w", err)
	}
	return nil
}

func (r *QueueRepository) UpdateSchedule(ctx context.Context, schedule *queue.ScheduledJob) error {
	schedule.UpdatedAt = time.Now().UTC()
	payload := scheduledJobToMap(schedule)
	delete(payload, "id")

	res, err := r.builder().Update("scheduled_jobs").SetMap(payload).Where(squirrel.Eq{"id": schedule.ID}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to update schedule %s: %w", schedule.ID, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return queue.ErrScheduleNotFound
	}
	return nil
}

func (r *QueueRepository) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	res, err := r.builder().Delete("scheduled_jobs").Where(squirrel.Eq{"id": id}).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete schedule %s: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return queue.ErrScheduleNotFound
	}
	return nil
}

func (r *QueueRepository) GetSchedule(ctx context.Context, id uuid.UUID) (*queue.ScheduledJob, error) {
	var dbSchedule dbScheduledJob
	query, args, err := r.builder().Select("*").
		From("scheduled_jobs").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get schedule query: %w", err)
	}

	if err := sqlx.GetContext(ctx, r.runner, &dbSchedule, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, queue.ErrScheduleNotFound.WithUnderlying(err)
		}
		return nil, fmt.Errorf("failed to get schedule %s: %w", id, err)
	}
	return dbSchedule.toDomain(), nil
}

func (r *QueueRepository) ListSchedules(ctx context.Context, limit, offset int) ([]*queue.ScheduledJob, int, error) {
	var total int
	countSql, args, err := r.builder().Select("COUNT(*)").From("scheduled_jobs").ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build count schedules query: %w", err)
	}
	if err := sqlx.GetContext(ctx, r.runner, &total, countSql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to count schedules: %w", err)
	}

	if total == 0 {
		return []*queue.ScheduledJob{}, 0, nil
	}

	queryBuilder := r.builder().Select("*").From("scheduled_jobs").
		OrderBy("name ASC").
		Offset(uint64(offset))
	if limit > 0 {
		queryBuilder = queryBuilder.Limit(uint64(limit))
	}

	var dbSchedules []dbScheduledJob
	querySql, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build list schedules query: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &dbSchedules, querySql, args...); err != nil {
		return nil, 0, fmt.Errorf("failed to list schedules: %w", err)
	}

	schedules := make([]*queue.ScheduledJob, len(dbSchedules))
	for i, dbSch := range dbSchedules {
		schedules[i] = dbSch.toDomain()
	}
	return schedules, total, nil
}

func (r *QueueRepository) GetActiveSchedules(ctx context.Context) ([]*queue.ScheduledJob, error) {
	var dbSchedules []dbScheduledJob

	query, args, err := r.builder().Select("*").
		From("scheduled_jobs").
		Where(squirrel.Eq{"is_active": true}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get active schedules query: %w", err)
	}

	if err := sqlx.SelectContext(ctx, r.runner, &dbSchedules, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*queue.ScheduledJob{}, nil
		}
		return nil, fmt.Errorf("failed to get active schedules: %w", err)
	}

	schedules := make([]*queue.ScheduledJob, len(dbSchedules))
	for i, dbSch := range dbSchedules {
		schedules[i] = dbSch.toDomain()
	}
	return schedules, nil
}

// --- Dependency Methods ---

func (r *QueueRepository) CheckAndUnblockDependents(ctx context.Context, completedJobID uuid.UUID) ([]*queue.Job, error) {
	var unblockedDbJobs []dbJob

	query := `
		WITH dependent_jobs AS (
			SELECT job_id FROM job_dependencies WHERE depends_on_job_id = $1
		),
		ready_to_unblock AS (
			SELECT dj.job_id
			FROM dependent_jobs dj
			WHERE NOT EXISTS (
				SELECT 1
				FROM job_dependencies dep
				JOIN jobs j ON dep.depends_on_job_id = j.id
				WHERE dep.job_id = dj.job_id
				  AND j.id != $1
				  AND j.status != 'COMPLETED'
			)
		)
		UPDATE jobs
		SET status = $2, updated_at = NOW() AT TIME ZONE 'utc'
		WHERE id IN (SELECT job_id FROM ready_to_unblock) AND status = 'BLOCKED'
		RETURNING *;
	`
	err := sqlx.SelectContext(ctx, r.runner, &unblockedDbJobs, query, completedJobID, queue.StatusPending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check and unblock dependents: %w", err)
	}

	if len(unblockedDbJobs) > 0 {
		outboxInsertBuilder := r.builder().Insert("outbox_events").Columns("id", "job_id", "queue_name", "priority", "status")
		for _, unblockedJob := range unblockedDbJobs {
			outboxInsertBuilder = outboxInsertBuilder.Values(
				uuid.New(),
				unblockedJob.ID,
				unblockedJob.Queue,
				unblockedJob.Priority,
				queue.OutboxStatusPending,
			)
		}
		if _, err := outboxInsertBuilder.ExecContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to create outbox events for unblocked jobs: %w", err)
		}
	}

	unblockedJobs := make([]*queue.Job, len(unblockedDbJobs))
	for i, j := range unblockedDbJobs {
		job, err := j.toDomain()
		if err != nil {
			return nil, fmt.Errorf("failed to convert unblocked db job: %w", err)
		}
		unblockedJobs[i] = job
	}
	return unblockedJobs, nil
}

func (r *QueueRepository) GetJobDependencies(ctx context.Context, jobID uuid.UUID) ([]*queue.Job, error) {
	queryBuilder := r.builder().Select("j.*").
		From("jobs j").
		Join("job_dependencies jd ON j.id = jd.depends_on_job_id").
		Where(squirrel.Eq{"jd.job_id": jobID})

	var dbJobs []dbJob
	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get job dependencies query: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &dbJobs, query, args...); err != nil {
		return nil, fmt.Errorf("failed to execute get job dependencies query for job %s: %w", jobID, err)
	}

	jobs := make([]*queue.Job, len(dbJobs))
	for i, j := range dbJobs {
		job, err := j.toDomain()
		if err != nil {
			return nil, fmt.Errorf("failed to convert db job dependency: %w", err)
		}
		jobs[i] = job
	}
	return jobs, nil
}

func (r *QueueRepository) GetJobDependents(ctx context.Context, jobID uuid.UUID) ([]*queue.Job, error) {
	queryBuilder := r.builder().Select("j.*").
		From("jobs j").
		Join("job_dependencies jd ON j.id = jd.job_id").
		Where(squirrel.Eq{"jd.depends_on_job_id": jobID})

	var dbJobs []dbJob
	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get job dependents query: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &dbJobs, query, args...); err != nil {
		return nil, fmt.Errorf("failed to execute get job dependents query for job %s: %w", jobID, err)
	}

	jobs := make([]*queue.Job, len(dbJobs))
	for i, j := range dbJobs {
		job, err := j.toDomain()
		if err != nil {
			return nil, fmt.Errorf("failed to convert db job dependent: %w", err)
		}
		jobs[i] = job
	}
	return jobs, nil
}

// --- Stats & Monitoring Methods ---

func (r *QueueRepository) GetAggregatedStats(ctx context.Context) (*queue.DashboardStats, error) {
	stats := &queue.DashboardStats{}
	query := `
		SELECT
			(SELECT COUNT(*) FROM jobs WHERE status IN ('PENDING', 'DELAYED', 'BLOCKED')) as total_pending,
			(SELECT COUNT(*) FROM jobs WHERE status = 'PROCESSING') as total_processing,
			(SELECT COUNT(*) FROM failed_jobs) as total_failed,
			(SELECT COUNT(*) FROM scheduled_jobs WHERE is_active = TRUE) as total_schedules
	`
	if err := sqlx.GetContext(ctx, r.runner, stats, query); err != nil {
		return nil, fmt.Errorf("failed to get aggregated stats: %w", err)
	}
	return stats, nil
}

func (r *QueueRepository) GetQueueStats(ctx context.Context) ([]queue.QueueStats, error) {
	query := `
		SELECT
			j.queue as name,
			COUNT(*) FILTER (WHERE j.status IN ('PENDING', 'DELAYED', 'BLOCKED')) as pending,
			COUNT(*) FILTER (WHERE j.status = 'PROCESSING') as processing,
			(SELECT COUNT(*) FROM failed_jobs fj WHERE fj.queue = j.queue) as failed
		FROM jobs j
		GROUP BY j.queue
		ORDER BY j.queue;
	`
	var stats []queue.QueueStats
	if err := sqlx.SelectContext(ctx, r.runner, &stats, query); err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}
	return stats, nil
}

type namedExecer interface {
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
}

func (r *QueueRepository) RecordHeartbeat(ctx context.Context, metrics *queue.WorkerMetrics) error {
	query := `
		INSERT INTO worker_metrics (id, host, queues, status, jobs_processed, last_heartbeat, started_at)
		VALUES (:id, :host, :queues, :status, :jobs_processed, :last_heartbeat, :started_at)
		ON CONFLICT (id) DO UPDATE SET
			host = EXCLUDED.host,
			queues = EXCLUDED.queues,
			status = EXCLUDED.status,
			jobs_processed = EXCLUDED.jobs_processed,
			last_heartbeat = EXCLUDED.last_heartbeat;`

	execer, ok := r.runner.(namedExecer)
	if !ok {
		return errors.New("internal error: runner does not support named exec")
	}

	_, err := execer.NamedExecContext(ctx, query, map[string]interface{}{
		"id":             metrics.ID,
		"host":           metrics.Host,
		"queues":         pq.Array(metrics.Queues),
		"status":         metrics.Status,
		"jobs_processed": metrics.JobsProcessed,
		"last_heartbeat": metrics.LastHeartbeat,
		"started_at":     metrics.StartedAt,
	})

	if err != nil {
		return fmt.Errorf("failed to record heartbeat for worker %s: %w", metrics.ID, err)
	}
	return nil
}

func (r *QueueRepository) GetActiveWorkers(ctx context.Context, heartbeatTimeout time.Duration) ([]*queue.WorkerMetrics, error) {
	queryBuilder := r.builder().Select("*").From("worker_metrics").
		Where(squirrel.GtOrEq{"last_heartbeat": time.Now().UTC().Add(-heartbeatTimeout)}).
		OrderBy("id")

	var dbWorkers []dbWorkerMetrics
	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build get active workers query: %w", err)
	}
	if err := sqlx.SelectContext(ctx, r.runner, &dbWorkers, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get active workers: %w", err)
	}

	workers := make([]*queue.WorkerMetrics, len(dbWorkers))
	for i, w := range dbWorkers {
		workers[i] = w.toDomain()
	}
	return workers, nil
}

// --- Maintenance Methods ---
func (r *QueueRepository) DeleteJobsOlderThan(ctx context.Context, status queue.JobStatus, t time.Time) (int64, error) {
	res, err := r.builder().Delete("jobs").
		Where(squirrel.Eq{"status": status}).
		Where(squirrel.Lt{"updated_at": t}).
		ExecContext(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to delete old jobs with status %s: %w", status, err)
	}
	return res.RowsAffected()
}

func (r *QueueRepository) DeleteFailedJobsOlderThan(ctx context.Context, t time.Time) (int64, error) {
	res, err := r.builder().Delete("failed_jobs").
		Where(squirrel.Lt{"failed_at": t}).
		ExecContext(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to delete old failed jobs: %w", err)
	}
	return res.RowsAffected()
}

func (r *QueueRepository) MaintainPartitions(ctx context.Context) error {
	_, err := r.runner.ExecContext(ctx, "SELECT create_analytics_partitions(3)")
	if err != nil {
		return fmt.Errorf("failed to maintain partitions: %w", err)
	}
	return nil
}

func (r *QueueRepository) DeleteJobsByQueue(ctx context.Context, queueName string) (int64, error) {
	res, err := r.builder().Delete("jobs").Where(squirrel.Eq{"queue": queueName}).ExecContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete jobs for queue '%s': %w", queueName, err)
	}
	return res.RowsAffected()
}

func (r *QueueRepository) DeleteFailedJobsByQueue(ctx context.Context, queueName string) (int64, error) {
	res, err := r.builder().Delete("failed_jobs").Where(squirrel.Eq{"queue": queueName}).ExecContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete failed_jobs for queue '%s': %w", queueName, err)
	}
	return res.RowsAffected()
}

// --- Outbox Methods ---

func (r *QueueRepository) FetchAndLockOutboxEvents(ctx context.Context, limit int) ([]*queue.OutboxEvent, error) {
	query, args, err := r.builder().Select("*").From("outbox_events").
		Where(squirrel.Eq{"status": queue.OutboxStatusPending}).
		OrderBy("created_at ASC").
		Limit(uint64(limit)).
		Suffix("FOR UPDATE SKIP LOCKED").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build fetch outbox query: %w", err)
	}

	var events []*queue.OutboxEvent
	if err := sqlx.SelectContext(ctx, r.runner, &events, query, args...); err != nil {
		return nil, fmt.Errorf("failed to fetch and lock outbox events: %w", err)
	}
	return events, nil
}

func (r *QueueRepository) UpdateOutboxEvent(ctx context.Context, event *queue.OutboxEvent) error {
	_, err := r.builder().Update("outbox_events").
		SetMap(outboxEventToMap(event)).
		Where(squirrel.Eq{"id": event.ID}).
		ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to update outbox event %s: %w", event.ID, err)
	}
	return nil
}

// --- Helper Structs and Methods ---

type dbWorkerMetrics struct {
	ID            string         `db:"id"`
	Host          string         `db:"host"`
	Queues        pq.StringArray `db:"queues"`
	Status        string         `db:"status"`
	JobsProcessed int64          `db:"jobs_processed"`
	LastHeartbeat time.Time      `db:"last_heartbeat"`
	StartedAt     time.Time      `db:"started_at"`
}

func (dwm *dbWorkerMetrics) toDomain() *queue.WorkerMetrics {
	return &queue.WorkerMetrics{
		ID:            dwm.ID,
		Host:          dwm.Host,
		Queues:        []string(dwm.Queues),
		Status:        dwm.Status,
		JobsProcessed: dwm.JobsProcessed,
		LastHeartbeat: dwm.LastHeartbeat,
		StartedAt:     dwm.StartedAt,
	}
}

type dbScheduledJob struct {
	ID             uuid.UUID      `db:"id"`
	Name           string         `db:"name"`
	Description    sql.NullString `db:"description"`
	Type           core.JobType   `db:"type"`
	Payload        []byte         `db:"payload"`
	Queue          string         `db:"queue"`
	CronExpression string         `db:"cron_expression"`
	Timezone       string         `db:"timezone"`
	IsActive       bool           `db:"is_active"`
	LastError      *string        `db:"last_error"`
	LastRunAt      *time.Time     `db:"last_run_at"`
	NextRunAt      *time.Time     `db:"next_run_at"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (dbs *dbScheduledJob) toDomain() *queue.ScheduledJob {
	return &queue.ScheduledJob{
		ID:             dbs.ID,
		Name:           dbs.Name,
		Description:    dbs.Description.String,
		Type:           dbs.Type,
		Payload:        dbs.Payload,
		Queue:          dbs.Queue,
		CronExpression: dbs.CronExpression,
		Timezone:       dbs.Timezone,
		IsActive:       dbs.IsActive,
		LastError:      dbs.LastError,
		LastRunAt:      dbs.LastRunAt,
		NextRunAt:      dbs.NextRunAt,
		CreatedAt:      dbs.CreatedAt,
		UpdatedAt:      dbs.UpdatedAt,
	}
}

type dbJob struct {
	ID             uuid.UUID      `db:"id"`
	Type           core.JobType   `db:"type"`
	Queue          string         `db:"queue"`
	Priority       int            `db:"priority"`
	Payload        []byte         `db:"payload"`
	Status         string         `db:"status"`
	Attempts       int            `db:"attempts"`
	MaxAttempts    int            `db:"max_attempts"`
	TimeoutSeconds int            `db:"timeout_seconds"`
	AvailableAt    time.Time      `db:"available_at"`
	Tags           pq.StringArray `db:"tags"`
	Metadata       []byte         `db:"metadata"`
	ParentJobID    *uuid.UUID     `db:"parent_job_id"`
	WorkflowID     *uuid.UUID     `db:"workflow_id"`
	Error          sql.NullString `db:"error"`
	Result         []byte         `db:"result"`
	ActorID        *uuid.UUID     `db:"actor_id"`
	ActorType      sql.NullString `db:"actor_type"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func (dj *dbJob) toDomain() (*queue.Job, error) {
	var errStr *string
	if dj.Error.Valid {
		errStr = &dj.Error.String
	}
	tags := []string(dj.Tags)
	if tags == nil {
		tags = []string{}
	}
	return &queue.Job{
		ID:          dj.ID,
		Type:        dj.Type,
		Queue:       dj.Queue,
		Priority:    dj.Priority,
		Payload:     dj.Payload,
		Status:      queue.JobStatus(dj.Status),
		Attempts:    dj.Attempts,
		MaxAttempts: dj.MaxAttempts,
		Timeout:     time.Duration(dj.TimeoutSeconds) * time.Second,
		AvailableAt: dj.AvailableAt,
		Tags:        tags,
		Metadata:    dj.Metadata,
		ParentJobID: dj.ParentJobID,
		WorkflowID:  dj.WorkflowID,
		Error:       errStr,
		Result:      dj.Result,
		ActorID:     dj.ActorID,
		ActorType:   dj.ActorType.String,
		CreatedAt:   dj.CreatedAt,
		UpdatedAt:   dj.UpdatedAt,
	}, nil
}

func jobToMap(job *queue.Job) (map[string]interface{}, error) {
	m := map[string]interface{}{
		"id":              job.ID,
		"type":            job.Type,
		"queue":           job.Queue,
		"priority":        job.Priority,
		"status":          job.Status,
		"attempts":        job.Attempts,
		"max_attempts":    job.MaxAttempts,
		"timeout_seconds": int(job.Timeout.Seconds()),
		"available_at":    job.AvailableAt,
		"tags":            pq.Array(job.Tags),
		"error":           job.Error,
		"metadata":        nil,
		"parent_job_id":   job.ParentJobID,
		"workflow_id":     job.WorkflowID,
		"actor_id":        job.ActorID,
		"actor_type":      job.ActorType,
		"created_at":      job.CreatedAt,
		"updated_at":      job.UpdatedAt,
	}

	if len(job.Payload) > 0 && string(job.Payload) != "null" {
		m["payload"] = string(job.Payload)
	} else {
		m["payload"] = "{}"
	}

	if len(job.Result) > 0 && string(job.Result) != "null" {
		m["result"] = string(job.Result)
	} else {
		m["result"] = nil
	}

	if len(job.Metadata) > 0 && string(job.Metadata) != "null" {
		m["metadata"] = string(job.Metadata)
	} else {
		m["metadata"] = nil
	}

	return m, nil
}

func failedJobToMap(failedJob *queue.FailedJob) map[string]interface{} {
	m := map[string]interface{}{
		"id":              failedJob.ID,
		"original_job_id": failedJob.JobID,
		"type":            failedJob.Type,
		"queue":           failedJob.Queue,
		"exception":       failedJob.Exception,
		"stack_trace":     failedJob.StackTrace,
		"attempts":        failedJob.Attempts,
		"worker_id":       failedJob.WorkerID,
		"worker_host":     failedJob.WorkerHost,
		"failed_at":       failedJob.FailedAt,
	}

	if len(failedJob.Payload) > 0 && string(failedJob.Payload) != "null" {
		m["payload"] = string(failedJob.Payload)
	} else {
		m["payload"] = "{}"
	}
	return m
}

func scheduledJobToMap(schedule *queue.ScheduledJob) map[string]interface{} {
	m := map[string]interface{}{
		"id":              schedule.ID,
		"name":            schedule.Name,
		"description":     schedule.Description,
		"type":            schedule.Type,
		"queue":           schedule.Queue,
		"cron_expression": schedule.CronExpression,
		"timezone":        schedule.Timezone,
		"is_active":       schedule.IsActive,
		"last_error":      schedule.LastError,
		"last_run_at":     schedule.LastRunAt,
		"next_run_at":     schedule.NextRunAt,
		"updated_at":      schedule.UpdatedAt,
	}

	if len(schedule.Payload) > 0 && string(schedule.Payload) != "null" {
		m["payload"] = string(schedule.Payload)
	} else {
		m["payload"] = "{}"
	}
	return m
}

func outboxEventToMap(event *queue.OutboxEvent) map[string]interface{} {
	m := map[string]interface{}{
		"id":         event.ID,
		"job_id":     event.JobID,
		"queue_name": event.QueueName,
		"priority":   event.Priority,
		"status":     event.Status,
		"attempts":   event.Attempts,
		"created_at": event.CreatedAt,
	}
	if event.LastAttemptAt != nil {
		m["last_attempt_at"] = event.LastAttemptAt
	}
	if event.ProcessingError != nil {
		m["processing_error"] = event.ProcessingError
	}
	if event.ProcessedAt != nil {
		m["processed_at"] = event.ProcessedAt
	}
	return m
}

type builderWithWhere[B any] interface {
	Where(pred interface{}, args ...interface{}) B
}

func buildFailedJobFilterQuery[B builderWithWhere[B]](builder B, filters queue.FailedJobFilters) B {
	if filters.Queue != nil {
		builder = builder.Where(squirrel.Eq{"queue": *filters.Queue})
	}
	if filters.Type != nil {
		builder = builder.Where(squirrel.Eq{"type": *filters.Type})
	}
	if filters.StartDate != nil {
		builder = builder.Where(squirrel.GtOrEq{"failed_at": *filters.StartDate})
	}
	if filters.EndDate != nil {
		builder = builder.Where(squirrel.LtOrEq{"failed_at": *filters.EndDate})
	}
	return builder
}

func buildJobFilterQuery[B builderWithWhere[B]](builder B, filters queue.JobSearchFilters) B {
	if filters.Queue != nil {
		builder = builder.Where(squirrel.Eq{"queue": *filters.Queue})
	}
	if filters.Type != nil {
		builder = builder.Where(squirrel.Eq{"type": *filters.Type})
	}
	if filters.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filters.Status})
	}
	if len(filters.Tags) > 0 {
		builder = builder.Where("tags @> ?", pq.Array(filters.Tags))
	}
	if filters.StartDate != nil {
		builder = builder.Where(squirrel.GtOrEq{"created_at": *filters.StartDate})
	}
	if filters.EndDate != nil {
		builder = builder.Where(squirrel.LtOrEq{"created_at": *filters.EndDate})
	}
	return builder
}

func (r *QueueRepository) EnqueueJob(ctx context.Context, job *queue.Job) error {
	return r.CreateJob(ctx, job)
}

func (r *QueueRepository) CreateJobForTest(ctx context.Context, job *queue.Job) error {
	return r.createJob(ctx, job)
}
