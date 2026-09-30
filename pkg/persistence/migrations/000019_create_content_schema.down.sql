-- Drop Schema Isolation for content
DROP SCHEMA IF EXISTS content CASCADE;

DROP TABLE IF EXISTS versions;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS spaces;