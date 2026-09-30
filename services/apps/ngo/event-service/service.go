// internal/services/event/service.go
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ event.Service = (*Service)(nil)
var _ reporting.DataFetcher = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo       event.Repository
	audit      audit.EventRecorder
	eventBus   events.Bus
	paymentSvc financial.Service
	logger     log.Logger
}

func NewService(
	repo event.Repository,
	audit audit.EventRecorder,
	eventBus events.Bus,
	paymentSvc financial.Service,
	logger log.Logger,
) *Service {
	return &Service{
		repo:       repo,
		audit:      audit,
		eventBus:   eventBus,
		paymentSvc: paymentSvc,
		logger:     logger.With(log.String("service", "event_public")),
	}
}

func (s *Service) FetchData(ctx context.Context, params map[string]any) (any, error) {
	// 1. Extract Event ID
	eventIDStr, ok := params["event_id"].(string)
	if !ok {
		return nil, fmt.Errorf("event_id required")
	}
	eventID := uuid.MustParse(eventIDStr)

	// 2. Fetch Registrations (Using Admin Access)
	regs, _, err := s.repo.ListRegistrationsForEvent(ctx, eventID, event.RegistrationFilter{
		Limit: 10000, // No pagination for report export
	})
	if err != nil {
		return nil, err
	}

	// 3. Enhance with User Data (if needed, but usually registration has basics or links)
	// Here we return the raw list or a simplified view
	type AttendeeRow struct {
		RegistrationID string `json:"registration_id"`
		UserID         string `json:"user_id"`
		Status         string `json:"status"`
		TicketID       string `json:"ticket_id"`
	}

	rows := make([]AttendeeRow, len(regs))
	for i, r := range regs {
		rows[i] = AttendeeRow{
			RegistrationID: r.ID.String(),
			UserID:         r.UserID.String(), // Ideally Name (requires User Service call or Join)
			Status:         string(r.Status),
			TicketID:       r.TicketID.String(),
		}
	}

	return rows, nil
}

// --- Event Discovery (Reads) ---

func (s *Service) GetEvent(ctx context.Context, id uuid.UUID) (*event.Event, error) {
	return s.repo.GetEvent(ctx, id)
}

func (s *Service) GetEventBySlug(ctx context.Context, slug string) (*event.Event, error) {
	return s.repo.GetEventBySlug(ctx, slug)
}

func (s *Service) ListEvents(ctx context.Context, filter event.EventFilter) ([]*event.Event, int, error) {
	// Production enhancement: Default to only showing PUBLISHED events for public API
	// unless specifically requested otherwise (and caller is admin, though this service is public-facing).
	// We trust the handler/filter to set this, but we can enforce it if filter.Status is nil.
	if filter.Status == nil {
		st := event.StatusPublished
		filter.Status = &st
	}
	return s.repo.ListEvents(ctx, filter)
}

func (s *Service) ListTicketsForEvent(ctx context.Context, eventID uuid.UUID) ([]*event.Ticket, error) {
	evt, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt.Status == event.StatusDraft {
		return nil, event.ErrEventNotFound
	}
	return s.repo.ListTicketsForEvent(ctx, eventID)
}

func (s *Service) ListScheduleForEvent(ctx context.Context, eventID uuid.UUID) ([]*event.ScheduleItem, error) {
	evt, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt.Status == event.StatusDraft {
		return nil, event.ErrEventNotFound
	}
	return s.repo.ListScheduleItemsForEvent(ctx, eventID)
}

// --- Registration Flow (Writes) ---

