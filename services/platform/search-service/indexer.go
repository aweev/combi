package search

import (
	"context"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Indexer struct {
	service search.Service
	logger  log.Logger
}

func NewIndexer(service search.Service, logger log.Logger) *Indexer {
	return &Indexer{
		service: service,
		logger:  logger.With(log.String("component", "search_indexer")),
	}
}

// HandleEvent processes incoming domain events and updates the index.
func (i *Indexer) HandleEvent(ctx context.Context, evt events.DomainEvent) error {
	switch evt.Action {
	// --- Users ---
	case core.EventUserCreated, core.EventUserProfileUpdated:
		return i.indexUser(ctx, evt)
	case core.EventUserDeleted:
		return i.service.DeleteDocument(ctx, "user:"+evt.TargetID.String())

	// --- Content (CMS) ---
	case core.EventContentPublished, core.EventContentUpdated:
		return i.indexContent(ctx, evt)
	case core.EventContentUnpublished, core.EventContentDeleted:
		// If unpublished, we usually remove from search index to hide it
		return i.service.DeleteDocument(ctx, "content:"+evt.TargetID.String())

	// --- Catalog (Opportunities) ---
	case core.EventOpportunityCreated: // Assuming update event exists too
		return i.indexOpportunity(ctx, evt)
	case core.EventOpportunityDeleted:
		return i.service.DeleteDocument(ctx, "opportunity:"+evt.TargetID.String())

	// --- Projects ---
	case core.EventProjectCreated, core.EventProjectUpdated:
		return i.indexProject(ctx, evt)
	case core.EventProjectArchived:
		// Archived projects might still be searchable but with low weight, or removed.
		// Let's keep them indexed but mark status as archived.
		// Since the Payload of Archive event might not have full details, we rely on the ProjectUpdated payload logic if passed,
		// or fetch it. If the event payload is just ID, this might fail to re-index full data.
		// For simplicity, let's treat Archive as an Update if payload exists, else skip.
		return i.indexProject(ctx, evt)
	}
	return nil
}

func (i *Indexer) indexUser(ctx context.Context, evt events.DomainEvent) error {
	// 1. Unmarshal Payload
	// We expect the payload to match user.ProfileUpdatedPayload or similar snapshot
	var p user.ProfileUpdatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		// Fallback: Try generic map if payload structure varies
		return fmt.Errorf("failed to unmarshal user payload: %w", err)
	}

	// 2. Map to Search Document
	displayName := p.DisplayName
	if displayName == nil {
		s := "Unknown User"
		displayName = &s
	}
	bio := ""
	if p.Bio != nil {
		bio = *p.Bio
	}

	doc := search.SearchDocument{
		ID:          "user:" + p.UserID.String(),
		Type:        search.IndexUsers,
		Title:       *displayName,
		Subtitle:    "Member",
		Description: bio,
		// ImageURL:    resolved in frontend or passed if available in payload
		Link:           "/profile/" + p.UserID.String(),
		Content:        *displayName + " " + bio,
		Status:         "active",
		CreatedAt:      evt.Timestamp,
		UpdatedAt:      time.Now().UTC(),
		AllowedViewers: []string{"public"}, // Adjust based on privacy settings if passed
	}

	return i.service.IndexDocument(ctx, doc)
}

func (i *Indexer) indexContent(ctx context.Context, evt events.DomainEvent) error {
	var p content.ContentPublishedPayload // Ensure this struct is defined in domain
	if err := evt.UnmarshalPayload(&p); err != nil {
		// Try ContentUpdatedPayload if different
		return fmt.Errorf("failed to unmarshal content payload: %w", err)
	}

	// Combine tags for searching
	tagsStr := ""
	for _, t := range p.Tags {
		tagsStr += t + " "
	}

	doc := search.SearchDocument{
		ID:             "content:" + p.ContentID.String(),
		Type:           search.IndexContent,
		Title:          p.Title,
		Subtitle:       p.Slug,
		Description:    p.Summary,
		Link:           "/content/" + p.Slug,
		Content:        p.Title + " " + p.Summary + " " + tagsStr,
		Tags:           p.Tags,
		Status:         "published",
		CreatedAt:      evt.Timestamp,
		UpdatedAt:      time.Now().UTC(),
		AllowedViewers: []string{"public"}, // CMS content usually public
	}

	return i.service.IndexDocument(ctx, doc)
}

func (i *Indexer) indexOpportunity(ctx context.Context, evt events.DomainEvent) error {
	var p catalog.OpportunityCreatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("failed to unmarshal opportunity payload: %w", err)
	}

	skillsStr := ""
	for _, s := range p.RequiredSkills {
		skillsStr += s + " "
	}

	doc := search.SearchDocument{
		ID:             "opportunity:" + p.ID.String(),
		Type:           search.IndexCatalog,
		Title:          p.Title,
		Subtitle:       p.Location,
		Description:    p.Description,
		Link:           "/opportunities/" + p.ID.String(),
		Content:        p.Title + " " + p.Description + " " + skillsStr + " " + p.Location,
		Tags:           p.RequiredSkills,
		Status:         "open",
		CreatedAt:      evt.Timestamp,
		UpdatedAt:      time.Now().UTC(),
		AllowedViewers: []string{"public"},
	}

	return i.service.IndexDocument(ctx, doc)
}

func (i *Indexer) indexProject(ctx context.Context, evt events.DomainEvent) error {
	// Assuming payload is the Project struct or a specific payload
	var p project.Project
	if err := evt.UnmarshalPayload(&p); err != nil {
		return fmt.Errorf("failed to unmarshal project payload: %w", err)
	}

	// Project visibility logic
	viewers := []string{
		"org:" + p.OrganizationID.String(), // Visible to whole org?
		"project:" + p.ID.String(),         // Visible to members
	}

	statusStr := string(p.Status)

	doc := search.SearchDocument{
		ID:             "project:" + p.ID.String(),
		Type:           search.IndexProjects, // Define this in domain if missing
		Title:          p.Name,
		Subtitle:       statusStr,
		Description:    p.Description,
		Link:           "/projects/" + p.ID.String(),
		Content:        p.Name + " " + p.Description,
		Status:         statusStr,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		AllowedViewers: viewers,
	}

	return i.service.IndexDocument(ctx, doc)
}
