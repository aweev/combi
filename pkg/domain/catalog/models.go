package catalog

import (
	"encoding/json"
	"time"

	"github.com/combi/pkg/domain/user"
	"github.com/google/uuid"
)

// --- Enums ---

type ItemStatus string

const (
	StatusDraft      ItemStatus = "DRAFT"
	StatusAvailable  ItemStatus = "AVAILABLE"
	StatusArchived   ItemStatus = "ARCHIVED"
	StatusComingSoon ItemStatus = "COMING_SOON"
)

type ApplicationStatus string

const (
	AppStatusPending   ApplicationStatus = "PENDING"
	AppStatusAccepted  ApplicationStatus = "ACCEPTED"
	AppStatusRejected  ApplicationStatus = "REJECTED"
	AppStatusWithdrawn ApplicationStatus = "WITHDRAWN"
	AppStatusApproved  ApplicationStatus = "APPROVED"
)

func (s ApplicationStatus) IsFinal() bool {
	return s == AppStatusAccepted || s == AppStatusRejected || s == AppStatusWithdrawn || s == AppStatusApproved
}

// --- Core Models ---

// Category represents a hierarchical grouping (e.g., "Education", "Healthcare").
type Category struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ParentID    *uuid.UUID `json:"parent_id" db:"parent_id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description *string    `json:"description" db:"description"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Item is the central entity. It represents Programs, Volunteer Opportunities, or Products.
type Item struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Type        string     `json:"type" db:"type"` // e.g., "PROGRAM", "VOLUNTEER_OPP"
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description string     `json:"description" db:"description"`
	Status      ItemStatus `json:"status" db:"status"`

	// Properties stores type-specific data (e.g., {"location": "Berlin", "hours": 5})
	// Handled as JSONB in Postgres.
	Properties json.RawMessage `json:"properties" db:"properties"`

	// Tags for searching/filtering
	Tags json.RawMessage `json:"tags" db:"tags"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Variant allows an item to have options (e.g., "T-Shirt Size L" or "Weekend Shift").
type Variant struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	ItemID     uuid.UUID       `json:"item_id" db:"item_id"`
	SKU        string          `json:"sku" db:"sku"`
	Name       string          `json:"name" db:"name"`
	Price      int64           `json:"price" db:"price_amount"`      // Smallest currency unit
	Currency   string          `json:"currency" db:"price_currency"` // ISO 4217
	Stock      *int            `json:"stock" db:"stock"`             // Nil = Infinite
	Attributes json.RawMessage `json:"attributes" db:"attributes"`   // JSONB
	IsActive   bool            `json:"is_active" db:"is_active"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// Application tracks a user applying for an Item.
type Application struct {
	ID        uuid.UUID         `json:"id" db:"id"`
	UserID    uuid.UUID         `json:"user_id" db:"user_id"`
	ItemID    uuid.UUID         `json:"item_id" db:"item_id"`
	VariantID *uuid.UUID        `json:"variant_id" db:"variant_id"`
	Status    ApplicationStatus `json:"status" db:"status"`

	// Answers to application form questions
	Answers json.RawMessage `json:"answers" db:"answers"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// --- DTOs for Logic/Responses ---

// EnrichedApplication includes related entities for API responses.
type EnrichedApplication struct {
	Application
	User user.PublicProfile `json:"user"`
	Item Item               `json:"item"`
}

// CategoryNode represents the tree structure for the UI.
type CategoryNode struct {
	Category
	Children []*CategoryNode `json:"children"`
}

// OpportunityCreatedPayload (for "catalog.opportunity.created")
type OpportunityCreatedPayload struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Type           string    `json:"type"` // "volunteer_role", "mentorship"
	RequiredSkills []string  `json:"required_skills"`
	Location       string    `json:"location"`
	OrgID          uuid.UUID `json:"organization_id"`
}

// ApplicationSubmittedPayload (for "catalog.application.submitted")
type ApplicationSubmittedPayload struct {
	ApplicationID uuid.UUID `json:"application_id"`
	ItemID        uuid.UUID `json:"item_id"`
	UserID        uuid.UUID `json:"user_id"`
	Status        string    `json:"status"`
}
