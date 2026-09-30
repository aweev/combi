-- Drop Schema Isolation for sync
DROP SCHEMA IF EXISTS sync CASCADE;

-- Migration: Create sync service tables
-- Description: Tracks client device sync states and operation history

DROP TABLE IF EXISTS sync_operations;
DROP TABLE IF EXISTS sync_devices;