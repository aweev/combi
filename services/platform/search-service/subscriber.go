package search

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/catalog"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

type Subscriber struct {
	service search.Service
	logger  log.Logger
}

func NewSubscriber(service search.Service, logger log.Logger) *Subscriber {
	return &Subscriber{service: service, logger: logger}
}

func (s *Subscriber) SubscribeToAll(bus events.Bus) {
	// User Events
	bus.Subscribe(string(core.EventUserCreated), s.handleUserUpsert)
	bus.Subscribe(string(core.EventUserProfileUpdated), s.handleUserUpsert)
	bus.Subscribe(string(core.EventUserDeleted), s.handleUserDelete)

	// Content Events
	bus.Subscribe(string(core.EventContentPublished), s.handleContentUpsert)
	bus.Subscribe(string(core.EventContentUpdated), s.handleContentUpsert)
	bus.Subscribe(string(core.EventContentDeleted), s.handleContentDelete)

	// Catalog Events
	bus.Subscribe(string(core.EventOpportunityCreated), s.handleOpportunityUpsert)
}

func (s *Subscriber) handleUserUpsert(ctx context.Context, evt events.DomainEvent) error {
	var p user.ProfileUpdatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}

	doc := search.SearchDocument{
		ID:             "user:" + p.UserID.String(),
		Type:           search.IndexUsers,
		Title:          safeStr(p.DisplayName),
		Content:        safeStr(p.Bio),
		AllowedViewers: []string{"public"},
		UpdatedAt:      time.Now().UTC(),
	}
	return s.service.IndexDocument(ctx, doc)
}

func (s *Subscriber) handleContentUpsert(ctx context.Context, evt events.DomainEvent) error {
	var p content.ContentPublishedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}

	doc := search.SearchDocument{
		ID:             "content:" + p.ContentID.String(),
		Type:           search.IndexContent,
		Title:          p.Title,
		Subtitle:       p.Slug,
		Description:    p.Summary,
		Link:           "/content/" + p.Slug,
		Tags:           p.Tags,
		AllowedViewers: []string{"public"},
		UpdatedAt:      time.Now().UTC(),
	}
	return s.service.IndexDocument(ctx, doc)
}

func (s *Subscriber) handleOpportunityUpsert(ctx context.Context, evt events.DomainEvent) error {
	var p catalog.OpportunityCreatedPayload
	if err := evt.UnmarshalPayload(&p); err != nil {
		return nil
	}

	doc := search.SearchDocument{
		ID:             "opportunity:" + p.ID.String(),
		Type:           search.IndexCatalog,
		Title:          p.Title,
		Subtitle:       p.Location,
		Description:    p.Type,
		Tags:           p.RequiredSkills,
		AllowedViewers: []string{"public"},
		UpdatedAt:      time.Now().UTC(),
	}
	return s.service.IndexDocument(ctx, doc)
}

func (s *Subscriber) handleUserDelete(ctx context.Context, evt events.DomainEvent) error {
	return s.service.DeleteDocument(ctx, "user:"+evt.ActorID.String())
}

func (s *Subscriber) handleContentDelete(ctx context.Context, evt events.DomainEvent) error {
	if evt.TargetID == nil {
		return nil
	}
	return s.service.DeleteDocument(ctx, "content:"+evt.TargetID.String())
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
