-- Drop Schema Isolation for notification
DROP SCHEMA IF EXISTS notification CASCADE;

DROP TABLE IF EXISTS notification_events;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS template_translations;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_providers;