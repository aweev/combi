package matchmaking

import "github.com/google/uuid"

// const (
// 	// Emitted when the algorithm finds high-quality matches.
// 	// Consumers: NotificationService (email user: "We found potential mentors!").
// 	EventSuggestionsReady core.EventAction = "match.suggestions.ready"

// 	// Emitted when a match is officially active.
// 	// Consumers: ChatService (create a private room), ProjectService (create a collab space).
// 	EventMatchEstablished core.EventAction = "match.established"
// 	EventMatchRequest     core.EventAction = "match.request"
// 	// Emitted when a match ends.
// 	// Consumers: FormService (send feedback survey).
// 	EventMatchEnded core.EventAction = "match.ended"
// )

type MatchEstablishedPayload struct {
	MatchID   uuid.UUID `json:"match_id"`
	MentorID  uuid.UUID `json:"mentor_id"`
	MenteeID  uuid.UUID `json:"mentee_id"`
	RequestID uuid.UUID `json:"request_id"`
}
