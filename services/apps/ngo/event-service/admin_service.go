package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ event.AdminService = (*AdminService)(nil)

type AdminService struct {
	repo     event.Repository
	rbac     rbac.AuthorizationService
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewAdminService(
	repo event.Repository,
	rbac rbac.AuthorizationService,
	audit audit.EventRecorder,
	eventBus events.Bus,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:     repo,
		rbac:     rbac,
		audit:    audit,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "event_admin")),
	}
}

// --- Event Lifecycle ---

func (s *AdminService) CreateEvent(ctx context.Context, params event.CreateEventParams) (*event.Event, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	if params.StartTime.After(params.EndTime) {
		return nil, event.ErrInvalidTimeRange
	}

	slg := params.Slug
	if slg == "" {
		slg = slug.Make(params.Name)
	}

	e := &event.Event{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		VenueID:        params.VenueID,
		CreatedBy:      params.CreatedBy,
		Name:           params.Name,
		Slug:           slg,
		Description:    params.Description,
		Status:         event.StatusDraft, // Always start as draft
		Format:         params.Format,
		StartTime:      params.StartTime,
		EndTime:        params.EndTime,
		Timezone:       params.Timezone,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateEvent(ctx, e); err != nil {
		return nil, err
	}

	s.auditSuccess(ctx, core.EventCreated, e.ID, "event", e.Name)
	return e, nil
}

func (s *AdminService) UpdateEvent(ctx context.Context, eventID uuid.UUID, params event.UpdateEventParams) (*event.Event, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	e, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		e.Name = *params.Name
	}
	if params.Description != nil {
		e.Description = *params.Description
	}
	if params.StartTime != nil {
		e.StartTime = *params.StartTime
	}
	if params.EndTime != nil {
		e.EndTime = *params.EndTime
	}
	if params.VenueID != nil {
		e.VenueID = params.VenueID
	}
	if params.StreamURL != nil {
		e.StreamURL = params.StreamURL
	}
	if params.Status != nil {
		// Simple state transition validation could go here
		e.Status = *params.Status
	}

	if e.StartTime.After(e.EndTime) {
		return nil, event.ErrInvalidTimeRange
	}

	if err := s.repo.UpdateEvent(ctx, e); err != nil {
		return nil, err
	}

	s.auditSuccess(ctx, core.EventUpdated, e.ID, "event", e.Name)

	// Publish event if status changed to PUBLISHED
	if params.Status != nil && *params.Status == event.StatusPublished {
		s.publishEvent(ctx, core.EventUpdated, e.ID, e)
	}

	return e, nil
}

func (s *AdminService) CancelEvent(ctx context.Context, eventID uuid.UUID, reason string) (*event.Event, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	e, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	e.Status = event.StatusCancelled
	// Logic could add reason to properties map

	if err := s.repo.UpdateEvent(ctx, e); err != nil {
		return nil, err
	}

	// TODO: Dispatch job to notify all registered users
	s.auditSuccess(ctx, core.EventCancelled, e.ID, "core", reason)
	s.publishEvent(ctx, core.EventCancelled, e.ID, map[string]interface{}{"event_id": e.ID, "reason": reason})

	return e, nil
}

func (s *AdminService) DeleteEvent(ctx context.Context, eventID uuid.UUID) error {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return err
	}
	// Usually soft-delete, but here we follow repo contract
	return s.repo.DeleteEvent(ctx, eventID)
}

// --- Tickets ---

