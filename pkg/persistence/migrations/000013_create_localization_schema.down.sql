-- Drop Schema Isolation for localization
DROP SCHEMA IF EXISTS localization CASCADE;

DROP TABLE IF EXISTS translation_values;
DROP TABLE IF EXISTS translation_keys;
DROP TABLE IF EXISTS locales;