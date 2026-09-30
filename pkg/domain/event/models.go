// internal/domain/event/models.go
package event

import (
	"time"

	"github.com/combi/pkg/domain/financial"
	"github.com/google/uuid"
)

// --- Enums ---

type EventStatus string

const (
	StatusDraft     EventStatus = "DRAFT"
	StatusPublished EventStatus = "PUBLISHED"
	StatusCompleted EventStatus = "COMPLETED"
	StatusCancelled EventStatus = "CANCELLED"
)

type EventFormat string

const (
	FormatPhysical EventFormat = "PHYSICAL"
	FormatVirtual  EventFormat = "VIRTUAL"
	FormatHybrid   EventFormat = "HYBRID"
)

type RegistrationStatus string

const (
	RegStatusConfirmed RegistrationStatus = "CONFIRMED"
	RegStatusCancelled RegistrationStatus = "CANCELLED"
	RegStatusAttended  RegistrationStatus = "ATTENDED"
)

// --- Core Models ---

// Venue represents a physical location where an event can be held.
type Venue struct {
	ID        uuid.UUID
	Name      string
	Address   string
	City      string
	Country   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Event is the central entity for a time-based gathering.
type Event struct {
	ID             uuid.UUID
	OrganizationID *uuid.UUID // Optional link to an organization
	Name           string
	Slug           string `gorm:"uniqueIndex"`
	Description    string
	Status         EventStatus
	Format         EventFormat
	StartTime      time.Time
	EndTime        time.Time
	Timezone       string // e.g., "America/New_York"
	VenueID        *uuid.UUID
	StreamURL      *string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Properties     map[string]interface{} `gorm:"type:jsonb"` // For flexible business rules
}

// Ticket defines an option for attending an event.
type Ticket struct {
	ID                 uuid.UUID
	EventID            uuid.UUID
	Name               string          // e.g., "General Admission", "VIP"
	Price              financial.Price `gorm:"embedded;embeddedPrefix:price_"`
	Capacity           *int            // Null for unlimited.
	RegistrationsCount int
	SaleStartsAt       *time.Time
	SaleEndsAt         *time.Time
	IsActive           bool
	CreatedAt          time.Time
}

func (t *Ticket) IsAvailable() bool {
	if !t.IsActive {
		return false
	}
	now := time.Now().UTC()
	if t.SaleStartsAt != nil && now.Before(*t.SaleStartsAt) {
		return false
	}
	if t.SaleEndsAt != nil && now.After(*t.SaleEndsAt) {
		return false
	}
	if t.Capacity != nil && t.RegistrationsCount >= *t.Capacity {
		return false
	}
	return true
}

// Registration represents a user's attendance for an event.
type Registration struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	EventID       uuid.UUID
	TicketID      uuid.UUID
	Status        RegistrationStatus
	TransactionID *uuid.UUID // Links to financial transaction if the ticket was paid.
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CheckedInAt   *time.Time
}

// Speaker represents a presenter at an event.
type Speaker struct {
	ID        uuid.UUID
	Name      string
	Title     string
	Bio       *string
	AvatarURL *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ScheduleItem represents a single session or talk within an event's agenda.
type ScheduleItem struct {
	ID          uuid.UUID
	EventID     uuid.UUID
	Title       string
	Description *string
	StartTime   time.Time
	EndTime     time.Time
	SpeakerIDs  []uuid.UUID `gorm:"type:jsonb"`
	Track       *string     // e.g., "Developer Track"
	CreatedAt   time.Time
}
