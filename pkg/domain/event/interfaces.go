// internal/domain/event/interfaces.go
package event

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// Service defines the core contract for managing events and attendance.
type Service interface {
	// Event Discovery
	GetEvent(ctx context.Context, eventID uuid.UUID) (*Event, error)
	GetEventBySlug(ctx context.Context, slug string) (*Event, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*Event, int, error)
	ListTicketsForEvent(ctx context.Context, eventID uuid.UUID) ([]*Ticket, error)
	ListScheduleForEvent(ctx context.Context, eventID uuid.UUID) ([]*ScheduleItem, error)

	// Registration Flow
	RegisterForEvent(ctx context.Context, params RegisterParams) (*Registration, error)
	WithdrawRegistration(ctx context.Context, registrationID uuid.UUID) (*Registration, error)
	GetUserRegistration(ctx context.Context, userID, eventID uuid.UUID) (*Registration, error)
}

// AdminService defines the contract for creating and managing event content and logistics.
type AdminService interface {
	// Event Management
	CreateEvent(ctx context.Context, params CreateEventParams) (*Event, error)
	UpdateEvent(ctx context.Context, eventID uuid.UUID, params UpdateEventParams) (*Event, error)
	CancelEvent(ctx context.Context, eventID uuid.UUID, reason string) (*Event, error)
	DeleteEvent(ctx context.Context, eventID uuid.UUID) error

	// Event Content Management
	CreateTicket(ctx context.Context, eventID uuid.UUID, params CreateTicketParams) (*Ticket, error)
	UpdateTicket(ctx context.Context, ticketID uuid.UUID, params UpdateTicketParams) (*Ticket, error)
	AddScheduleItem(ctx context.Context, eventID uuid.UUID, params CreateScheduleItemParams) (*ScheduleItem, error)
	UpdateScheduleItem(ctx context.Context, itemID uuid.UUID, params UpdateScheduleItemParams) (*ScheduleItem, error)

	// Logistics & Attendee Management
	ListRegistrationsForEvent(ctx context.Context, eventID uuid.UUID, filter RegistrationFilter) ([]*Registration, int, error)
	CheckInUser(ctx context.Context, registrationID uuid.UUID) (*Registration, error)

	// Resource Management (Venues, Speakers)
	CreateVenue(ctx context.Context, params CreateVenueParams) (*Venue, error)
	CreateSpeaker(ctx context.Context, params CreateSpeakerParams) (*Speaker, error)
}

// --- DTOs for Service Methods ---

type CreateEventParams struct {
	CreatedBy      uuid.UUID `validate:"required"`
	OrganizationID *uuid.UUID
	Name           string      `validate:"required"`
	Slug           string      `validate:"required,slug"`
	Description    string      `validate:"required"`
	Format         EventFormat `validate:"required"`
	StartTime      time.Time   `validate:"required"`
	EndTime        time.Time   `validate:"required,gtefield=StartTime"`
	Timezone       string      `validate:"required"`
	VenueID        *uuid.UUID
}

type UpdateEventParams struct {
	Name        *string
	Description *string
	StartTime   *time.Time
	EndTime     *time.Time
	Status      *EventStatus
	VenueID     *uuid.UUID
	StreamURL   *string
}

type EventFilter struct {
	Status     *EventStatus
	Format     *EventFormat
	IsUpcoming bool
	Search     string
	Limit      int
	Offset     int
}

type CreateTicketParams struct {
	Name         string `validate:"required"`
	Amount       int64  // Price in cents
	Currency     string `validate:"iso4217"`
	Capacity     *int   `validate:"omitempty,min=1"`
	SaleStartsAt *time.Time
	SaleEndsAt   *time.Time
	IsActive     bool
}

type UpdateTicketParams struct {
	Name         *string
	Amount       *int64
	Capacity     *int
	SaleStartsAt *time.Time
	SaleEndsAt   *time.Time
	IsActive     *bool
}

type RegisterParams struct {
	UserID        uuid.UUID `validate:"required"`
	EventID       uuid.UUID `validate:"required"`
	TicketID      uuid.UUID `validate:"required"`
	ProviderToken *string   // For paid tickets with a new card
}

type CreateScheduleItemParams struct {
	Title       string `validate:"required"`
	Description *string
	StartTime   time.Time `validate:"required"`
	EndTime     time.Time `validate:"required"`
	SpeakerIDs  []uuid.UUID
	Track       *string
}

type CreateVenueParams struct {
	Name    string `validate:"required"`
	Address string `validate:"required"`
	City    string `validate:"required"`
	Country string `validate:"required"`
}

type CreateSpeakerParams struct {
	Name      string `validate:"required"`
	Title     string `validate:"required"`
	Bio       *string
	AvatarURL *string `validate:"omitempty,url"`
}

type RegistrationFilter struct {
	Status *RegistrationStatus
	Limit  int
	Offset int
}

type UpdateScheduleItemParams struct {
	Title       *string
	Description *string
	StartTime   *time.Time
	EndTime     *time.Time
	SpeakerIDs  []uuid.UUID // Note: Slices are reference types, so a pointer isn't needed for partial updates. Send the full new list.
	Track       *string
}
