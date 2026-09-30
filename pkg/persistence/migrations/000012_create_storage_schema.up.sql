-- Auto-Generated Schema Isolation for storage
CREATE SCHEMA IF NOT EXISTS storage;
SET search_path TO storage, public;

-- STORAGE BUCKETS
CREATE TABLE storage_buckets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    provider VARCHAR(50) NOT NULL,
    region VARCHAR(100) NOT NULL,
    endpoint TEXT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE UNIQUE INDEX idx_storage_buckets_default ON storage_buckets(is_default) WHERE is_default = TRUE;
CREATE TRIGGER set_storage_buckets_updated_at BEFORE UPDATE ON storage_buckets FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- STORAGE QUOTAS
CREATE TABLE storage_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    owner_type VARCHAR(50) NOT NULL,
    limit_bytes BIGINT NOT NULL DEFAULT 0, -- 0 = Unlimited
    used_bytes BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    UNIQUE(owner_id, owner_type)
);

-- FILES
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    owner_type VARCHAR(50) NOT NULL,
    bucket_id UUID NOT NULL REFERENCES storage_buckets(id),
    original_name VARCHAR(255) NOT NULL,
    path TEXT NOT NULL, -- Full path in the bucket
    mime_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    category VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    visibility VARCHAR(50) NOT NULL DEFAULT 'PRIVATE',
    hash VARCHAR(255), -- SHA256 or similar
    
    -- Metadata stored as JSONB
    metadata JSONB NOT NULL DEFAULT '{}', -- width, height, duration, custom fields
    tags TEXT[] DEFAULT '{}',

    -- Lifecycle
    upload_completed_at TIMESTAMPTZ,
    last_accessed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_files_owner ON files(owner_id);
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_bucket ON files(bucket_id);
CREATE TRIGGER set_files_updated_at BEFORE UPDATE ON files FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- FILE UPLOADS (Sessions)
CREATE TABLE file_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_ref TEXT, -- UploadID from S3
    total_size BIGINT NOT NULL,
    bytes_uploaded BIGINT NOT NULL DEFAULT 0,
    is_multipart BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_file_uploads_expiry ON file_uploads(expires_at);