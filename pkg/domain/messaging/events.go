package messaging

import (
	"time"

	"github.com/google/uuid"
)

// const (
// 	// Emitted when a new message is sent.
// 	// Consumers: RealtimeService (push to websocket), NotificationService (push notification if offline).
// 	EventMessageCreated core.EventAction = "messaging.message.created"

// 	// Emitted when a user reads a message.
// 	// Consumers: RealtimeService (update read receipts).
// 	EventMessageRead core.EventAction = "messaging.message.read"

// 	// Emitted when a message is flagged.
// 	// Consumers: NotificationService (alert moderators).
// 	EventMessageFlagged core.EventAction = "messaging.message.flagged"
// )

type MessageCreatedPayload struct {
	MessageID uuid.UUID `json:"message_id"`
	RoomID    uuid.UUID `json:"room_id"`
	SenderID  uuid.UUID `json:"sender_id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // text, image, etc.
	CreatedAt time.Time `json:"created_at"`
}
