-- Auto-Generated Schema Isolation for achievement
CREATE SCHEMA IF NOT EXISTS achievement;
SET search_path TO achievement, public;

-- Enable UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Badges
CREATE TABLE IF NOT EXISTS badges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    image_url TEXT,
    type VARCHAR(50) NOT NULL, -- MILESTONE, EVENT, SKILL, MANUAL
    points INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Achievement Rules
CREATE TABLE IF NOT EXISTS achievement_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    badge_id UUID NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    
    rule_type VARCHAR(50) NOT NULL, -- OCCURRENCE, ACCUMULATION
    event_action VARCHAR(255) NOT NULL,
    
    -- Accumulation specifics
    metric_name VARCHAR(255), 
    threshold NUMERIC(10, 2),
    
    -- Occurrence specifics
    metadata_filter JSONB DEFAULT '{}',
    
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Data Integrity: Ensure ACCUMULATION rules have metrics and thresholds
    CONSTRAINT chk_rule_integrity CHECK (
        (rule_type = 'ACCUMULATION' AND metric_name IS NOT NULL AND threshold IS NOT NULL) OR
        (rule_type = 'OCCURRENCE')
    )
);

CREATE INDEX IF NOT EXISTS idx_rules_event_action ON achievement_rules(event_action) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_rules_badge_id ON achievement_rules(badge_id);

-- 3. User Badges (The "Earned" table)
CREATE TABLE IF NOT EXISTS user_badges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL, -- Logical Ref to Users table
    badge_id UUID NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason TEXT,
    ref_id UUID, -- Optional link to the event that triggered it
    
    CONSTRAINT uq_user_badge UNIQUE (user_id, badge_id)
);

CREATE INDEX IF NOT EXISTS idx_user_badges_user ON user_badges(user_id);
-- Optimization for Leaderboard calculations (covering index)
CREATE INDEX IF NOT EXISTS idx_user_badges_lookup ON user_badges(badge_id, user_id);

-- 4. User Progress (The "Metrics" table)
CREATE TABLE IF NOT EXISTS user_progress (
    user_id UUID NOT NULL,
    metric_name VARCHAR(255) NOT NULL,
    value NUMERIC(10, 2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (user_id, metric_name)
);