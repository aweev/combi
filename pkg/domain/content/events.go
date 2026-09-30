// internal/domain/content/events.go
package content

import (
	"time"

	"github.com/google/uuid"
)

// const (
// 	EventSpaceCreated core.EventAction = "content.space.created"
// 	EventSpaceUpdated core.EventAction = "content.space.updated"
// 	EventSpaceDeleted core.EventAction = "content.space.deleted"

// 	EventModelCreated core.EventAction = "content.model.created"
// 	EventModelUpdated core.EventAction = "content.model.updated"
// 	EventModelDeleted core.EventAction = "content.model.deleted"

// 	EventEntryCreated   core.EventAction = "content.entry.created"
// 	EventEntryUpdated   core.EventAction = "content.entry.updated"
// 	EventEntryPublished core.EventAction = "content.entry.published"
// 	EventEntryArchived  core.EventAction = "content.entry.archived"

// 	EventEntryVersionRestored core.EventAction = "content.entry.version_restored"

// 	EventEntryUnpublished core.EventAction = "content.entry.unpublished"
// 	EventEntryDeleted     core.EventAction = "content.entry.deleted"
// )

type ContentPublishedPayload struct {
	ContentID   uuid.UUID `json:"content_id"`
	Type        string    `json:"type"` // "article", "page", "story"
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Summary     string    `json:"summary"`
	AuthorID    uuid.UUID `json:"author_id"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
	IsPublic    bool      `json:"is_public"`
}

type ContentArchivedPayload struct {
	ContentID uuid.UUID `json:"content_id"`
}
