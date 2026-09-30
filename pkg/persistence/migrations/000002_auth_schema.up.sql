-- Auto-Generated Schema Isolation for auth
CREATE SCHEMA IF NOT EXISTS auth;
SET search_path TO auth, public;

-- This migration sets up the complete schema for the core Authentication service.
-- It includes tables for accounts, authentication methods (passwords, MFA), sessions,
-- and single-use tokens.

-- Enable extensions used for UUID generation.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- A reusable function to automatically update the `updated_at` timestamp on any table.
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = (NOW() AT TIME ZONE 'utc');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- ACCOUNTS TABLE
-- ============================================================================
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'UNVERIFIED' CHECK (status IN ('UNVERIFIED', 'ACTIVE', 'SUSPENDED', 'PENDING')),
    mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

COMMENT ON TABLE accounts IS 'Central identity record for authentication purposes.';
COMMENT ON COLUMN accounts.user_id IS 'Foreign key to the user profile in the UserService domain.';
COMMENT ON COLUMN accounts.status IS 'The lifecycle status of the account (e.g., pending verification, active).';
COMMENT ON COLUMN accounts.mfa_enabled IS 'A denormalized flag for quick MFA checks during login.';

CREATE INDEX idx_accounts_email ON accounts(email);
CREATE INDEX idx_accounts_user_id ON accounts(user_id);
CREATE INDEX idx_accounts_locked_until ON accounts(locked_until) WHERE locked_until IS NOT NULL;

CREATE TRIGGER set_accounts_updated_at
BEFORE UPDATE ON accounts
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- AUTH_METHODS TABLE
-- ============================================================================
CREATE TABLE auth_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('PASSWORD', 'TOTP', 'BACKUP_CODE', 'OAUTH')), -- Simplified CHECK for new `type`
    provider VARCHAR(50), -- ADDED: For OAuth providers like 'google', 'github'
    credential TEXT NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    -- UPDATED: New unique constraint to handle multiple OAuth providers per account
    UNIQUE (account_id, type, provider)
);

COMMENT ON TABLE auth_methods IS 'Stores credentials for an account, such as passwords or MFA secrets.';
COMMENT ON COLUMN auth_methods.type IS 'The type of authentication method (e.g., PASSWORD, OAUTH).';
COMMENT ON COLUMN auth_methods.provider IS 'Specifies the provider for OAUTH type methods (e.g., google, github).';
COMMENT ON COLUMN auth_methods.credential IS 'The credential data, format depends on the type (e.g., hash, provider user ID).';
COMMENT ON COLUMN auth_methods.verified IS 'Indicates if an MFA method has been confirmed by the user.';

CREATE INDEX idx_auth_methods_account_id ON auth_methods(account_id);

CREATE TRIGGER set_auth_methods_updated_at
BEFORE UPDATE ON auth_methods
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- ============================================================================
-- SESSIONS TABLE
-- ============================================================================
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) UNIQUE NOT NULL,
    device_fingerprint_hash VARCHAR(64) NOT NULL DEFAULT '',
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    revoked_at TIMESTAMPTZ
);

COMMENT ON TABLE sessions IS 'Stores active user sessions for refresh token rotation and management.';
COMMENT ON COLUMN sessions.refresh_token_hash IS 'The unique, hashed version of the refresh token.';
COMMENT ON COLUMN sessions.last_used_at IS 'Timestamp of the last time this session was used to refresh tokens.';
COMMENT ON COLUMN sessions.revoked_at IS 'When the session was revoked. NULL means active.';

CREATE INDEX idx_sessions_account_id ON sessions(account_id);
CREATE INDEX idx_sessions_device_fingerprint_hash ON sessions(device_fingerprint_hash);


-- ============================================================================
-- TOKENS TABLE
-- ============================================================================
CREATE TABLE tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('EMAIL_VERIFICATION', 'PASSWORD_RESET', 'MFA_CHALLENGE')),
    hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

COMMENT ON TABLE tokens IS 'Stores single-use tokens for actions like password resets.';
COMMENT ON COLUMN tokens.used_at IS 'If not NULL, the token has been consumed and is invalid.';

CREATE INDEX idx_tokens_hash_type_unused ON tokens(hash, type) WHERE used_at IS NULL;
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
-- Unique provider per account (only for OAuth)
CREATE UNIQUE INDEX idx_auth_oauth_provider ON auth_methods (account_id, provider) WHERE type = 'OAUTH';
-- Only one password per account
CREATE UNIQUE INDEX idx_auth_password ON auth_methods (account_id) WHERE type = 'PASSWORD';