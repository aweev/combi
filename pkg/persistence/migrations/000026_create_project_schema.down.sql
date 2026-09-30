-- Drop Schema Isolation for project
DROP SCHEMA IF EXISTS project CASCADE;

DROP TABLE IF EXISTS project_activities;
DROP TABLE IF EXISTS project_files;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS milestones;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;