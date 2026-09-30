-- Auto-Generated Schema Isolation for search
CREATE SCHEMA IF NOT EXISTS search;
SET search_path TO search, public;

CREATE TABLE search_documents (
    id VARCHAR(255) PRIMARY KEY,      -- "user:uuid" or "program:uuid"
    type VARCHAR(50) NOT NULL,        -- "users", "content"
    
    -- Display Data (JSONB allows flexibility without schema changes)
    display_data JSONB NOT NULL,      -- {title, subtitle, image_url, link}
    
    -- Faceting & Security (Indexed for fast filtering)
    category VARCHAR(100),
    status VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL,
    tags TEXT[],                      -- Array for tag filtering
    allowed_viewers TEXT[],           -- Array for security: ["public", "user:123"]
    
    -- Full Text Search Vector
    -- We store title (A weight) and content (C weight) together
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(display_data->>'title', '')), 'A') ||
        setweight(to_tsvector('english', coalesce(display_data->>'description', '')), 'B') ||
        setweight(to_tsvector('english', coalesce(content, '')), 'C')
    ) STORED,
    
    content TEXT -- Original text for highlighting (optional, can be large)
);

-- Indexes for Speed
CREATE INDEX idx_search_type ON search_documents(type);
CREATE INDEX idx_search_vector ON search_documents USING GIN(search_vector);
CREATE INDEX idx_search_allowed_viewers ON search_documents USING GIN(allowed_viewers); -- Critical for security
CREATE INDEX idx_search_tags ON search_documents USING GIN(tags);