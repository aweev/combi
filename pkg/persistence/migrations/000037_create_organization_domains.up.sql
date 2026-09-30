-- Auto-Generated Schema Isolation for 000037
CREATE SCHEMA IF NOT EXISTS 000037;
SET search_path TO 000037, public;

CREATE TABLE organization_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    domain VARCHAR(255) UNIQUE NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_org_domains_org_id ON organization_domains(organization_id);
CREATE INDEX idx_org_domains_domain ON organization_domains(domain);

CREATE TRIGGER set_organization_domains_updated_at 
BEFORE UPDATE ON organization_domains 
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