func (s *Service) RegisterForEvent(ctx context.Context, params event.RegisterParams) (*event.Registration, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != params.UserID {
		return nil, auth.ErrUnauthorized
	}

	// 1. Validation & Availability Check
	evt, err := s.repo.GetEvent(ctx, params.EventID)
	if err != nil {
		return nil, err
	}
	if evt.Status != event.StatusPublished {
		return nil, event.ErrRegistrationClosed
	}

	// Idempotency check
	if _, err := s.repo.GetUserRegistrationForEvent(ctx, params.UserID, params.EventID); err == nil {
		return nil, event.ErrAlreadyRegistered
	}

	ticket, err := s.repo.GetTicket(ctx, params.TicketID)
	if err != nil {
		return nil, event.ErrTicketNotFound
	}
	if !ticket.IsAvailable() {
		return nil, event.ErrTicketNotAvailable
	}
	if ticket.EventID != evt.ID {
		return nil, event.ErrTicketNotFound
	}

	// 2. Payment Processing (if applicable)
	var transactionID *uuid.UUID

	// PRODUCTION LOGIC: Paid Ticket
	if ticket.Price.Amount > 0 {
		if s.paymentSvc == nil {
			s.logger.Error("Paid ticket requested but Financial Service is missing", err)
			return nil, apperror.New(500, "SYSTEM_CONFIG_ERROR", "Payment processing unavailable", nil)
		}
		if params.ProviderToken == nil {
			return nil, apperror.New(400, "PAYMENT_REQUIRED", "Payment token required for paid tickets", nil)
		}

		// Call Financial Service
		// We use the new CreateOneTimePayment signature
		paymentParams := financial.CreatePaymentParams{
			UserID:         params.UserID,
			Amount:         ticket.Price.Amount,
			Currency:       ticket.Price.Currency,
			Description:    fmt.Sprintf("Event: %s (%s)", evt.Name, ticket.Name),
			ProviderToken:  params.ProviderToken,
			IdempotencyKey: fmt.Sprintf("reg:%s:%s", params.EventID, params.UserID), // Unique key
			Metadata: map[string]interface{}{
				"event_id":  evt.ID.String(),
				"ticket_id": ticket.ID.String(),
			},
		}

		tx, err := s.paymentSvc.CreateOneTimePayment(ctx, paymentParams)
		if err != nil {
			// Payment failed (Declined, etc.) -> Propagate error to user
			return nil, err
		}
		if tx.Status == financial.StatusFailed {
			return nil, financial.ErrPaymentDeclined
		}
		transactionID = &tx.ID
	}

	// 3. Create Registration (Transactional)
	reg := &event.Registration{
		ID:            uuid.New(),
		UserID:        params.UserID,
		EventID:       params.EventID,
		TicketID:      params.TicketID,
		Status:        event.RegStatusConfirmed,
		TransactionID: transactionID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	err = s.repo.WithTx(ctx, func(txRepo event.Repository) error {
		// Double-check capacity inside lock to prevent overbooking race conditions
		// Note: This requires GetTicket to be part of the txRepo (which it is)
		t, err := txRepo.GetTicket(ctx, ticket.ID)
		if err != nil {
			return err
		}
		if !t.IsAvailable() {
			return event.ErrTicketNotAvailable
		}

		if err := txRepo.CreateRegistration(ctx, reg); err != nil {
			return err
		}
		if err := txRepo.IncrementRegistrationsCount(ctx, ticket.ID, 1); err != nil {
			return err
		}
		return nil
	})

	// 4. Rollback Payment if DB Fails
	if err != nil {
		if transactionID != nil {
			// CRITICAL: We charged the user, but failed to register them. We MUST refund.
			// We assume AdminService has IssueRefund. If public service doesn't have access,
			// we need a way to trigger it. Ideally, Financial Service has a Refund method exposed or we dispatch a job.
			// For this implementation, we log a CRITICAL error for manual intervention if auto-refund isn't available,
			// or call the payment service if it allows refunds.

			// Assuming s.paymentSvc (Financial Service) might NOT have Refund exposed in public interface
			// based on your interface defs (it was in AdminService).
			// WE SHOULD LOG THIS LOUDLY.
			s.logger.Error("CRITICAL: Charged user but failed to register. REFUND REQUIRED.", err,
				log.String("transaction_id", transactionID.String()),
				log.String("user_id", params.UserID.String()),
			)
		}
		return nil, err
	}

	// 5. Post-Process (Events/Audit)
	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventRegistrationCreated).
		WithTarget(reg.ID, "registration", evt.Name).WithSuccess().Build()); err != nil {
		s.logger.Error("failed to record audit event for registration", err)
	}

	s.publishEvent(ctx, core.EventRegistrationCreated, params.UserID, reg)

	return reg, nil
}

func (s *Service) WithdrawRegistration(ctx context.Context, registrationID uuid.UUID) (*event.Registration, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	var reg *event.Registration

	err := s.repo.WithTx(ctx, func(txRepo event.Repository) error {
		r, err := txRepo.GetRegistration(ctx, registrationID)
		if err != nil {
			return err
		}
		// Check ownership
		if r.UserID != actor.UserID {
			return event.ErrAccessDenied
		}
		if r.Status == event.RegStatusCancelled {
			reg = r
			return nil // Idempotent
		}

		r.Status = event.RegStatusCancelled
		r.UpdatedAt = time.Now().UTC()

		if err := txRepo.UpdateRegistration(ctx, r); err != nil {
			return err
		}
		// Decrement ticket count so someone else can take the spot
		if err := txRepo.IncrementRegistrationsCount(ctx, r.TicketID, -1); err != nil {
			return err
		}
		reg = r
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Note: Automatic refunds for withdrawals are complex business logic (policy, fees, timing).
	// Usually handled by a separate workflow or manual admin review, so we don't trigger it here automatically.

	if err := s.audit.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventRegistrationWithdrawn).
		WithTarget(registrationID, "registration", "").WithSuccess().Build()); err != nil {
		s.logger.Error("failed to record audit event for withdrawal", err)
	}

	return reg, nil
}

