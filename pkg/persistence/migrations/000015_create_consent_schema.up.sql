-- Auto-Generated Schema Isolation for consent
CREATE SCHEMA IF NOT EXISTS consent;
SET search_path TO consent, public;

CREATE TABLE agreements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(100) UNIQUE NOT NULL, -- "global-tos"
    type VARCHAR(50) NOT NULL CHECK (type IN ('TERMS_OF_SERVICE', 'PRIVACY_POLICY', 'MARKETING', 'DATA_SHARING', 'PHOTO_RELEASE', 'COOKIE_TRACKING')),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_mandatory BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE TABLE revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agreement_id UUID NOT NULL REFERENCES agreements(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL, -- "1.0", "2023-10-01"
    content_url TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL, -- SHA256 hex string
    published_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Ensure version uniqueness per agreement
    UNIQUE (agreement_id, version)
);

-- Optimize fetching the "Current" version
CREATE INDEX idx_revisions_agreement_active ON revisions(agreement_id, is_active);

CREATE TABLE user_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Loose coupling to Auth/User service
    revision_id UUID NOT NULL REFERENCES revisions(id) ON DELETE RESTRICT, -- Cannot delete a revision if users signed it
    status VARCHAR(50) NOT NULL CHECK (status IN ('ACTIVE', 'REVOKED', 'EXPIRED')),
    ip_address INET,
    user_agent TEXT,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    revoked_at TIMESTAMPTZ,
    
    -- A user can only have one active consent record per revision
    UNIQUE (user_id, revision_id)
);

CREATE INDEX idx_user_consents_user ON user_consents(user_id);