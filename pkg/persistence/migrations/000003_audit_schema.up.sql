-- Auto-Generated Schema Isolation for audit
CREATE SCHEMA IF NOT EXISTS audit;
SET search_path TO audit, public;

-- AUDIT EVENTS
CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL,
    actor_id uuid NOT NULL,
    actor_type text NOT NULL,
    action text NOT NULL,
    outcome text NOT NULL,
    outcome_reason text,
    target_id uuid,
    target_type text,
    encrypted_data bytea,
    encryption_context jsonb,
    ip_address text,
    user_agent text,
    trace_id text
);

CREATE INDEX idx_audit_actor ON audit_events(actor_id);
CREATE INDEX idx_audit_target ON audit_events(target_id);
CREATE INDEX idx_audit_action ON audit_events(action);
CREATE INDEX idx_audit_created_at ON audit_events USING BRIN(created_at);

-- LEGAL HOLDS
CREATE TABLE legal_holds (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    description text,
    hold_subject text NOT NULL,
    reason text,                 -- Added matching the struct
    filter jsonb NOT NULL,       -- Added matching the struct
    active boolean NOT NULL DEFAULT true,
    created_by uuid NOT NULL,
    issued_at timestamptz NOT NULL DEFAULT NOW(),
    released_at timestamptz,
    released_by uuid
);

-- RETENTION POLICIES
CREATE TABLE retention_policies (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    description text,
    data_type text NOT NULL,
    duration_days integer NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);