-- Drop Schema Isolation for queue
DROP SCHEMA IF EXISTS queue CASCADE;

-- Reverts the schema for the background job queueing system.
-- Tables are dropped in reverse order of creation to respect foreign key constraints.

DROP TABLE IF EXISTS job_dependencies;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS scheduled_jobs;
DROP TABLE IF EXISTS failed_jobs;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS worker_metrics;

DROP TRIGGER IF EXISTS trigger_notify_schedule_change ON scheduled_jobs;
DROP FUNCTION IF EXISTS notify_schedule_change;