-- Drop Schema Isolation for messaging
DROP SCHEMA IF EXISTS messaging CASCADE;

ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_last_msg;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS channel_members;
DROP TABLE IF EXISTS channels;