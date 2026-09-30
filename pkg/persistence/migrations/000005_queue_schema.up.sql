-- Auto-Generated Schema Isolation for queue
CREATE SCHEMA IF NOT EXISTS queue;
SET search_path TO queue, public;

-- This migration sets up the complete schema for the background job queueing system.
-- It includes tables for active jobs, failed jobs, recurring schedules, dependencies,
-- and the transactional outbox pattern.

-- ============================================================================
-- JOBS TABLE
-- ============================================================================
-- This is the main table for storing units of work to be processed.
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(255) NOT NULL,
    queue VARCHAR(255) NOT NULL DEFAULT 'default',
    priority INT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED', 'DELAYED', 'BLOCKED')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    timeout_seconds INT NOT NULL DEFAULT 300, -- 5 minutes
    available_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    error TEXT,
    result JSONB,
    tags TEXT[],
    metadata JSONB,
    
    -- Workflow / Hierarchy Tracking
    parent_job_id UUID, -- For sub-jobs or retries
    workflow_id UUID,   -- For grouping related jobs in a sequence
    
    actor_id UUID,
    actor_type VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

COMMENT ON TABLE jobs IS 'Stores all background jobs to be processed by workers.';
COMMENT ON COLUMN jobs.type IS 'The job type, e.g., "notification.send_email".';
COMMENT ON COLUMN jobs.queue IS 'The queue this job belongs to, e.g., "default", "critical".';
COMMENT ON COLUMN jobs.priority IS 'Job priority, lower numbers are higher priority.';
COMMENT ON COLUMN jobs.status IS 'The current lifecycle status of the job.';
COMMENT ON COLUMN jobs.available_at IS 'A job is not available for processing until this time.';
COMMENT ON COLUMN jobs.tags IS 'An array of tags for searching and categorization.';

-- This is the most critical index for worker performance. It allows workers to
-- efficiently find the next available job in their assigned queue.
CREATE INDEX idx_jobs_fetch ON jobs(status, queue, priority, available_at)
WHERE status = 'PENDING' OR status = 'DELAYED';

CREATE INDEX idx_jobs_type ON jobs(type);
CREATE INDEX idx_jobs_status ON jobs(status);

-- NEW: Indexes for Workflow/Parent lookups to prevent timeouts during dependency checks
CREATE INDEX idx_jobs_parent_id ON jobs(parent_job_id);
CREATE INDEX idx_jobs_workflow_id ON jobs(workflow_id);

-- GIN index allows for fast searching of jobs by their tags.
CREATE INDEX idx_jobs_tags ON jobs USING GIN(tags);

CREATE TRIGGER set_jobs_updated_at
BEFORE UPDATE ON jobs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- FAILED_JOBS TABLE
-- ============================================================================
-- An immutable archive of jobs that have exhausted their retries.
CREATE TABLE failed_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_job_id UUID NOT NULL,
    type VARCHAR(255) NOT NULL,
    queue VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    exception TEXT NOT NULL,
    stack_trace TEXT,
    attempts INT NOT NULL,
    worker_id VARCHAR(255),
    worker_host VARCHAR(255),
    failed_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

COMMENT ON TABLE failed_jobs IS 'An immutable archive of jobs that have permanently failed.';
COMMENT ON COLUMN failed_jobs.original_job_id IS 'The ID of the job in the main jobs table.';
COMMENT ON COLUMN failed_jobs.exception IS 'The final error message that caused the failure.';

CREATE INDEX idx_failed_jobs_original_job_id ON failed_jobs(original_job_id);
CREATE INDEX idx_failed_jobs_queue ON failed_jobs(queue);
CREATE INDEX idx_failed_jobs_type ON failed_jobs(type);
CREATE INDEX idx_failed_jobs_failed_at ON failed_jobs(failed_at);


-- ============================================================================
-- SCHEDULED_JOBS TABLE
-- ============================================================================
-- Defines recurring jobs based on a CRON schedule.
CREATE TABLE scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    queue VARCHAR(255) NOT NULL,
    cron_expression VARCHAR(255) NOT NULL,
    timezone VARCHAR(255) NOT NULL DEFAULT 'UTC',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_error TEXT,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

COMMENT ON TABLE scheduled_jobs IS 'Defines recurring jobs and their schedules.';
COMMENT ON COLUMN scheduled_jobs.name IS 'A unique, human-readable name for the schedule.';
COMMENT ON COLUMN scheduled_jobs.cron_expression IS 'The CRON expression defining the run frequency.';

-- Critical index for the scheduler to find which jobs need to be enqueued.
CREATE INDEX idx_scheduled_jobs_next_run ON scheduled_jobs(is_active, next_run_at);

CREATE TRIGGER set_scheduled_jobs_updated_at
BEFORE UPDATE ON scheduled_jobs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- JOB_DEPENDENCIES TABLE
-- ============================================================================
-- A join table for creating dependency chains between jobs (job A must complete
-- before job B can start).
CREATE TABLE job_dependencies (
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    depends_on_job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    PRIMARY KEY (job_id, depends_on_job_id)
);

COMMENT ON TABLE job_dependencies IS 'Models dependencies between jobs.';
COMMENT ON COLUMN job_dependencies.job_id IS 'The job that is blocked.';
COMMENT ON COLUMN job_dependencies.depends_on_job_id IS 'The job that must be completed first.';

-- Critical index for finding "Which jobs are waiting on this job that just finished?"
CREATE INDEX idx_job_dependencies_depends_on_job_id ON job_dependencies(depends_on_job_id);


-- ============================================================================
-- OUTBOX_EVENTS TABLE
-- ============================================================================
-- Implements the transactional outbox pattern. Events are written here in the
-- same transaction as business logic, to be reliably published to the broker.
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    queue_name VARCHAR(255) NOT NULL,
    priority INT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSED', 'FAILED')),
    attempts INT NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    processing_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    processed_at TIMESTAMPTZ
);

COMMENT ON TABLE outbox_events IS 'Stores events for reliable, transactional publishing to a message broker.';

-- Critical index for the relay process to find pending events.
CREATE INDEX idx_outbox_events_pending ON outbox_events(status, created_at)
WHERE status = 'PENDING';

CREATE TABLE worker_metrics (
    id VARCHAR(255) PRIMARY KEY,
    host VARCHAR(255) NOT NULL,
    queues TEXT[] NOT NULL,
    status VARCHAR(50) NOT NULL,
    jobs_processed BIGINT NOT NULL DEFAULT 0,
    last_heartbeat TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL
);

COMMENT ON TABLE worker_metrics IS 'Stores heartbeat and metrics for active worker processes.';

-- ============================================================================
-- NOTIFICATION TRIGGERS
-- ============================================================================
-- This function allows the application (SchedulerService) to be notified instantly
-- via Postgres LISTEN/NOTIFY when a schedule is created, updated, or deleted.
-- This removes the need for the Scheduler to poll the database constantly for changes.

CREATE OR REPLACE FUNCTION notify_schedule_change() RETURNS TRIGGER AS $$
DECLARE
  row_id UUID;
BEGIN
  IF (TG_OP = 'DELETE') THEN
    row_id := OLD.id;
  ELSE
    row_id := NEW.id;
  END IF;
  
  -- Broadcast the ID of the changed schedule on the 'schedule_changes' channel.
  PERFORM pg_notify('schedule_changes', row_id::text);
  
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_notify_schedule_change
AFTER INSERT OR UPDATE OR DELETE ON scheduled_jobs
FOR EACH ROW EXECUTE FUNCTION notify_schedule_change();