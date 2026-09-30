package system

import "github.com/google/uuid"

// This package defines canonical identities and constants for system-level actors and operations.

const (
	// const is the designated type for the system actor in audit logs.
	SystemActorType = "system"
)

// SystemActorID is a predefined, stable UUID for actions performed by the system
// (e.g., in a background job or workflow), rather than by a specific user.
var SystemActorID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
