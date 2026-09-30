package user

import (
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/google/uuid"
)

type UserStatus string

const (
	StatusPending     UserStatus = "PENDING"
	StatusActive      UserStatus = "ACTIVE"
	StatusDeactivated UserStatus = "DEACTIVATED"
	StatusAnonymized  UserStatus = "ANONYMIZED"
)

type ConnectionType string

const (
	ConnFollowing ConnectionType = "FOLLOWING"
	ConnBlocking  ConnectionType = "BLOCKING"
)

// --- Core Models ---

type User struct {
	ID        uuid.UUID  `db:"id"`
	FirstName string     `db:"first_name"`
	LastName  string     `db:"last_name"`
	Email     core.Email `db:"email"`
	Status    UserStatus `db:"status"`
	Locale    *string    `db:"locale"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

type Connection struct {
	ID           uuid.UUID      `db:"id"`
	SourceUserID uuid.UUID      `db:"source_user_id" gorm:"uniqueIndex:idx_connection_unique"`
	TargetUserID uuid.UUID      `db:"target_user_id" gorm:"uniqueIndex:idx_connection_unique"`
	Type         ConnectionType `db:"type" gorm:"uniqueIndex:idx_connection_unique"`
	CreatedAt    time.Time      `db:"created_at"`
}

type PublicProfile struct {
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	// Email        string     `json:"email" db:"email"`
	DisplayName  *string    `json:"display_name" db:"display_name"`
	AvatarFileID *uuid.UUID `json:"-" db:"avatar_file_id"`
	AvatarURL    *string    `json:"avatar_url"`
}

func (u *User) FullName() string {
	return fmt.Sprintf("%s %s", u.FirstName, u.LastName)
}
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

type ContactInfoDTO struct {
	ID    uuid.UUID
	Email core.Email
}

// Profile is the rich data associated with an Identity.
type Profile struct {
	ID uuid.UUID `json:"id" db:"id"` // Matches Auth.Identity.ID

	// Public Info
	Handle       string     `json:"handle" db:"handle"` // @username
	DisplayName  string     `json:"display_name" db:"display_name"`
	AvatarFileID *uuid.UUID `json:"avatar_file_id" db:"avatar_file_id"`

	// Sensitive PII (Encrypted at Rest)
	// Struct: { FirstName, LastName, Phone, Address }
	//PrivateData security.SecureData `json:"-" db:"private_data"`

	Bio *string `json:"bio" db:"bio"`

	// Preferences (JSONB)
	Preferences UserPreferences `json:"preferences" db:"preferences"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	CustomFields map[string]interface{} `json:"custom_fields" db:"custom_fields"`
}

type UserPreferences struct {
	Locale      string `json:"locale"`   // "en-US"
	Theme       string `json:"theme"`    // "dark"
	Timezone    string `json:"timezone"` // "Africa/Nairobi"
	EmailDigest bool   `json:"email_digest"`
}

// PersonalData is the unencrypted DTO for the PrivateData field.
type PersonalData struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
}
