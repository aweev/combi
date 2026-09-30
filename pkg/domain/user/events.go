package user

import (
	"time"

	"github.com/google/uuid"
)

// UserCreatedPayload is emitted when a new account is registered.
type UserCreatedPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"` // "volunteer", "admin", etc.
	CreatedAt time.Time `json:"created_at"`
}

// ProfileUpdatedPayload is emitted when profile details change.
// Used heavily by Search (indexing) and Matchmaking (skills).
type ProfileUpdatedPayload struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName *string   `json:"display_name,omitempty"`
	Bio         *string   `json:"bio,omitempty"`
	Skills      []string  `json:"skills,omitempty"`
	Interests   []string  `json:"interests,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

type ConnectionCreatedPayload struct {
	SourceUserID uuid.UUID `json:"source_user_id"`
	TargetUserID uuid.UUID `json:"target_user_id"`
	Type         string    `json:"type"` // "FOLLOWING", etc.
}