func (s *Service) GetUserRegistration(ctx context.Context, userID, eventID uuid.UUID) (*event.Registration, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil || actor.UserID != userID {
		return nil, event.ErrAccessDenied
	}
	return s.repo.GetUserRegistrationForEvent(ctx, userID, eventID)
}

// ApplySyncChange handles offline registration requests.
// NOTE: Payment processing is skipped here as we assume offline registrations are either free
// or handled via "pay later" invoice logic. If strict payment is required, offline sync is risky.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	switch op.EntityType {
	case "event_registration":
		if op.Action != sync_domain.ActionCreate {
			return uuid.Nil, 0, fmt.Errorf("only CREATE action supported for offline registrations")
		}

		// 1. Unmarshal Payload
		var req struct {
			EventID  uuid.UUID `json:"event_id"`
			TicketID uuid.UUID `json:"ticket_id"`
		}
		if err := json.Unmarshal(op.Payload, &req); err != nil {
			return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
		}

		// 2. Call Logic (Reusing existing RegisterForEvent)
		// We mock the context actor since ApplySyncChange is called by SyncService which validated auth
		// However, RegisterForEvent checks ActorFromContext internally.
		// We must inject the actor into the context.
		actor := &auth.Actor{UserID: actorID}
		userCtx := auth.NewContextWithActor(ctx, actor)

		// Note: We pass nil for ProviderToken, implying offline registrations are free or invoicable.
		// If the ticket requires payment, RegisterForEvent will return an error (correct behavior).
		reg, err := s.RegisterForEvent(userCtx, event.RegisterParams{
			UserID:        actorID,
			EventID:       req.EventID,
			TicketID:      req.TicketID,
			ProviderToken: nil, // Offline flow constraint
		})

		if err != nil {
			// Map specific errors if needed
			return uuid.Nil, 0, err
		}

		return reg.ID, 1, nil

	case "ticket":
		// Handle Check-In (Update Ticket/Registration status)
		// Assuming staff app sends ticket update to mark 'checked_in'
		// For MVP, we'll skip this implementation complexity or map to CheckIn method
		return uuid.Nil, 0, fmt.Errorf("ticket updates not supported in MVP")

	default:
		return uuid.Nil, 0, fmt.Errorf("unknown entity type: %s", op.EntityType)
	}
}

// FetchDelta returns upcoming events and the user's registrations.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	results := make([]sync_domain.EntityState, 0)

	// 1. Fetch Active/Published Events
	// We want the user to see what's coming up.
	// Filtering: Updated after 'since' OR StartDate in future (to catch new relevant events)
	// For simplicity, we fetch published events and filter by UpdatedAt.

	publishedStatus := event.StatusPublished
	filter := event.EventFilter{
		Status: &publishedStatus,
		Limit:  100, // Cap for safety
	}
	eventsList, _, err := s.repo.ListEvents(ctx, filter)
	if err != nil {
		return nil, err
	}

	for _, e := range eventsList {
		if e.UpdatedAt.After(since) {
			data, _ := json.Marshal(e)
			results = append(results, sync_domain.EntityState{
				ID:        e.ID,
				Version:   int(e.UpdatedAt.Unix()),
				Deleted:   false,
				Data:      data,
				UpdatedAt: e.UpdatedAt,
			})
		}
	}

	// 2. Fetch User's Registrations
	// User needs to know if they are registered or waitlisted.
	userRegs, _, err := s.repo.ListRegistrationsForUser(ctx, actorID, event.RegistrationFilter{})
	if err != nil {
		return nil, err
	}

	for _, r := range userRegs {
		if r.UpdatedAt.After(since) {
			data, _ := json.Marshal(r)
			results = append(results, sync_domain.EntityState{
				ID:        r.ID,
				Version:   int(r.UpdatedAt.Unix()),
				Deleted:   r.Status == event.RegStatusCancelled, // Treat cancelled as deleted for sync view? Or just status update.
				Data:      data,
				UpdatedAt: r.UpdatedAt,
			})
		}
	}

	return results, nil
}

// Helper
func (s *Service) publishEvent(ctx context.Context, action core.EventAction, userID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, userID, "user", payload)
	if err == nil {
		_ = s.eventBus.Publish(ctx, evt)
	}
}
