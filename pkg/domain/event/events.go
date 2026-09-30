// internal/domain/event/events.go
package event

import (
	"time"

	"github.com/google/uuid"
)

// EventCreatedPayload (for "event.created")
type EventCreatedPayload struct {
	EventID   uuid.UUID `json:"event_id"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	Format    string    `json:"format"` // "ONLINE", "IN_PERSON"
}

// SessionEndedPayload (for "event.session.ended")
// Consumed by: Impact Service (Lives Touched)
type SessionEndedPayload struct {
	EventID       uuid.UUID `json:"event_id"`
	Title         string    `json:"title"`
	AttendeeCount int       `json:"attendee_count"`
	DurationMin   int       `json:"duration_minutes"`
	EndedAt       time.Time `json:"ended_at"`
}

// AttendeeJoinedPayload (for "event.attendee.joined")
// Consumed by: Gamification (Badge), Automation (Welcome Email)
type AttendeeJoinedPayload struct {
	EventID        uuid.UUID `json:"event_id"`
	UserID         uuid.UUID `json:"user_id"`
	RegistrationID uuid.UUID `json:"registration_id"`
	TicketType     string    `json:"ticket_type"`
	JoinedAt       time.Time `json:"joined_at"`
	IsCheckIn      bool      `json:"is_check_in"` // True if physical check-in, False if just registration
}
