// internal/domain/event/repository.go
package event

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	EventRepository
	TicketRepository
	RegistrationRepository
	VenueRepository
	SpeakerRepository
	ScheduleItemRepository
}

type EventRepository interface {
	CreateEvent(ctx context.Context, event *Event) error
	GetEvent(ctx context.Context, id uuid.UUID) (*Event, error)
	GetEventBySlug(ctx context.Context, slug string) (*Event, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*Event, int, error)
	UpdateEvent(ctx context.Context, event *Event) error
	DeleteEvent(ctx context.Context, id uuid.UUID) error
}

type TicketRepository interface {
	CreateTicket(ctx context.Context, ticket *Ticket) error
	GetTicket(ctx context.Context, id uuid.UUID) (*Ticket, error)
	ListTicketsForEvent(ctx context.Context, eventID uuid.UUID) ([]*Ticket, error)
	UpdateTicket(ctx context.Context, ticket *Ticket) error
	DeleteTicket(ctx context.Context, id uuid.UUID) error
	IncrementRegistrationsCount(ctx context.Context, ticketID uuid.UUID, amount int) error
}

type RegistrationRepository interface {
	CreateRegistration(ctx context.Context, reg *Registration) error
	GetRegistration(ctx context.Context, id uuid.UUID) (*Registration, error)
	GetUserRegistrationForEvent(ctx context.Context, userID, eventID uuid.UUID) (*Registration, error)
	ListRegistrationsForEvent(ctx context.Context, eventID uuid.UUID, filter RegistrationFilter) ([]*Registration, int, error)
	ListRegistrationsForUser(ctx context.Context, actorID uuid.UUID, filter RegistrationFilter) ([]*Registration, int, error)
	UpdateRegistration(ctx context.Context, reg *Registration) error
}

type VenueRepository interface {
	CreateVenue(ctx context.Context, venue *Venue) error
	GetVenue(ctx context.Context, id uuid.UUID) (*Venue, error)
	ListVenues(ctx context.Context) ([]*Venue, error)
	UpdateVenue(ctx context.Context, venue *Venue) error
	DeleteVenue(ctx context.Context, id uuid.UUID) error
}

type SpeakerRepository interface {
	CreateSpeaker(ctx context.Context, speaker *Speaker) error
	GetSpeaker(ctx context.Context, id uuid.UUID) (*Speaker, error)
	ListSpeakers(ctx context.Context) ([]*Speaker, error)
	UpdateSpeaker(ctx context.Context, speaker *Speaker) error
	DeleteSpeaker(ctx context.Context, id uuid.UUID) error
}

type ScheduleItemRepository interface {
	CreateScheduleItem(ctx context.Context, item *ScheduleItem) error
	GetScheduleItem(ctx context.Context, id uuid.UUID) (*ScheduleItem, error) // <-- ADD THIS LINE
	ListScheduleItemsForEvent(ctx context.Context, eventID uuid.UUID) ([]*ScheduleItem, error)
	UpdateScheduleItem(ctx context.Context, item *ScheduleItem) error
	DeleteScheduleItem(ctx context.Context, id uuid.UUID) error
}
