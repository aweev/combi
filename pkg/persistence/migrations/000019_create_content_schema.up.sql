-- Auto-Generated Schema Isolation for content
CREATE SCHEMA IF NOT EXISTS content;
SET search_path TO content, public;

CREATE TABLE spaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    locales TEXT[] NOT NULL DEFAULT '{en-US}',
    default_locale VARCHAR(10) NOT NULL DEFAULT 'en-US',
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

-- Only one default space allowed
CREATE UNIQUE INDEX idx_spaces_default ON spaces(is_default) WHERE is_default = true;

CREATE TABLE models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    api_key VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    fields JSONB NOT NULL DEFAULT '[]', -- Schema definition
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    
    UNIQUE (space_id, api_key)
);

CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE RESTRICT, -- Cannot delete model if entries exist
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    slug VARCHAR(255), -- Optional URL slug
    status VARCHAR(50) NOT NULL CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED', 'CHANGED', 'REJECTED')),
    
    -- The content itself
    fields JSONB NOT NULL DEFAULT '{}', 
    
    -- Versioning and Audit
    version INTEGER NOT NULL DEFAULT 1,
    published_version INTEGER,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    published_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_entries_model ON entries(model_id);
CREATE INDEX idx_entries_status ON entries(status);

CREATE TABLE versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id UUID NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    fields JSONB NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    
    UNIQUE (entry_id, version)
);