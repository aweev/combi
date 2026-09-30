-- Auto-Generated Schema Isolation for 000040
CREATE SCHEMA IF NOT EXISTS 000040;
SET search_path TO 000040, public;

-- Create Video Rooms
CREATE TABLE video_rooms (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL, -- MEETING, WEBINAR, BOARD
    status VARCHAR(50) NOT NULL, -- ACTIVE, SCHEDULED, ENDED
    
    ref_type VARCHAR(50), -- project, event
    ref_id UUID,
    
    max_participants INTEGER DEFAULT 0,
    is_recorded BOOLEAN DEFAULT FALSE,
    
    start_time TIMESTAMP WITH TIME ZONE,
    end_time TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for ref lookups
CREATE INDEX idx_video_rooms_ref ON video_rooms(ref_type, ref_id);

-- Create Video Participants
CREATE TABLE video_participants (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES video_rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    is_host BOOLEAN DEFAULT FALSE,
    is_muted BOOLEAN DEFAULT FALSE,
    video_off BOOLEAN DEFAULT FALSE,
    
    joined_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    left_at TIMESTAMP WITH TIME ZONE,
    
    UNIQUE(room_id, user_id)
);
