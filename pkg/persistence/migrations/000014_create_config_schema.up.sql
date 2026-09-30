-- Auto-Generated Schema Isolation for config
CREATE SCHEMA IF NOT EXISTS config;
SET search_path TO config, public;

DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS theme_configs;
DROP TABLE IF EXISTS system_settings;

-- 1. System Settings
CREATE TABLE system_settings (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('STRING', 'BOOLEAN', 'INTEGER', 'JSON')),
    description TEXT,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

-- 2. Themes (Consolidated Schema)
CREATE TABLE theme_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL DEFAULT '', -- Added directly
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    priority INTEGER NOT NULL DEFAULT 0,   -- Added directly
    variables JSONB NOT NULL DEFAULT '{}',
    active_from TIMESTAMPTZ,
    active_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE UNIQUE INDEX idx_theme_slug ON theme_configs(slug);

-- 3. Brands
CREATE TABLE brands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID,
    project_id UUID,
    name VARCHAR(255) NOT NULL,
    logo_url TEXT NOT NULL DEFAULT '',
    favicon_url TEXT NOT NULL DEFAULT '',
    theme_id UUID NOT NULL REFERENCES theme_configs(id),
    custom_overrides JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE UNIQUE INDEX idx_brands_org ON brands(organization_id) WHERE project_id IS NULL;
CREATE UNIQUE INDEX idx_brands_project ON brands(project_id) WHERE project_id IS NOT NULL;

-- 4. Initial Seed (Minimal)
-- The Go Seeder will handle the complex stuff, but we need the default theme for constraints.
INSERT INTO theme_configs (id, slug, name, is_system, variables) VALUES (
    '00000000-0000-0000-0000-000000000001', 
    'default-system',
    'Default System', 
    true, 
    '{"primary": "#D95D39", "secondary": "#F0A202", "bg-body": "#FFFFFF", "text-body": "#1A1A1A"}'
);

INSERT INTO brands (organization_id, project_id, name, theme_id) VALUES (
    NULL, NULL, 'Default Organization', '00000000-0000-0000-0000-000000000001'
);
