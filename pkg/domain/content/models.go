// internal/domain/content/models.go
package content

import (
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type FieldType string

const (
	FieldTypeText      FieldType = "TEXT"      // Single-line text
	FieldTypeRichText  FieldType = "RICH_TEXT" // Multi-line, formatted text (e.g., Markdown, JSON)
	FieldTypeNumber    FieldType = "NUMBER"    // Integer or float
	FieldTypeBoolean   FieldType = "BOOLEAN"
	FieldTypeDateTime  FieldType = "DATE_TIME"
	FieldTypeAsset     FieldType = "ASSET"     // Link to a file in the StorageService
	FieldTypeReference FieldType = "REFERENCE" // Link to another Entry
	FieldTypeJSON      FieldType = "JSON"      // Arbitrary JSON object
)

type EntryStatus string

const (
	StatusDraft     EntryStatus = "DRAFT"     // Has unpublished changes.
	StatusPublished EntryStatus = "PUBLISHED" // A version is live.
	StatusArchived  EntryStatus = "ARCHIVED"  // Hidden from delivery APIs, but not deleted.
	StatusChanged   EntryStatus = "CHANGED"   // Published, but also has a newer draft.
	StatusRejected  EntryStatus = "REJECTED"
)

// --- Core Models ---

// Space is a top-level container for content, like a project or a website.
// It defines settings like available languages.
type Space struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"is_default"`
	// Locales defines the languages supported in this space.
	Locales       []string  `json:"locales"`
	DefaultLocale string    `json:"default_locale"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Model is the schema or blueprint for a type of content.
type Model struct {
	ID        uuid.UUID `json:"id"`
	SpaceID   uuid.UUID `json:"space_id"`
	APIKey    string    `json:"api_key"`
	Name      string    `json:"name"` // Human-readable name, e.g., "News Article"
	Fields    []Field   `json:"fields"`
	IsSystem  bool      `json:"is_system"` // prevent edits/deletes
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Field defines a single piece of data within a Model's schema.
type Field struct {
	ID          string         `json:"id"`   // e.g., "title", "publicationDate"
	Name        string         `json:"name"` // e.g., "Title", "Publication Date"
	Type        FieldType      `json:"type"`
	IsRequired  bool           `json:"is_required"`
	IsLocalized bool           `json:"is_localized"`          // Can this field have different values per locale?
	Validations map[string]any `json:"validations,omitempty"` // e.g., {"maxLength": 255, "assetType": "image"}
}

// Entry is an instance of a Model, representing a single piece of content.
type Entry struct {
	ID      uuid.UUID   `json:"id"`
	ModelID uuid.UUID   `json:"model_id"`
	SpaceID uuid.UUID   `json:"space_id"`
	Slug    string      `json:"slug"` // optional field for URL-friendly references
	Status  EntryStatus `json:"status"`
	// The canonical set of fields for this entry, localized.
	// e.g., {"en-US": {"title": "Hello", "body": "..."}, "fr-FR": {"title": "Bonjour", "body": "..."}}
	Fields           map[string]map[string]any `json:"fields"`
	Version          int                       `json:"version"`                     // The latest version number of this entry
	PublishedVersion *int                      `json:"published_version,omitempty"` // The version number that is currently live
	CreatedBy        uuid.UUID                 `json:"created_by"`
	UpdatedBy        uuid.UUID                 `json:"updated_by"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
	PublishedAt      *time.Time                `json:"published_at,omitempty"`
}

// Version represents an immutable, historical snapshot of an Entry's fields.
type Version struct {
	ID        uuid.UUID                 `json:"id"`
	EntryID   uuid.UUID                 `json:"entry_id"`
	Version   int                       `json:"version"`
	Fields    map[string]map[string]any `json:"fields" gorm:"type:jsonb"`
	CreatedBy uuid.UUID                 `json:"created_by"`
	CreatedAt time.Time                 `json:"created_at"`
}

type UpdateEntryStatusParams struct {
	Status EntryStatus `validate:"required,oneof=DRAFT PUBLISHED ARCHIVED CHANGED REJECTED"` // Add REJECTED if needed
	Reason *string
}
