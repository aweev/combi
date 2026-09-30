package domain

import (
	"time"

	"github.com/google/uuid"
)

type RoomStatus string

const (
	RoomStatusActive    RoomStatus = "ACTIVE"
	RoomStatusScheduled RoomStatus = "SCHEDULED"
	RoomStatusEnded     RoomStatus = "ENDED"
)

type RoomType string

const (
	RoomTypeMeeting RoomType = "MEETING" // 1-on-1 or small group
	RoomTypeWebinar RoomType = "WEBINAR" // One-to-many
	RoomTypeBoard   RoomType = "BOARD"   // Highly secure, recorded
)

type Room struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Title       string     `json:"title" db:"title"`
	Description string     `json:"description" db:"description"`
	Type        RoomType   `json:"type" db:"type"`
	Status      RoomStatus `json:"status" db:"status"`

	// Linking to other domains
	RefType string    `json:"ref_type" db:"ref_type"` // e.g., "project", "event"
	RefID   uuid.UUID `json:"ref_id" db:"ref_id"`

	MaxParticipants int  `json:"max_participants" db:"max_participants"`
	IsRecorded      bool `json:"is_recorded" db:"is_recorded"`

	StartTime *time.Time `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time" db:"end_time"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Participant struct {
	ID       uuid.UUID  `json:"id" db:"id"`
	RoomID   uuid.UUID  `json:"room_id" db:"room_id"`
	UserID   uuid.UUID  `json:"user_id" db:"user_id"`
	IsHost   bool       `json:"is_host" db:"is_host"`
	IsMuted  bool       `json:"is_muted" db:"is_muted"`
	VideoOff bool       `json:"video_off" db:"video_off"`
	JoinedAt time.Time  `json:"joined_at" db:"joined_at"`
	LeftAt   *time.Time `json:"left_at" db:"left_at"`
}

type SessionDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type IceCandidate struct {
	Candidate     string  `json:"candidate"`
	SDPMid        *string `json:"sdpMid"`
	SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
}
