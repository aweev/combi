// internal/domain/content/repository.go
package content

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the master persistence interface for the content domain.
type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	SpaceRepository
	ModelRepository
	EntryRepository
	VersionRepository
}

type SpaceRepository interface {
	CreateSpace(ctx context.Context, space *Space) error
	GetSpace(ctx context.Context, id uuid.UUID) (*Space, error)
	ListSpaces(ctx context.Context) ([]*Space, error)
	UpdateSpace(ctx context.Context, space *Space) error
	DeleteSpace(ctx context.Context, id uuid.UUID) error
}

type ModelRepository interface {
	CreateModel(ctx context.Context, model *Model) error
	GetModel(ctx context.Context, id uuid.UUID) (*Model, error)
	ListModels(ctx context.Context, spaceID uuid.UUID) ([]*Model, error)
	UpdateModel(ctx context.Context, model *Model) error
	DeleteModel(ctx context.Context, id uuid.UUID) error
	IsModelInUse(ctx context.Context, id uuid.UUID) (bool, error)
	GetModelByAPIKey(ctx context.Context, apiKey string) (*Model, error)
}

type EntryRepository interface {
	CreateEntry(ctx context.Context, entry *Entry) error
	GetEntry(ctx context.Context, id uuid.UUID) (*Entry, error)
	ListEntries(ctx context.Context, modelID uuid.UUID, filter EntryFilter) ([]*Entry, int, error)
	UpdateEntry(ctx context.Context, entry *Entry) error
	DeleteEntry(ctx context.Context, id uuid.UUID) error
}

type VersionRepository interface {
	CreateVersion(ctx context.Context, version *Version) error
	GetVersion(ctx context.Context, id uuid.UUID) (*Version, error)
	GetLatestVersionForEntry(ctx context.Context, entryID uuid.UUID) (*Version, error)
	ListVersionsForEntry(ctx context.Context, entryID uuid.UUID) ([]*Version, error)
}
