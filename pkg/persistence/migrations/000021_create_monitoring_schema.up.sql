-- Auto-Generated Schema Isolation for monitoring
CREATE SCHEMA IF NOT EXISTS monitoring;
SET search_path TO monitoring, public;

CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    severity VARCHAR(50) NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    prom_ql_query TEXT NOT NULL,
    for_duration_ns BIGINT NOT NULL, -- Duration in Nanoseconds
    labels JSONB NOT NULL DEFAULT '{}', -- Key-Value pairs
    annotations JSONB NOT NULL DEFAULT '{}', -- Key-Value pairs
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE TABLE alert_silences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    matchers JSONB NOT NULL DEFAULT '{}', -- Key-Value pairs to match alerts
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    comment TEXT,
    created_by VARCHAR(255) NOT NULL, -- Username or System
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE INDEX idx_alert_silences_active ON alert_silences(ends_at);