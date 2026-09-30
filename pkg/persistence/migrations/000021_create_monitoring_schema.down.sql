-- Drop Schema Isolation for monitoring
DROP SCHEMA IF EXISTS monitoring CASCADE;

DROP TABLE IF EXISTS alert_silences;
DROP TABLE IF EXISTS alert_rules;