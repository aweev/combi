-- Drop Schema Isolation for achievement
DROP SCHEMA IF EXISTS achievement CASCADE;

DROP TABLE IF EXISTS user_progress;
DROP TABLE IF EXISTS user_badges;
DROP TABLE IF EXISTS achievement_rules;
DROP TABLE IF EXISTS badges;