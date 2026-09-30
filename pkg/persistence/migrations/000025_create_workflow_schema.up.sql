-- Auto-Generated Schema Isolation for workflow
CREATE SCHEMA IF NOT EXISTS workflow;
SET search_path TO workflow, public;

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =================================================================================
-- 1. WORKFLOW DEFINITIONS
-- =================================================================================
CREATE TABLE IF NOT EXISTS workflow_definitions (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- e.g., 'VOLUNTEER_ONBOARDING'
    version INT NOT NULL DEFAULT 1,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Configuration (Stored as JSONB)
    config JSONB NOT NULL DEFAULT '{}',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure we don't have duplicate active definitions for the same type (optional, business rule dependant)
CREATE INDEX idx_workflow_definitions_type_active ON workflow_definitions(type) WHERE is_active = TRUE;

-- =================================================================================
-- 2. WORKFLOW INSTANCES
-- =================================================================================
CREATE TABLE IF NOT EXISTS workflow_instances (
    id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL REFERENCES workflow_definitions(id),
    
    -- Temporal Core Mapping
    temporal_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    
    -- Domain Context
    subject_id UUID NOT NULL, -- UserID, ExpenseID, etc.
    
    -- State
    status TEXT NOT NULL CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED', 'WAITING_FOR_INPUT')),
    current_step TEXT NOT NULL, -- Descriptive label of where it is
    
    -- Data Snapshots
    input_data JSONB NOT NULL DEFAULT '{}',
    result_data JSONB NOT NULL DEFAULT '{}',
    
    -- Timing
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_workflow_instances_subject ON workflow_instances(subject_id);
CREATE INDEX idx_workflow_instances_status ON workflow_instances(status);
CREATE INDEX idx_workflow_instances_type_status ON workflow_instances(workflow_id, status);

-- =================================================================================
-- 3. APPROVAL REQUESTS
-- =================================================================================
CREATE TABLE IF NOT EXISTS approval_requests (
    id UUID PRIMARY KEY,
    instance_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    
    -- Assignment
    assignee_role TEXT NOT NULL, -- e.g., 'finance_manager'
    assignee_id UUID,            -- Optional: Assign to specific person
    
    -- Content
    message TEXT NOT NULL,
    
    -- Decision State
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    decision_note TEXT,
    decided_by UUID,
    
    -- Timing
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ
);

-- Optimized for "My Pending Approvals" dashboard
CREATE INDEX idx_approval_requests_pending ON approval_requests(status, assignee_role, assignee_id);
CREATE INDEX idx_approval_requests_instance ON approval_requests(instance_id);