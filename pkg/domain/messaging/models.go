package messaging

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type ChannelType string

const (
	TypeDirect  ChannelType = "DIRECT"  // 1-on-1
	TypeGroup   ChannelType = "GROUP"   // Ad-hoc group
	TypeProject ChannelType = "PROJECT" // Linked to a specific Project
	TypeEvent   ChannelType = "EVENT"   // Linked to a specific Event
)

type MessageType string

const (
	MsgText   MessageType = "TEXT"
	MsgImage  MessageType = "IMAGE"
	MsgFile   MessageType = "FILE"
	MsgSystem MessageType = "SYSTEM" // e.g., "Alice joined the chat"
)

// --- Core Models ---

// Channel represents a conversation context.
type Channel struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	Type        ChannelType `json:"type" db:"type"`
	Name        *string     `json:"name" db:"name"` // Null for Direct messages
	Description *string     `json:"description" db:"description"`

	AvatarURL *string `json:"avatar_url" db:"avatar_url"`

	// Context linking (e.g., Project ID)
	RefType *string    `json:"ref_type" db:"ref_type"`
	RefID   *uuid.UUID `json:"ref_id" db:"ref_id"`

	LastMessageID *uuid.UUID `json:"last_message_id" db:"last_message_id"`
	LastMessageAt time.Time  `json:"last_message_at" db:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// ChannelMember tracks who is in a conversation and what they've read.
type ChannelMember struct {
	ChannelID uuid.UUID `json:"channel_id" db:"channel_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Role      string    `json:"role" db:"role"` // "owner", "admin", "member"

	// Read State
	LastReadAt    time.Time  `json:"last_read_at" db:"last_read_at"`
	LastReadMsgID *uuid.UUID `json:"last_read_msg_id" db:"last_read_msg_id"`

	Notifications bool      `json:"notifications" db:"notifications"` // Mute toggle
	JoinedAt      time.Time `json:"joined_at" db:"joined_at"`
}

// Message represents a single communication unit.
type Message struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	ChannelID uuid.UUID   `json:"channel_id" db:"channel_id"`
	SenderID  uuid.UUID   `json:"sender_id" db:"sender_id"` // SystemActorID for system messages
	Type      MessageType `json:"type" db:"type"`
	Content   string      `json:"content" db:"content"` // Text or Caption

	// Attachments (for Image/File types)
	// e.g. { "url": "...", "size": 1024, "mime": "image/png" }
	Metadata json.RawMessage `json:"metadata" db:"metadata"`

	// Threading
	ReplyToID *uuid.UUID `json:"reply_to_id" db:"reply_to_id"`

	IsEdited  bool       `json:"is_edited" db:"is_edited"`
	DeletedAt *time.Time `json:"deleted_at" db:"deleted_at"` // Soft delete
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// DTOs for Service/Repo communication
type ChannelSummary struct {
	Channel
	UnreadCount int         `json:"unread_count"`
	LastMessage *MessageDTO `json:"last_message"` // Enriched with Sender Name
	Members     []uuid.UUID `json:"member_ids"`   // IDs of participants (for Direct chats)
}

// MessageDTO enriches the raw message with Sender info for API responses
type MessageDTO struct {
	Message
	SenderName   string `json:"sender_name"`
	SenderAvatar string `json:"sender_avatar"`
}