func (s *AdminService) CreateTicket(ctx context.Context, eventID uuid.UUID, params event.CreateTicketParams) (*event.Ticket, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	t := &event.Ticket{
		ID:           uuid.New(),
		EventID:      eventID,
		Name:         params.Name,
		Price:        financial.Price{Amount: params.Amount, Currency: params.Currency},
		Capacity:     params.Capacity,
		SaleStartsAt: params.SaleStartsAt,
		SaleEndsAt:   params.SaleEndsAt,
		IsActive:     params.IsActive,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *AdminService) UpdateTicket(ctx context.Context, ticketID uuid.UUID, params event.UpdateTicketParams) (*event.Ticket, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	t, err := s.repo.GetTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		t.Name = *params.Name
	}
	if params.Amount != nil {
		t.Price.Amount = *params.Amount
	}
	if params.Capacity != nil {
		t.Capacity = params.Capacity
	}
	if params.SaleStartsAt != nil {
		t.SaleStartsAt = params.SaleStartsAt
	}
	if params.SaleEndsAt != nil {
		t.SaleEndsAt = params.SaleEndsAt
	}
	if params.IsActive != nil {
		t.IsActive = *params.IsActive
	}

	if err := s.repo.UpdateTicket(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// --- Schedule ---

func (s *AdminService) AddScheduleItem(ctx context.Context, eventID uuid.UUID, params event.CreateScheduleItemParams) (*event.ScheduleItem, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	item := &event.ScheduleItem{
		ID:          uuid.New(),
		EventID:     eventID,
		Title:       params.Title,
		Description: params.Description,
		StartTime:   params.StartTime,
		EndTime:     params.EndTime,
		SpeakerIDs:  params.SpeakerIDs,
		Track:       params.Track,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateScheduleItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AdminService) UpdateScheduleItem(ctx context.Context, itemID uuid.UUID, params event.UpdateScheduleItemParams) (*event.ScheduleItem, error) {
	if err := s.checkPerm(ctx, event.PermEventManage); err != nil {
		return nil, err
	}

	item, err := s.repo.GetScheduleItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if params.Title != nil {
		item.Title = *params.Title
	}
	if params.Description != nil {
		item.Description = params.Description
	}
	if params.StartTime != nil {
		item.StartTime = *params.StartTime
	}
	if params.EndTime != nil {
		item.EndTime = *params.EndTime
	}
	if params.SpeakerIDs != nil {
		item.SpeakerIDs = params.SpeakerIDs
	}
	if params.Track != nil {
		item.Track = params.Track
	}

	if err := s.repo.UpdateScheduleItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// --- Logistics ---

func (s *AdminService) ListRegistrationsForEvent(ctx context.Context, eventID uuid.UUID, filter event.RegistrationFilter) ([]*event.Registration, int, error) {
	if err := s.checkPerm(ctx, event.PermEventRegistrationRead); err != nil {
		return nil, 0, err
	}
	return s.repo.ListRegistrationsForEvent(ctx, eventID, filter)
}

func (s *AdminService) CheckInUser(ctx context.Context, registrationID uuid.UUID) (*event.Registration, error) {
	if err := s.checkPerm(ctx, event.PermEventCheckIn); err != nil {
		return nil, err
	}

	reg, err := s.repo.GetRegistration(ctx, registrationID)
	if err != nil {
		return nil, err
	}

	// Business Rule: Can only check in Confirmed users
	if reg.Status != event.RegStatusConfirmed {
		return nil, event.ErrInvalidInput // Or a more specific error
	}

	now := time.Now().UTC()
	reg.CheckedInAt = &now
	reg.Status = event.RegStatusAttended

	if err := s.repo.UpdateRegistration(ctx, reg); err != nil {
		return nil, err
	}

	s.auditSuccess(ctx, core.EventUserCheckedIn, reg.ID, "registration", "User checked in")
	return reg, nil
}

// --- Resources ---

func (s *AdminService) CreateVenue(ctx context.Context, params event.CreateVenueParams) (*event.Venue, error) {
	if err := s.checkPerm(ctx, event.PermEventVenueManage); err != nil {
		return nil, err
	}
	v := &event.Venue{
		ID:        uuid.New(),
		Name:      params.Name,
		Address:   params.Address,
		City:      params.City,
		Country:   params.Country,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateVenue(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *AdminService) CreateSpeaker(ctx context.Context, params event.CreateSpeakerParams) (*event.Speaker, error) {
	if err := s.checkPerm(ctx, event.PermEventSpeakerManage); err != nil {
		return nil, err
	}
	sp := &event.Speaker{
		ID:        uuid.New(),
		Name:      params.Name,
		Title:     params.Title,
		Bio:       params.Bio,
		AvatarURL: params.AvatarURL,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateSpeaker(ctx, sp); err != nil {
		return nil, err
	}
	return sp, nil
}

// --- Helpers ---

func (s *AdminService) checkPerm(ctx context.Context, corePerm core.PermissionName) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	allowed, err := s.rbac.Authorize(ctx, rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: corePerm,
	})
	if err != nil {
		return err
	}
	if !allowed {
		return event.ErrAccessDenied
	}
	return nil
}

func (s *AdminService) auditSuccess(ctx context.Context, action core.EventAction, targetID uuid.UUID, targetType, msg string) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return
	}
	evt := audit.NewEvent(actor.UserID, "user", action).
		WithTarget(targetID, targetType, msg).
		WithSuccess().
		WithContext(ctx).
		Build()

	if err := s.audit.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *AdminService) publishEvent(ctx context.Context, action core.EventAction, targetID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, uuid.Nil, "system", payload)
	if err == nil {
		// Add target metadata
		tType := "event"
		evt.TargetID = &targetID
		evt.TargetType = &tType
		if err := s.eventBus.Publish(ctx, evt); err != nil {
			s.logger.Error("failed to publish event", err)
		}
	}
}
