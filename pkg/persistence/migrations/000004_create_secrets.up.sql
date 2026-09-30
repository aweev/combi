-- Auto-Generated Schema Isolation for secret
CREATE SCHEMA IF NOT EXISTS secret;
SET search_path TO secret, public;

CREATE TABLE IF NOT EXISTS secrets (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    encrypted_value bytea NOT NULL,
    encryption_context bytea NOT NULL, -- Stores the Nonce/KeyID
    version integer NOT NULL DEFAULT 1,
    status text NOT NULL,
    rotation_policy jsonb, -- Stores {enabled, frequency, next_at}
    metadata jsonb,        -- Stores {owner, description, etc}
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

-- Index for listing by prefix
CREATE INDEX IF NOT EXISTS idx_secrets_name_prefix ON secrets(name text_pattern_ops);

-- Index for finding rotation candidates efficiently
CREATE INDEX IF NOT EXISTS idx_secrets_rotation ON secrets 
USING gin ((rotation_policy->'next_rotation_at'));