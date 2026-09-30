-- Auto-Generated Schema Isolation for reporting
CREATE SCHEMA IF NOT EXISTS reporting;
SET search_path TO reporting, public;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Report Templates
CREATE TABLE IF NOT EXISTS report_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'GENERAL', 
    description TEXT,
    
    content TEXT NOT NULL, -- The actual template code (HTML/Liquid/Etc)
    param_schema JSONB NOT NULL DEFAULT '{}', -- JSON Schema for validation
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Generated Reports
CREATE TABLE IF NOT EXISTS generated_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID REFERENCES report_templates(id) ON DELETE SET NULL,
    requester_id UUID NOT NULL, -- User who asked for it
    
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, PROCESSING, COMPLETED, FAILED
    format VARCHAR(10) NOT NULL, -- PDF, CSV
    
    parameters JSONB NOT NULL DEFAULT '{}', -- Inputs used
    
    file_url TEXT, -- S3/Storage Link
    file_size BIGINT,
    error_reason TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ -- Auto-deletion policy
);

CREATE INDEX IF NOT EXISTS idx_generated_reports_requester ON generated_reports(requester_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_generated_reports_status ON generated_reports(status);