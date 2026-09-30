package catalog

import (
	"context"

	"github.com/google/uuid"
)

// --- Inbound Ports (Services) ---

// Service is the public-facing read/interaction layer.
type Service interface {
	GetItem(ctx context.Context, itemID uuid.UUID) (*Item, error)
	GetItemBySlug(ctx context.Context, slug string) (*Item, error)
	ListItems(ctx context.Context, filter ItemFilter) ([]*Item, int, error)
	ListVariants(ctx context.Context, itemID uuid.UUID) ([]*Variant, error)
	GetCategoryTree(ctx context.Context) ([]*CategoryNode, error)

	SubmitApplication(ctx context.Context, params SubmitApplicationParams) (*Application, error)
	ListUserApplications(ctx context.Context, userID uuid.UUID) ([]*EnrichedApplication, error)
	WithdrawApplication(ctx context.Context, applicationID, userID uuid.UUID) error
}

// AdminService manages the catalog structure and workflow.
type AdminService interface {
	// CRUD
	CreateItem(ctx context.Context, params CreateItemParams) (*Item, error)
	UpdateItem(ctx context.Context, itemID uuid.UUID, params UpdateItemParams) (*Item, error)
	DeleteItem(ctx context.Context, itemID uuid.UUID) error

	CreateCategory(ctx context.Context, params CreateCategoryParams) (*Category, error)
	UpdateCategory(ctx context.Context, categoryID uuid.UUID, params UpdateCategoryParams) (*Category, error)
	DeleteCategory(ctx context.Context, categoryID uuid.UUID) error

	// Application Management
	GetApplication(ctx context.Context, applicationID uuid.UUID) (*EnrichedApplication, error)
	ListApplicationsForItem(ctx context.Context, itemID uuid.UUID, filter ApplicationFilter) ([]*EnrichedApplication, int, error)
	UpdateApplicationStatus(ctx context.Context, applicationID uuid.UUID, status ApplicationStatus, reason *string) (*Application, error)
}

// --- Outbound Ports (Dependencies) ---

// SearchIndexer updates the external search index (Search Service).
type SearchIndexer interface {
	IndexItem(ctx context.Context, item Item) error
	DeleteItem(ctx context.Context, itemID uuid.UUID) error
}

// NotificationDispatcher sends alerts (Notification Service).
type NotificationDispatcher interface {
	DispatchStatusUpdate(ctx context.Context, app Application, reason *string) error
}

// WorkflowManager starts business processes (Workflow Service).
type WorkflowManager interface {
	StartApplicationReview(ctx context.Context, app Application) error
}

// --- DTOs ---

type CreateItemParams struct {
	Type        string
	Name        string
	Slug        string
	Description string
	Status      ItemStatus
	Properties  map[string]interface{}
	Tags        []string
	CategoryIDs []uuid.UUID
}

type UpdateItemParams struct {
	Name        *string
	Slug        *string
	Description *string
	Status      *ItemStatus
	Properties  map[string]any
	Tags        []string
	CategoryIDs []uuid.UUID
}

type ItemFilter struct {
	Type       *string
	Status     *ItemStatus
	CategoryID *uuid.UUID
	Search     string
	Limit      int
	Offset     int
}

type SubmitApplicationParams struct {
	UserID    uuid.UUID
	ItemID    uuid.UUID
	VariantID *uuid.UUID
	Answers   map[string]any
}

type ApplicationFilter struct {
	Status *ApplicationStatus
	Limit  int
	Offset int
}

type CreateCategoryParams struct {
	ParentID    *uuid.UUID
	Name        string
	Slug        string
	Description *string
}

type UpdateCategoryParams struct {
	ParentID    *uuid.UUID
	Name        *string
	Slug        *string
	Description *string
}
