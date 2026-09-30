-- Auto-Generated Schema Isolation for 000038
CREATE SCHEMA IF NOT EXISTS 000038;
SET search_path TO 000038, public;

-- Add organization_id to content schema for multi-tenancy
ALTER TABLE spaces ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE models ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE entries ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

-- Backfill organization_id for existing rows if any (assuming a system org or first org)
-- For a fresh dev environment, we can leave them NULL or set to a default.
-- For this transition, we allow NULL to represent "System/Global" content if needed, 
-- or we can enforce it. The plan says "Support individual Organizations", 
-- so most content should have an org_id.

CREATE INDEX idx_spaces_org ON spaces(organization_id);
CREATE INDEX idx_models_org ON models(organization_id);
CREATE INDEX idx_entries_org ON entries(organization_id);

-- Unique constraint for models should now include organization_id
ALTER TABLE models DROP CONSTRAINT models_space_id_api_key_key;
ALTER TABLE models ADD CONSTRAINT models_space_org_api_key_key UNIQUE (space_id, organization_id, api_key);
