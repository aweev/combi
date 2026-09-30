-- Auto-Generated Schema Isolation for messaging
CREATE SCHEMA IF NOT EXISTS messaging;
SET search_path TO messaging, public;

-- Enable UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Channels
CREATE TABLE IF NOT EXISTS channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type VARCHAR(50) NOT NULL, -- DIRECT, GROUP, PROJECT, EVENT
    name VARCHAR(255),         -- Nullable for DIRECT
    description TEXT,
    avatar_url TEXT,
    
    -- Context Linking
    ref_type VARCHAR(50),      -- project, event
    ref_id UUID,

    -- Denormalization for sorting
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_id UUID,      -- FK added later to avoid circular dependency issues during creation if needed

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing user channels efficiently usually involves joins, 
-- but we index the context lookup for "Find chat for Project X"
CREATE INDEX IF NOT EXISTS idx_channels_ref ON channels(ref_type, ref_id);

-- 2. Channel Members
CREATE TABLE IF NOT EXISTS channel_members (
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id UUID NOT NULL, -- Logical ref to Users table
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- owner, admin, member
    
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_msg_id UUID, -- Helpful for "Jump to unread"
    
    notifications BOOLEAN NOT NULL DEFAULT TRUE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_members_user ON channel_members(user_id);

-- 3. Messages
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL, -- Logical ref
    type VARCHAR(50) NOT NULL DEFAULT 'TEXT',
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}', -- Attachments, dimensions, etc.
    
    reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMPTZ, -- Soft Delete
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for scrolling history (cursor based pagination)
CREATE INDEX IF NOT EXISTS idx_messages_channel_time ON messages(channel_id, created_at DESC);

-- Add the circular FK for optimization now that table exists
ALTER TABLE channels 
ADD CONSTRAINT fk_channels_last_msg 
FOREIGN KEY (last_message_id) REFERENCES messages(id) ON DELETE SET NULL;