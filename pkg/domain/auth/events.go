package auth

import (
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

// UserCreatedPayload (for "user.created" / "auth.account.created")
type UserCreatedPayload struct {
	UserID           uuid.UUID  `json:"user_id"`
	Email            core.Email `json:"email"`
	Name             string     `json:"name"` // Populated if available (e.g. OAuth)
	VerificationLink string     `json:"verification_link,omitempty"`
}

// PasswordResetRequestedPayload
type PasswordResetRequestedPayload struct {
	UserID    uuid.UUID  `json:"user_id"`
	Email     core.Email `json:"email"`
	Name      string     `json:"name"`
	ResetLink string     `json:"reset_link"`
}

// MagicLinkRequestedPayload
type MagicLinkRequestedPayload struct {
	UserID    uuid.UUID  `json:"user_id"`
	Email     core.Email `json:"email"`
	LoginLink string     `json:"login_link"`
	IPAddress string     `json:"ip_address"`
	UserAgent string     `json:"user_agent"`
}

// LoginPayload (for "auth.user.loggedin")
type LoginPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	Method    string    `json:"method"` // "password", "google", "magic_link"
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Time      time.Time `json:"time"`
}
