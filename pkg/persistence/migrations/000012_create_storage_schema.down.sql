-- Drop Schema Isolation for storage
DROP SCHEMA IF EXISTS storage CASCADE;

DROP TABLE IF EXISTS file_uploads;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS storage_quotas;
DROP TABLE IF EXISTS storage_buckets;