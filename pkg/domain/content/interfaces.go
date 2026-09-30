// internal/domain/content/interfaces.go
package content

import (
	"context"

	"github.com/google/uuid"
)

// --- High-Level Application Service Interfaces (Inbound Ports) ---

// Service defines the contract for managing content entries.
// This is the primary API for content editors and delivery frontends.
type Service interface {
	// Entry Management
	CreateEntry(ctx context.Context, params CreateEntryParams) (*Entry, error)
	GetEntry(ctx context.Context, entryID uuid.UUID) (*Entry, error)
	ListEntries(ctx context.Context, modelID uuid.UUID, filter EntryFilter) ([]*Entry, int, error)
	ListEntriesByModelKey(ctx context.Context, modelKey string, filter EntryFilter) ([]*Entry, int, error)
	UpdateEntry(ctx context.Context, entryID uuid.UUID, params UpdateEntryParams) (*Entry, error)
	ArchiveEntry(ctx context.Context, entryID uuid.UUID) error
	DeleteEntry(ctx context.Context, entryID uuid.UUID) error

	// Publishing & Versioning
	PublishEntry(ctx context.Context, entryID uuid.UUID) error
	UnpublishEntry(ctx context.Context, entryID uuid.UUID) error
	ListVersions(ctx context.Context, entryID uuid.UUID) ([]*Version, error)
	RestoreVersion(ctx context.Context, entryID, versionID uuid.UUID) (*Entry, error)
}

// AdminService defines the contract for administrative operations on content structure.
// This is used to manage schemas (Models) and containers (Spaces).
type AdminService interface {
	// Space Management
	CreateSpace(ctx context.Context, params CreateSpaceParams) (*Space, error)
	GetSpace(ctx context.Context, spaceID uuid.UUID) (*Space, error)
	ListSpaces(ctx context.Context) ([]*Space, error)
	UpdateSpace(ctx context.Context, spaceID uuid.UUID, params UpdateSpaceParams) (*Space, error)
	DeleteSpace(ctx context.Context, spaceID uuid.UUID) error
	UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, params UpdateEntryStatusParams) (*Entry, error)
	// Model (Schema) Management
	CreateModel(ctx context.Context, params CreateModelParams) (*Model, error)
	GetModel(ctx context.Context, modelID uuid.UUID) (*Model, error)
	ListModels(ctx context.Context, spaceID uuid.UUID) ([]*Model, error)
	UpdateModel(ctx context.Context, modelID uuid.UUID, params UpdateModelParams) (*Model, error)
	DeleteModel(ctx context.Context, modelID uuid.UUID) error

	GetModelByAPIKey(ctx context.Context, apiKey string) (*Model, error)
}

// --- DTOs for Service Methods ---

type CreateEntryParams struct {
	ModelID uuid.UUID                 `validate:"required"`
	SpaceID uuid.UUID                 `validate:"required"`
	Fields  map[string]map[string]any `validate:"required"` // locale -> { fieldName -> value }
}

type UpdateEntryParams struct {
	Fields  map[string]map[string]any `validate:"required"`
	Version int                       `validate:"required"` // For optimistic locking
}

type EntryFilter struct {
	Status *EntryStatus
	Limit  int
	Offset int
}

type CreateSpaceParams struct {
	Key           string   `validate:"required,slug"`
	Name          string   `validate:"required"`
	DefaultLocale string   `validate:"required"`
	Locales       []string `validate:"required,min=1"`
}

type UpdateSpaceParams struct {
	Name          *string
	DefaultLocale *string
	Locales       []string
}

type CreateModelParams struct {
	SpaceID uuid.UUID `validate:"required"`
	APIKey  string    `validate:"required,slug"`
	Name    string    `validate:"required"`
	Fields  []Field   `validate:"required,min=1,dive"`
}

type UpdateModelParams struct {
	Name   *string
	APIKey *string `validate:"omitempty,slug"`
	Fields []Field `validate:"omitempty,min=1,dive"`
}

// --- System Components & Infrastructure ---

// Validator defines the contract for validating an entry's fields against its model's schema.
type Validator interface {
	Validate(ctx context.Context, fields map[string]map[string]any, model *Model) error
}
