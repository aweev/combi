-- Drop Schema Isolation for matchmaking
DROP SCHEMA IF EXISTS matchmaking CASCADE;

DROP TABLE IF EXISTS match_relationships;
DROP TABLE IF EXISTS match_suggestions;
DROP TABLE IF EXISTS match_candidates;
DROP TABLE IF EXISTS match_requests;