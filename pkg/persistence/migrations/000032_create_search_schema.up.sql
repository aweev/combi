-- Auto-Generated Schema Isolation for search
CREATE SCHEMA IF NOT EXISTS search;
SET search_path TO search, public;

-- Enable UUID if not enabled (standard for this project)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

DROP TRIGGER IF EXISTS tsvectorupdate ON search_documents;
DROP FUNCTION IF EXISTS search_documents_vector_update;
DROP TABLE IF EXISTS search_documents;

-- 1. Search Documents Table
CREATE TABLE search_documents (
    id TEXT PRIMARY KEY, -- Format "type:uuid" to ensure global uniqueness
    type TEXT NOT NULL,
    
    -- Filterable Attributes
    category TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    tags TEXT[] DEFAULT '{}',
    
    -- Security (RBAC): Array of strings like "public", "user:123", "role:admin"
    allowed_viewers TEXT[] NOT NULL DEFAULT '{}',
    
    -- Display Data (JSON blob to avoid hitting other tables for search results)
    display_data JSONB NOT NULL DEFAULT '{}',
    
    -- Raw Content (for highlighting)
    content TEXT NOT NULL DEFAULT '',
    
    -- Full Text Search Vector (The Magic)
    search_vector TSVECTOR
);

-- 2. Indexes
CREATE INDEX idx_search_vector ON search_documents USING GIN(search_vector);
CREATE INDEX idx_search_viewers ON search_documents USING GIN(allowed_viewers); -- Fast overlap check
CREATE INDEX idx_search_type ON search_documents(type);
CREATE INDEX idx_search_status ON search_documents(status);
CREATE INDEX idx_search_created ON search_documents(created_at DESC);

-- 3. Auto-Update Trigger function
-- Weighs fields differently: A (Title) > B (Tags/Desc) > D (Content)
CREATE OR REPLACE FUNCTION search_documents_vector_update() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('english', coalesce(NEW.display_data->>'title', '')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.display_data->>'subtitle', '')), 'A') ||
        setweight(to_tsvector('english', array_to_string(NEW.tags, ' ')), 'B') ||
        setweight(to_tsvector('english', coalesce(NEW.display_data->>'description', '')), 'B') ||
        setweight(to_tsvector('english', coalesce(NEW.content, '')), 'D');
    
    NEW.updated_at := NOW();
    
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER tsvectorupdate BEFORE INSERT OR UPDATE
ON search_documents FOR EACH ROW EXECUTE FUNCTION search_documents_vector_update();