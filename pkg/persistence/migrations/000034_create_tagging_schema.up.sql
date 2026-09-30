-- Auto-Generated Schema Isolation for tagging
CREATE SCHEMA IF NOT EXISTS tagging;
SET search_path TO tagging, public;

DROP TABLE IF EXISTS taggings;
DROP TABLE IF EXISTS tags;

-- 1. Tags
CREATE TABLE tags (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color VARCHAR(7) NOT NULL DEFAULT '#000000', -- Hex color
    count INTEGER NOT NULL DEFAULT 0,
    is_approved BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure a slug is unique within its type (e.g. Skill "Go" vs Topic "Go")
    UNIQUE (slug, type)
);

CREATE INDEX idx_tags_search ON tags(name text_pattern_ops);
CREATE INDEX idx_tags_count ON tags(count DESC);

-- 2. Taggings (Join Table)
CREATE TABLE taggings (
    id UUID PRIMARY KEY,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Prevent duplicate tagging of the same entity with the same tag
    UNIQUE (tag_id, entity_type, entity_id)
);

CREATE INDEX idx_taggings_entity ON taggings(entity_type, entity_id);
CREATE INDEX idx_taggings_tag ON taggings(tag_id, entity_type);