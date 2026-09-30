package notification

import (
	"time"

	"github.com/google/uuid"
)

type EmailBouncedPayload struct {
	NotificationID    uuid.UUID `json:"notification_id"`
	RecipientEmail    string    `json:"recipient_email"`
	Provider          string    `json:"provider"`
	ProviderMessageID string    `json:"provider_message_id"`
	BounceType        string    `json:"bounce_type"`
	Reason            string    `json:"reason"`
	BouncedAt         time.Time `json:"bounced_at"`
	Permanent         bool      `json:"permanent"`
}

type SMSFailedPayload struct {
	NotificationID    uuid.UUID `json:"notification_id"`
	Provider          string    `json:"provider"`
	ProviderMessageID string    `json:"provider_message_id"`
	RecipientNumber   string    `json:"recipient_number"`
	ErrorCode         string    `json:"error_code"`
	ErrorMessage      string    `json:"error_message"`
	FailedAt          time.Time `json:"failed_at"`
}
