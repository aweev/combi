-- Drop Schema Isolation for audit
DROP SCHEMA IF EXISTS audit CASCADE;

DROP TABLE IF EXISTS retention_policies;
DROP TABLE IF EXISTS legal_holds;
DROP TABLE IF EXISTS audit_events;