-- Drop Schema Isolation for impact
DROP SCHEMA IF EXISTS impact CASCADE;

DROP TABLE IF EXISTS impact_daily_aggregates CASCADE;
DROP TABLE IF EXISTS impact_event_mappings CASCADE;
DROP TABLE IF EXISTS impact_goals CASCADE;
DROP TABLE IF EXISTS impact_entries CASCADE;
DROP TABLE IF EXISTS impact_metrics CASCADE;