-- Drop Schema Isolation for config
DROP SCHEMA IF EXISTS config CASCADE;

DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS theme_configs;
DROP TABLE IF EXISTS system_settings;