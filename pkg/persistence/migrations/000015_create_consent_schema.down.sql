-- Drop Schema Isolation for consent
DROP SCHEMA IF EXISTS consent CASCADE;

DROP TABLE IF EXISTS user_consents;
DROP TABLE IF EXISTS revisions;
DROP TABLE IF EXISTS agreements;