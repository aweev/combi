-- Auto-Generated Schema Isolation for event
CREATE SCHEMA IF NOT EXISTS event;
SET search_path TO event, public;

-- Drop old tables if they exist to allow clean migration
DROP TABLE IF EXISTS attendees;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS registrations;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS schedule_items;
DROP TABLE IF EXISTS speakers;
DROP TABLE IF EXISTS venues;

-- 1. Venues
CREATE TABLE venues (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    city VARCHAR(100) NOT NULL,
    country VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Speakers
CREATE TABLE speakers (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    bio TEXT,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Events
CREATE TABLE events (
    id UUID PRIMARY KEY,
    organization_id UUID, -- Optional link to external org system
    venue_id UUID REFERENCES venues(id) ON DELETE SET NULL,
    created_by UUID NOT NULL,
    
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(50) NOT NULL, -- DRAFT, PUBLISHED, etc.
    format VARCHAR(50) NOT NULL, -- PHYSICAL, VIRTUAL, HYBRID
    
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    timezone VARCHAR(50) NOT NULL,
    
    stream_url TEXT,
    properties JSONB DEFAULT '{}', -- Flexible config
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_events_slug ON events(slug);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_start_time ON events(start_time);

-- 4. Tickets
CREATE TABLE tickets (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    
    name VARCHAR(255) NOT NULL,
    price_amount BIGINT NOT NULL DEFAULT 0, -- stored in cents
    price_currency CHAR(3) NOT NULL DEFAULT 'USD',
    
    capacity INTEGER, -- NULL means unlimited
    registrations_count INTEGER NOT NULL DEFAULT 0,
    
    sale_starts_at TIMESTAMPTZ,
    sale_ends_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_event_id ON tickets(event_id);

-- 5. Registrations
CREATE TABLE registrations (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE RESTRICT,
    transaction_id UUID, -- Link to payment system
    
    status VARCHAR(50) NOT NULL, -- CONFIRMED, CANCELLED, ATTENDED
    checked_in_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure a user only registers once per event
    UNIQUE(user_id, event_id)
);

CREATE INDEX idx_registrations_event_id ON registrations(event_id);
CREATE INDEX idx_registrations_user_id ON registrations(user_id);

-- 6. Schedule Items
CREATE TABLE schedule_items (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    
    title VARCHAR(255) NOT NULL,
    description TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    track VARCHAR(100),
    
    speaker_ids JSONB DEFAULT '[]', -- Array of UUIDs
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedule_event_id ON schedule_items(event_id);