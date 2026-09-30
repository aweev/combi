-- Auto-Generated Schema Isolation for form
CREATE SCHEMA IF NOT EXISTS form;
SET search_path TO form, public;

DROP TABLE IF EXISTS vote_records;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS form_definitions;

-- 1. Form Definitions
CREATE TABLE form_definitions (
    id UUID PRIMARY KEY,
    slug VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    
    schema JSONB NOT NULL DEFAULT '[]',
    voting_config JSONB, -- Nullable
    
    expires_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_forms_type ON form_definitions(type);
CREATE INDEX idx_forms_active ON form_definitions(is_active);

-- 2. Submissions
CREATE TABLE submissions (
    id UUID PRIMARY KEY,
    form_id UUID NOT NULL REFERENCES form_definitions(id) ON DELETE CASCADE,
    user_id UUID, -- Nullable for anonymous submissions
    
    status VARCHAR(50) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    score DOUBLE PRECISION,
    
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_submissions_form ON submissions(form_id);
CREATE INDEX idx_submissions_user ON submissions(user_id);

-- 3. Vote Records
-- Tracks WHO voted to prevent duplicates, separate from the "what" (submission)
CREATE TABLE vote_records (
    form_id UUID NOT NULL REFERENCES form_definitions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    voted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (form_id, user_id)
);