// internal/domain/event/errors.go
package event

import "errors"

var (
	// --- Event Errors ---
	ErrEventNotFound       = errors.New("event not found")
	ErrEventAlreadyExists  = errors.New("an event with this slug already exists")
	ErrEventInThePast      = errors.New("cannot create or modify an event to be in the past")
	ErrEventIsCancelled    = errors.New("event has been cancelled")
	ErrEventIsNotLive      = errors.New("event is not currently live")
	ErrInvalidTimeRange    = errors.New("event start time must be before the end time")
	ErrEventUpdateConflict = errors.New("event has been updated by someone else; please refresh")

	// --- Registration & Attendance Errors ---
	ErrRegistrationNotFound     = errors.New("registration not found for this user and event")
	ErrRegistrationClosed       = errors.New("registration for this event is closed")
	ErrRegistrationNotOpenYet   = errors.New("registration for this event has not opened yet")
	ErrAlreadyRegistered        = errors.New("user is already registered for this event")
	ErrEventIsFull              = errors.New("event has reached its maximum capacity")
	ErrTicketNotFound           = errors.New("ticket type not found for this event")
	ErrTicketNotAvailable       = errors.New("the selected ticket type is not available")
	ErrWithdrawalDeadlinePassed = errors.New("the deadline to withdraw from the event has passed")

	// --- Venue & Location Errors ---
	ErrVenueNotFound   = errors.New("venue not found")
	ErrVenueInUse      = errors.New("venue is in use by other events and cannot be deleted")
	ErrInvalidLocation = errors.New("invalid location data provided")

	// --- Access & Streaming Errors ---
	ErrAccessDenied          = errors.New("user does not have access to this event or its resources")
	ErrStreamURLNotAvailable = errors.New("the stream URL is not yet available for this event")

	// --- Speaker & Schedule Errors ---
	ErrSpeakerNotFound  = errors.New("speaker not found")
	ErrSessionNotFound  = errors.New("schedule session not found")
	ErrTimeSlotConflict = errors.New("the proposed session time conflicts with an existing session")

	// --- General Errors ---
	ErrInvalidInput = errors.New("invalid input")
)
