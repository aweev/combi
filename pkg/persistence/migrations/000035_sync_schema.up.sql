-- Auto-Generated Schema Isolation for sync
CREATE SCHEMA IF NOT EXISTS sync;
SET search_path TO sync, public;

-- Migration: Create sync service tables
-- Description: Tracks client device sync states and operation history

-- Safe Cleanup (Optional: Drop old names if you renamed them)
DROP TABLE IF EXISTS sync_operations; 

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Devices: Tracks specific physical devices/browsers
CREATE TABLE IF NOT EXISTS sync_devices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    device_id VARCHAR(255) NOT NULL, -- The string sent by client (e.g. "android_123")
    label VARCHAR(255),              -- e.g. "John's iPhone"
    
    last_sync_at TIMESTAMPTZ,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_sync_devices_user_device UNIQUE(user_id, device_id)
);

-- 2. Sync Logs: History of sync sessions (useful for debugging/idempotency)
CREATE TABLE IF NOT EXISTS sync_logs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    
    sync_type VARCHAR(50) NOT NULL, -- 'PUSH' or 'PULL'
    status VARCHAR(50) NOT NULL,    -- 'SUCCESS', 'PARTIAL', 'ERROR'
    
    ops_count INT DEFAULT 0,
    error_message TEXT,
    
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_logs_user ON sync_logs(user_id, occurred_at DESC);