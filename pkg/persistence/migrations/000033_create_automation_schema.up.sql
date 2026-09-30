-- Auto-Generated Schema Isolation for automation
CREATE SCHEMA IF NOT EXISTS automation;
SET search_path TO automation, public;

-- =================================================================================
-- CLEANUP: Drop existing tables to ensure a fresh, correct schema
-- =================================================================================
DROP TABLE IF EXISTS automation_executions CASCADE;
DROP TABLE IF EXISTS automation_rules CASCADE;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =================================================================================
-- 1. AUTOMATION RULES
-- =================================================================================
CREATE TABLE automation_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL CHECK (status IN ('DRAFT', 'ACTIVE', 'INACTIVE')),
    priority INT NOT NULL DEFAULT 0,
    
    -- Configuration (JSONB)
    trigger JSONB NOT NULL,
    conditions JSONB NOT NULL DEFAULT '[]',
    logic TEXT NOT NULL DEFAULT 'AND' CHECK (logic IN ('AND', 'OR')),
    actions JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Execution Strategy (New Feature for Parallel Execution)
    execution_strategy TEXT NOT NULL DEFAULT 'SEQUENCE' CHECK (execution_strategy IN ('SEQUENCE', 'PARALLEL')),
    
    -- Lifecycle
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Stats (Aggregated)
    total_executions BIGINT NOT NULL DEFAULT 0,
    successful_executions BIGINT NOT NULL DEFAULT 0
);

-- Indexes for Rules
CREATE INDEX idx_automation_rules_status ON automation_rules(status);
CREATE INDEX idx_automation_rules_trigger_type ON automation_rules USING GIN ((trigger -> 'type')); 
CREATE INDEX idx_automation_rules_priority ON automation_rules(priority ASC, name ASC);

-- =================================================================================
-- 2. AUTOMATION EXECUTIONS
-- =================================================================================
CREATE TABLE automation_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    event_id UUID NOT NULL, 
    
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED', 'SKIPPED')),
    
    -- Data Snapshot
    trigger_data JSONB NOT NULL,
    
    -- Results
    conditions_result JSONB DEFAULT '[]',
    actions_result JSONB DEFAULT '[]',
    
    -- Timing & Recovery
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Critical for Crash Recovery
    completed_at TIMESTAMPTZ,
    duration BIGINT DEFAULT 0, -- Nanoseconds
    error TEXT
);

-- Indexes for Executions
CREATE INDEX idx_automation_executions_rule_id ON automation_executions(rule_id);
CREATE INDEX idx_automation_executions_status ON automation_executions(status);

-- Optimization for "Stale Execution Sweeper" (Recovery Service)
CREATE INDEX idx_automation_executions_recovery ON automation_executions(status, updated_at);

-- Optimization for "Pruning Service" (Database Bloat)
CREATE INDEX idx_automation_executions_pruning ON automation_executions(started_at);

-- Optimization for Stats Reporting
CREATE INDEX idx_automation_executions_stats ON automation_executions(started_at, status);