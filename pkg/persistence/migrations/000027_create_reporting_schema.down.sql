-- Drop Schema Isolation for reporting
DROP SCHEMA IF EXISTS reporting CASCADE;

DROP TABLE IF EXISTS generated_reports;
DROP TABLE IF EXISTS report_templates;