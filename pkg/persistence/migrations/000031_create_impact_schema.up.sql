-- Auto-Generated Schema Isolation for impact
CREATE SCHEMA IF NOT EXISTS impact;
SET search_path TO impact, public;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =================================================================================
-- 1. METRICS (Definitions)
-- =================================================================================
CREATE TABLE IF NOT EXISTS impact_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) NOT NULL UNIQUE, -- e.g., "lives_touched"
    name TEXT NOT NULL,
    unit VARCHAR(50) NOT NULL,        -- e.g., "people", "kg"
    type VARCHAR(50) NOT NULL CHECK (type IN ('CUMULATIVE', 'GAUGE')),
    description TEXT,
    is_funding_source BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =================================================================================
-- 2. AGGREGATES
-- =================================================================================
CREATE TABLE IF NOT EXISTS impact_daily_aggregates (
    metric_id UUID NOT NULL REFERENCES impact_metrics(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    value DOUBLE PRECISION NOT NULL DEFAULT 0,
    PRIMARY KEY (metric_id, date)
);

-- =================================================================================
-- 3. GOALS (Targets)
-- =================================================================================
CREATE TABLE IF NOT EXISTS impact_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_id UUID NOT NULL REFERENCES impact_metrics(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    
    target_value DOUBLE PRECISION NOT NULL,
    current_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_impact_goals_dates ON impact_goals(start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_impact_goals_metric ON impact_goals(metric_id);

-- =================================================================================
-- 4. ENTRIES (The Ledger)
-- =================================================================================
CREATE TABLE IF NOT EXISTS impact_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_id UUID NOT NULL REFERENCES impact_metrics(id) ON DELETE CASCADE,
    value DOUBLE PRECISION NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    
    -- Audit Trail
    source_service VARCHAR(100) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    
    -- Attribution
    project_id UUID,      -- Nullable
    organization_id UUID, -- Nullable
    
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_impact_entries_agg ON impact_entries(metric_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_impact_entries_org ON impact_entries(organization_id, occurred_at);

-- =================================================================================
-- 5. MAPPINGS (Events -> Metrics)
-- =================================================================================
CREATE TABLE IF NOT EXISTS impact_event_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(255) NOT NULL,
    metric_id UUID NOT NULL REFERENCES impact_metrics(id) ON DELETE CASCADE,
    value_path VARCHAR(255) NOT NULL, -- e.g., "data.amount"
    multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(event_type, metric_id)
);