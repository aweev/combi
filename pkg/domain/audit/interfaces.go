package audit

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// EventRecorder is the simple, "write-only" interface for recording events.
// This is the primary dependency for all other domains that need to log actions.
type EventRecorder interface {
	Record(ctx context.Context, event Event) error
}

type Service interface {
	GetEvent(ctx context.Context, eventID uuid.UUID) (*Event, error)
	SearchEvents(ctx context.Context, filters EventFilters) ([]*Event, int, error)
	ExportEvents(ctx context.Context, filters EventFilters, format string) (*queue.Job, error)
}

type LogFilter struct {
	ActorID  *uuid.UUID
	TargetID *uuid.UUID
	Action   *core.EventAction
	From     *time.Time
	To       *time.Time
}

// ComplianceService defines the contract for managing legal holds and data retention.
type ComplianceService interface {
	ApplyLegalHold(ctx context.Context, params ApplyLegalHoldParams) (*LegalHold, error)
	ReleaseLegalHold(ctx context.Context, holdID uuid.UUID) (*LegalHold, error)
	ListLegalHolds(ctx context.Context, limit, offset int) ([]*LegalHold, int, error)

	DefineRetentionPolicy(ctx context.Context, params DefineRetentionPolicyParams) (*RetentionPolicy, error)
	GetRetentionPolicy(ctx context.Context, policyID uuid.UUID) (*RetentionPolicy, error)
	ListRetentionPolicies(ctx context.Context, limit, offset int) ([]*RetentionPolicy, int, error)

	// AnonymizeUser triggers a background job to perform a "Right to be Forgotten" request.
	AnonymizeUser(ctx context.Context, userID uuid.UUID) (*queue.Job, error)
	UpdateRetentionPolicy(ctx context.Context, id uuid.UUID, params DefineRetentionPolicyParams) (*RetentionPolicy, error)
	DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error
}

// --- DTOs for Service Methods ---

type EventFilters struct {
	ActorID   *uuid.UUID
	TargetID  *uuid.UUID
	Action    *core.EventAction
	Outcome   *Outcome
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
	SortBy    string // e.g., "created_at"
	SortOrder string // "asc" or "desc"
}

type ApplyLegalHoldParams struct {
	Name        string `validate:"required,min=3"`
	Description string `validate:"required,max=500"`
	HoldSubject string `validate:"required"` // e.g., "user_id:uuid-goes-here"
}

type DefineRetentionPolicyParams struct {
	Name        string        `validate:"required,min=3"`
	Description string        `validate:"required,max=500"`
	DataType    string        `validate:"required"` // e.g., "auth.*"
	Duration    time.Duration `validate:"required,gt=0"`
	IsActive    bool
}

// --- System Components & Initialization ---

// EventDefinition is a DTO used to register an event's metadata.
type EventDefinition struct {
	Action      core.EventAction
	Description string
	Category    string
}

// EventRegistrar collects all event definitions from across the application at startup.
type EventRegistrar interface {
	Register(def EventDefinition)
	GetAllDefinitions() []EventDefinition
}
