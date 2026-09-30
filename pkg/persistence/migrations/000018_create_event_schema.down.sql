-- Drop Schema Isolation for event
DROP SCHEMA IF EXISTS event CASCADE;

-- Drop tables in reverse order of dependency

DROP TABLE IF EXISTS schedule_items;
DROP TABLE IF EXISTS registrations;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS speakers;
DROP TABLE IF EXISTS venues;

-- Drop Indexes (Optional, as dropping tables drops indexes, but good for cleanup scripts if tables are kept)
-- Postgres drops indexes automatically when tables are dropped.