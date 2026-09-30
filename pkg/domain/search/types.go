package search

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
	ErrInvalidQuerySyntax = apperror.New(http.StatusBadRequest, "INVALID_QUERY", "The search query syntax is invalid.", nil)
	ErrUnknownIndex       = apperror.New(http.StatusBadRequest, "UNKNOWN_INDEX", "The specified search index does not exist.", nil)

	// 500 Internal
	ErrIndexFailed       = apperror.New(http.StatusInternalServerError, "INDEXING_FAILED", "Failed to update the search index.", nil)
	ErrSearchUnavailable = apperror.New(http.StatusServiceUnavailable, "SEARCH_UNAVAILABLE", "The search engine is currently offline.", nil)
)

const (
	// Permissions
	PermSearchAdmin core.PermissionName = "search:admin"

	// Jobs
	JobIndexEntity  core.JobType = "search:index_entity"
	JobRebuildIndex core.JobType = "search:rebuild_index"
	JobDeleteEntity core.JobType = "search:delete_entity"
)

type IndexName string

const (
	IndexUsers    IndexName = "users"
	IndexContent  IndexName = "content"
	IndexCatalog  IndexName = "catalog"
	IndexEvents   IndexName = "events"
	IndexProjects IndexName = "projects"
	IndexGlobal   IndexName = "global" // Virtual index searching all
)

// SearchDocument is the "Flattened" representation of any entity ready for indexing.
type SearchDocument struct {
	// Identity
	ID   string    `json:"id"`   // Format: "user:uuid"
	Type IndexName `json:"type"` // e.g. "users"

	// Display (UI Card)
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Link        string `json:"link"` // e.g. "/profile/123"

	// Content (Weighted Search)
	Content string   `json:"content"` // Main body text
	Tags    []string `json:"tags"`

	// Facets (Filtering)
	Category  string    `json:"category"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// RBAC (Security)
	// List of strings that a viewer must possess at least one of to see this doc.
	// Examples: "public", "user:{my_id}", "role:admin", "org:{org_id}"
	AllowedViewers []string `json:"allowed_viewers"`

	// Arbitrary Metadata (Not indexed for search, but returned in payload)
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SearchResult is the API response item.
type SearchResult struct {
	ID          string            `json:"id"`
	Type        IndexName         `json:"type"`
	Title       string            `json:"title"`
	Subtitle    string            `json:"subtitle"`
	Description string            `json:"description"`
	ImageURL    string            `json:"image_url"`
	Link        string            `json:"link"`
	Score       float64           `json:"score"`
	Highlights  map[string]string `json:"highlights"`
	Metadata    map[string]any    `json:"metadata"`
}

// SearchResponse is the full result set.
type SearchResponse struct {
	Hits             []SearchResult            `json:"hits"`
	TotalHits        int                       `json:"total_hits"`
	ProcessingTimeMs int64                     `json:"processing_time_ms"`
	Facets           map[string]map[string]int `json:"facets,omitempty"`
}

type SearchQuery struct {
	Index        IndexName
	Term         string
	Filters      string
	Facets       []string
	Limit        int
	Offset       int
	ViewerScopes []string // e.g. ["public", "user:UUID", "role:admin"]
}

// SearchParams represents the user intent.
type SearchParams struct {
	Query   string
	Index   IndexName
	Filters string
	Page    int
	Limit   int
	Facets  []string

	// Security Context (Populated by Handler)
	ViewerID     uuid.UUID
	ViewerRoles  []string
	ViewerOrgIDs []uuid.UUID
}

type Engine interface {
	Search(ctx context.Context, query SearchQuery) (*SearchResponse, error)

	// Live Updates
	IndexBatch(ctx context.Context, docs []SearchDocument) error
	DeleteBatch(ctx context.Context, ids []string) error

	// Blue/Green Rebuild Lifecycle
	// 1. Prepare: Creates search_documents_staging
	PrepareRebuild(ctx context.Context) error
	// 2. Index: Inserts into staging table
	IndexBatchStaging(ctx context.Context, docs []SearchDocument) error
	// 3. Commit: Swaps staging to live (Drop old, Rename new)
	CommitRebuild(ctx context.Context) error

	Ping(ctx context.Context) error
}

// DocumentProvider is implemented by domain services (User, Content)
// to allow the Search service to "pull" data for rebuilds.
type DocumentProvider interface {
	// IterateDocs pushes documents to the 'out' channel.
	// It MUST stop and return if ctx.Done() is closed.
	IterateDocs(ctx context.Context, out chan<- SearchDocument) error
}

type Service interface {
	Search(ctx context.Context, params SearchParams) (*SearchResponse, error)
	IndexDocument(ctx context.Context, doc SearchDocument) error
	DeleteDocument(ctx context.Context, id string) error
	TriggerRebuild(ctx context.Context, index IndexName) error

	// ProcessRebuildJob is internal but exposed for the Worker
	ProcessRebuildJob(ctx context.Context, indexName IndexName) error

	// RegisterProvider links a domain to an index
	RegisterProvider(index IndexName, provider DocumentProvider)
}
