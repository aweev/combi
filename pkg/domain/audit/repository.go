// internal/domain/audit/repository.go
package audit

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the master persistence interface for the audit domain.
// It composes smaller, more focused repository interfaces and provides transaction management.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	EventRepository
	LegalHoldRepository
	RetentionPolicyRepository
}

type EventRepository interface {
	CreateEvent(ctx context.Context, event *Event) error
	CreateBatchEvents(ctx context.Context, events []*Event) error
	GetEvent(ctx context.Context, eventID uuid.UUID) (*Event, error)
	SearchEvents(ctx context.Context, filters EventFilters) ([]*Event, int, error)
}

type LegalHoldRepository interface {
	CreateLegalHold(ctx context.Context, hold *LegalHold) error
	GetLegalHold(ctx context.Context, id uuid.UUID) (*LegalHold, error)
	UpdateLegalHold(ctx context.Context, hold *LegalHold) error
	ListLegalHolds(ctx context.Context, limit, offset int) ([]*LegalHold, int, error)
}

type RetentionPolicyRepository interface {
	CreateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	GetRetentionPolicy(ctx context.Context, id uuid.UUID) (*RetentionPolicy, error)
	UpdateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	DeleteRetentionPolicy(ctx context.Context, id uuid.UUID) error
	ListRetentionPolicies(ctx context.Context, limit, offset int) ([]*RetentionPolicy, int, error)
}
