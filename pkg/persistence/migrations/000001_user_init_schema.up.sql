-- Auto-Generated Schema Isolation for user
CREATE SCHEMA IF NOT EXISTS user;
SET search_path TO user, public;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = (NOW() AT TIME ZONE 'utc');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- PRIVACY REQUESTS (Layer 0)
-- This table handles GDPR "Right to be Forgotten" requests.
-- It is independent of the Auth system.
CREATE TABLE privacy_requests (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id uuid NOT NULL, -- Loose coupling: We assume a User UUID exists somewhere.
    type text NOT NULL CHECK (type IN ('ERASURE', 'EXPORT')),
    status text NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED')),
    requested_at timestamptz NOT NULL DEFAULT NOW(),
    completed_at timestamptz,
    reason text,
    result_location text,     -- S3 URL for Export results
    erasure_details jsonb     -- JSON report of deleted data
);

CREATE INDEX idx_privacy_requests_user ON privacy_requests(user_id);
CREATE INDEX idx_privacy_requests_status ON privacy_requests(status);


-- Enable extension for UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Helper for timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = (NOW() AT TIME ZONE 'utc');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- USERS TABLE
-- ============================================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'ACTIVE', 'DEACTIVATED', 'ANONYMIZED')),
    is_superuser BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_users_email ON users(email);
CREATE TRIGGER set_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- PROFILES TABLE
-- ============================================================================
-- 1:1 relationship with users. The ID is the User ID.
CREATE TABLE profiles (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    handle VARCHAR(100) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_file_id UUID,
    preferences JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_profiles_handle ON profiles(handle);
CREATE TRIGGER set_profiles_updated_at BEFORE UPDATE ON profiles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- CONNECTIONS TABLE
-- ============================================================================
CREATE TABLE connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('FOLLOWING', 'BLOCKING')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    UNIQUE(source_user_id, target_user_id, type)
);

CREATE INDEX idx_connections_source_type ON connections(source_user_id, type);
CREATE INDEX idx_connections_target_type ON connections(target_user_id, type);