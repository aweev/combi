-- Auto-Generated Schema Isolation for analytics
CREATE SCHEMA IF NOT EXISTS analytics;
SET search_path TO analytics, public;

-- =================================================================================
-- 1. BASE TABLE (PARTITIONED)
-- =================================================================================
-- We use partitioning to handle millions of rows. 
-- Old partitions can be dropped instantly (0ms latency) vs DELETE (slow).

CREATE TABLE IF NOT EXISTS analytics_events (
    id UUID NOT NULL,
    user_id UUID,
    session_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    
    -- Context
    path TEXT,
    referrer TEXT,
    
    -- Technical (Raw)
    user_agent TEXT,
    ip_address VARCHAR(45), -- IPv6 support
    
    -- ENRICHMENT (The Magic)
    -- Parsed before insert to allow fast SQL grouping without Regex
    device_type VARCHAR(50), -- 'mobile', 'desktop', 'tablet', 'bot'
    country VARCHAR(2),      -- 'US', 'DE', 'NG' (ISO 3166-1 alpha-2)
    
    -- Flexible Payload
    properties JSONB DEFAULT '{}',
    
    -- Partition Key
    created_at TIMESTAMPTZ NOT NULL,
    
    -- Composite PK required for partitioning
    PRIMARY KEY (id, created_at) 
) PARTITION BY RANGE (created_at);

-- =================================================================================
-- 2. INDEXES
-- =================================================================================
-- Indexes are automatically applied to all partitions created from the parent.

-- Fast lookup by User (User History)
CREATE INDEX IF NOT EXISTS idx_analytics_user ON analytics_events(user_id, created_at);

-- Fast lookup by Session (Anonymous Flow)
CREATE INDEX IF NOT EXISTS idx_analytics_session ON analytics_events(session_id, created_at);

-- Fast Event Counts ("How many signups yesterday?")
CREATE INDEX IF NOT EXISTS idx_analytics_type_time ON analytics_events(event_type, created_at);

-- Fast filtering by Enrichment ("Mobile users in Germany")
CREATE INDEX IF NOT EXISTS idx_analytics_device_geo ON analytics_events(device_type, country, created_at);

-- JSONB Index for querying inside properties ("Find events where properties->>'sku' = '123'")
CREATE INDEX IF NOT EXISTS idx_analytics_properties ON analytics_events USING GIN (properties);

-- =================================================================================
-- 3. DEFAULT PARTITION & MAINTENANCE FUNCTION
-- =================================================================================

-- Safety Net: Holds any data that falls outside created ranges (e.g. clock skew)
CREATE TABLE IF NOT EXISTS analytics_events_default PARTITION OF analytics_events DEFAULT;

-- Function: Automatically generates monthly partitions
-- NOTE: 'PARTITION OF' automatically inherits all columns (device_type, country) from parent.
CREATE OR REPLACE FUNCTION create_analytics_partitions(months_ahead INT) RETURNS VOID AS $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
    i INT;
BEGIN
    -- Start from the first day of the current month
    start_date := date_trunc('month', CURRENT_DATE);
    
    FOR i IN 0..months_ahead LOOP
        end_date := start_date + INTERVAL '1 month';
        
        -- Naming convention: analytics_events_y2024m05
        partition_name := 'analytics_events_y' || to_char(start_date, 'YYYY') || 'm' || to_char(start_date, 'MM');
        
        -- Check if table exists to be idempotent
        IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = partition_name) THEN
            RAISE NOTICE 'Creating partition: % (Range: % to %)', partition_name, start_date, end_date;
            
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF analytics_events FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
        END IF;
        
        -- Move to next month
        start_date := start_date + INTERVAL '1 month';
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Initialize: Create partitions for this month + next 2 months
SELECT create_analytics_partitions(2);

-- =================================================================================
-- 4. AGGREGATION TABLES (ROLLUPS)
-- =================================================================================
-- These tables make dashboards load in <100ms by pre-calculating counts.

-- 4.1 Daily Rollups (Long-term trends, >32 days)
CREATE TABLE IF NOT EXISTS analytics_daily_events (
    date DATE NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date, event_type)
);

CREATE TABLE IF NOT EXISTS analytics_daily_users (
    date DATE NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date)
);

-- 4.2 Hourly Rollups (Recent history, 1-32 days)
CREATE TABLE IF NOT EXISTS analytics_hourly_events (
    time TIMESTAMPTZ NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (time, event_type)
);

CREATE TABLE IF NOT EXISTS analytics_hourly_users (
    time TIMESTAMPTZ NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (time)
);

-- Indexes for Range Scans on Rollups
CREATE INDEX IF NOT EXISTS idx_daily_events_date ON analytics_daily_events(date);
CREATE INDEX IF NOT EXISTS idx_hourly_events_time ON analytics_hourly_events(time);