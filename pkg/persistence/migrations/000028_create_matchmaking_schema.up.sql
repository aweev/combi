-- Auto-Generated Schema Isolation for matchmaking
CREATE SCHEMA IF NOT EXISTS matchmaking;
SET search_path TO matchmaking, public;

-- Enable UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Match Requests (Demand)
CREATE TABLE IF NOT EXISTS match_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL, -- The Mentee
    type VARCHAR(50) NOT NULL, -- MENTORSHIP, VOLUNTEER_JOB
    status VARCHAR(50) NOT NULL DEFAULT 'OPEN', -- OPEN, MATCHED, CANCELLED
    
    criteria JSONB NOT NULL DEFAULT '{}', -- The "Needs" vector
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_match_requests_status ON match_requests(status);
CREATE INDEX IF NOT EXISTS idx_match_requests_user ON match_requests(user_id);

-- 2. Match Candidates (Supply)
CREATE TABLE IF NOT EXISTS match_candidates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL, -- The Mentor (Logical Ref to Users)
    type VARCHAR(50) NOT NULL,
    
    attributes JSONB NOT NULL DEFAULT '{}', -- The "Skills" vector
    
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one candidate profile per type per user
    CONSTRAINT uq_candidate_user_type UNIQUE (user_id, type)
);

-- GIN Index for fast searching of candidates by skills/attributes
CREATE INDEX IF NOT EXISTS idx_match_candidates_attributes ON match_candidates USING GIN (attributes);
CREATE INDEX IF NOT EXISTS idx_match_candidates_available ON match_candidates(type) WHERE is_available = TRUE;

-- 3. Match Suggestions (Algo Output)
CREATE TABLE IF NOT EXISTS match_suggestions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES match_requests(id) ON DELETE CASCADE,
    candidate_id UUID NOT NULL REFERENCES match_candidates(id) ON DELETE CASCADE,
    
    score NUMERIC(5, 4) NOT NULL, -- 0.0000 to 1.0000
    reason JSONB, -- Explainability
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Prevent duplicate suggestions for same pair
    CONSTRAINT uq_suggestion_pair UNIQUE (request_id, candidate_id)
);

CREATE INDEX IF NOT EXISTS idx_suggestions_req_score ON match_suggestions(request_id, score DESC);

-- 4. Match Relationships (Active Lifecycle)
CREATE TABLE IF NOT EXISTS match_relationships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES match_requests(id),
    candidate_id UUID NOT NULL REFERENCES match_candidates(id),
    
    -- Denormalized for easier querying
    mentee_id UUID NOT NULL, 
    mentor_id UUID NOT NULL,
    
    status VARCHAR(50) NOT NULL, -- PROPOSED, PENDING, ACTIVE, etc.
    
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    
    mentor_feedback TEXT,
    mentee_feedback TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_match_relationships_mentee ON match_relationships(mentee_id);
CREATE INDEX IF NOT EXISTS idx_match_relationships_mentor ON match_relationships(mentor_id);