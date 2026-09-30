-- Drop Schema Isolation for analytics
DROP SCHEMA IF EXISTS analytics CASCADE;

DROP TABLE IF EXISTS analytics_daily_users;
DROP TABLE IF EXISTS analytics_daily_events;
DROP TABLE IF EXISTS analytics_hourly_users;
DROP TABLE IF EXISTS analytics_hourly_events;
DROP TABLE IF EXISTS analytics_events; -- Cascades to partitions
DROP TABLE IF EXISTS privacy_requests;
DROP FUNCTION IF EXISTS create_analytics_partitions;