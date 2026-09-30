package tagging

import (
	"context"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 400 Bad Request
	ErrInvalidTagType     = apperror.New(http.StatusBadRequest, "INVALID_TAG_TYPE", "The specified tag type is not valid.", nil)
	ErrCannotMergeSameTag = apperror.New(http.StatusBadRequest, "CANNOT_MERGE_SAME_TAG", "Source and target tags must be different.", nil)

	ErrAccessDenied = apperror.New(http.StatusForbidden, "ACCESS_DENIED", "You do not have permission to access this resource.", nil)
	ErrInvalidInput = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "The provided input is not valid.", nil)

	// 404 Not Found
	ErrTagNotFound = apperror.New(http.StatusNotFound, "TAG_NOT_FOUND", "The specified tag could not be found.", nil)

	// 409 Conflict
	ErrTagAlreadyExists = apperror.New(http.StatusConflict, "TAG_ALREADY_EXISTS", "A tag with this name already exists in this category.", nil)
)

const (
	// PermTagManage allows creating, editing, and merging tags.
	PermTagManage core.PermissionName = "tagging:manage"
)

type TagType string

const (
	TypeSkill    TagType = "SKILL"    // e.g., "Python", "Project Management"
	TypeInterest TagType = "INTEREST" // e.g., "Sustainability", "Mentorship"
	TypeTopic    TagType = "TOPIC"    // e.g., "Health", "Education" (for Content/Events)
	TypeAudience TagType = "AUDIENCE" // e.g., "Youth", "Refugees"
	TypeSystem   TagType = "SYSTEM"   // Internal tags, hidden from UI
)

// --- Core Models ---

// Tag represents a reusable label in the system.
type Tag struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"` // Display name, e.g. "C++"
	Slug        string    `json:"slug" db:"slug"` // Normalized, e.g. "c-plus-plus"
	Type        TagType   `json:"type" db:"type"`
	Description string    `json:"description" db:"description"`
	Color       string    `json:"color" db:"color"`             // Hex code for UI chips
	Count       int       `json:"count" db:"count"`             // Denormalized usage count
	IsApproved  bool      `json:"is_approved" db:"is_approved"` // For user-submitted tags
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Tagging links a Tag to any entity in the system.
// This implements a polymorphic relationship pattern.
type Tagging struct {
	ID         uuid.UUID `json:"id" db:"id"`
	TagID      uuid.UUID `json:"tag_id" db:"tag_id"`
	EntityType string    `json:"entity_type" db:"entity_type"` // e.g., "user", "event", "content"
	EntityID   uuid.UUID `json:"entity_id" db:"entity_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`

	// Join fields (populated on read)
	Tag *Tag `json:"tag,omitempty" db:"-"`
}

// Service is the primary interface used by other domains.
type Service interface {
	// Tag Management
	CreateTag(ctx context.Context, name string, tType TagType) (*Tag, error)
	GetTag(ctx context.Context, id uuid.UUID) (*Tag, error)
	SearchTags(ctx context.Context, query string, tType *TagType, limit int) ([]Tag, error)
	MergeTags(ctx context.Context, sourceID, targetID uuid.UUID) error // Admin tool to cleanup dupes

	// Entity Tagging (e.g., "Add 'Go' to User X")
	TagEntity(ctx context.Context, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID) error
	UntagEntity(ctx context.Context, entityType string, entityID uuid.UUID, tagIDs []uuid.UUID) error

	// Retrieval
	GetTagsForEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]Tag, error)
	GetEntitiesByTag(ctx context.Context, tagSlug string, entityType string, limit, offset int) ([]uuid.UUID, int, error)
}

// AdminService exposes advanced management features.
type AdminService interface {
	MergeTags(ctx context.Context, sourceID, targetID uuid.UUID) error
	DeleteTag(ctx context.Context, id uuid.UUID) error
	UpdateTag(ctx context.Context, id uuid.UUID, name, description, color string) (*Tag, error)
}

type Repository interface {
	// Transaction support
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	// Tags
	CreateTag(ctx context.Context, tag *Tag) error
	GetTagByID(ctx context.Context, id uuid.UUID) (*Tag, error)
	GetTagBySlug(ctx context.Context, slug string, tType TagType) (*Tag, error)
	SearchTags(ctx context.Context, query string, tType *TagType, limit int) ([]Tag, error)
	UpdateTag(ctx context.Context, tag *Tag) error
	DeleteTag(ctx context.Context, id uuid.UUID) error

	// Taggings
	// AddTagging links an entity. Returns true if created, false if already existed.
	AddTagging(ctx context.Context, tagging *Tagging) (bool, error)
	RemoveTagging(ctx context.Context, tagID uuid.UUID, entityType string, entityID uuid.UUID) error

	// GetTagsForEntity returns the full Tag objects associated with an entity
	GetTagsForEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]Tag, error)

	// GetEntityIDsByTag returns UUIDs of entities (e.g. all UserIDs tagged "Go")
	GetEntityIDsByTag(ctx context.Context, tagID uuid.UUID, entityType string, limit, offset int) ([]uuid.UUID, int, error)

	// Batch Ops
	UpdateTagCount(ctx context.Context, tagID uuid.UUID, delta int) error

	// ReassignTaggings moves all links from oldTagID to newTagID.
	// Used for merging tags.
	ReassignTaggings(ctx context.Context, oldTagID, newTagID uuid.UUID) error
}
