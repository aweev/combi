-- Auto-Generated Schema Isolation for integration
CREATE SCHEMA IF NOT EXISTS integration;
SET search_path TO integration, public;

-- INTEGRATIONS TABLE
CREATE TABLE integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL, -- e.g., 'STRIPE', 'BREVO'
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    credentials_encrypted BYTEA NOT NULL, -- Binary data
    encryption_context JSONB NOT NULL DEFAULT '{}', -- Metadata for encryption (key_id, alg)
    configuration JSONB NOT NULL DEFAULT '{}', -- Non-sensitive config (webhook_secret, etc)
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_integrations_provider_status ON integrations(provider, status);
CREATE TRIGGER set_integrations_updated_at BEFORE UPDATE ON integrations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- WEBHOOKS TABLE
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    integration_id UUID REFERENCES integrations(id) ON DELETE SET NULL, -- Keep logs even if config is deleted
    provider VARCHAR(50) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    payload JSONB NOT NULL,
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    received_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    processed_at TIMESTAMPTZ,
    -- Prevent processing the same event from the provider twice
    UNIQUE(provider, provider_event_id)
);

CREATE INDEX idx_webhooks_integration_id ON webhooks(integration_id);
CREATE INDEX idx_webhooks_status ON webhooks(status);