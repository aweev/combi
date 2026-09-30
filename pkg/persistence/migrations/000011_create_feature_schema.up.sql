-- Auto-Generated Schema Isolation for feature
CREATE SCHEMA IF NOT EXISTS feature;
SET search_path TO feature, public;

CREATE TABLE feature_flags (
    key VARCHAR(255) PRIMARY KEY, -- e.g. "billing.new_checkout"
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL CHECK (type IN ('BOOLEAN', 'STRING', 'NUMBER', 'JSON')),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Core logic stored as JSONB for flexibility
    variations JSONB NOT NULL DEFAULT '[]',
    targeting_rules JSONB NOT NULL DEFAULT '[]',
    
    default_variation_id VARCHAR(255) NOT NULL,
    off_variation_id VARCHAR(255) NOT NULL,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_by UUID NOT NULL -- Audit trail of who changed the flag
);

-- Index for fast audit logging or listing recently changed flags
CREATE INDEX idx_feature_flags_updated_at ON feature_flags(updated_at DESC);