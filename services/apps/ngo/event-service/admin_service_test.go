package event_test

import (
	"context"
	"testing"
	"time"

	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/event"
	event_mocks "github.com/combi/pkg/domain/event/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	event_service "github.com/combi/event"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type adminServiceTestSuite struct {
	repo    *event_mocks.MockRepository
	rbac    *rbac_mocks.MockAuthorizationService
	audit   *audit_mocks.MockEventRecorder
	bus     *events_mocks.MockBus
	service event.AdminService
}

func setupAdminService(t *testing.T) adminServiceTestSuite {
	ts := adminServiceTestSuite{
		repo:  event_mocks.NewMockRepository(t),
		rbac:  rbac_mocks.NewMockAuthorizationService(t),
		audit: audit_mocks.NewMockEventRecorder(t),
		bus:   events_mocks.NewMockBus(t),
	}
	ts.service = event_service.NewAdminService(
		ts.repo, ts.rbac, ts.audit, ts.bus, &log.NoOpLogger{},
	)
	return ts
}

func contextWithAdmin(adminID uuid.UUID) context.Context {
	actor := &auth.Actor{UserID: adminID, AccountID: adminID}
	return auth.NewContextWithActor(context.Background(), actor)
}

// --- EVENT CRUD ---

func TestAdminService_CreateEvent(t *testing.T) {
	adminID := uuid.New()
	ctx := contextWithAdmin(adminID)

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)
		params := event.CreateEventParams{
			Name:      "New Event",
			Slug:      "new-event",
			StartTime: time.Now().Add(time.Hour),
			EndTime:   time.Now().Add(2 * time.Hour),
			CreatedBy: adminID,
		}

		// 1. RBAC
		ts.rbac.EXPECT().Authorize(ctx, mock.MatchedBy(func(c interface{}) bool { return true })).Return(true, nil).Once()

		// 2. Repo
		ts.repo.EXPECT().CreateEvent(ctx, mock.MatchedBy(func(e *event.Event) bool {
			return e.Name == "New Event" && e.Status == event.StatusDraft
		})).Return(nil).Once()

		// 3. Audit
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		e, err := ts.service.CreateEvent(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "New Event", e.Name)
	})

	t.Run("Fail_InvalidTime", func(t *testing.T) {
		ts := setupAdminService(t)
		params := event.CreateEventParams{
			StartTime: time.Now().Add(2 * time.Hour),
			EndTime:   time.Now().Add(time.Hour), // End before Start
		}

		ts.rbac.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()

		_, err := ts.service.CreateEvent(ctx, params)
		assert.ErrorIs(t, err, event.ErrInvalidTimeRange)
	})
}

func TestAdminService_UpdateEvent(t *testing.T) {
	adminID := uuid.New()
	ctx := contextWithAdmin(adminID)
	eventID := uuid.New()

	t.Run("Success_Publish", func(t *testing.T) {
		ts := setupAdminService(t)
		status := event.StatusPublished
		params := event.UpdateEventParams{Status: &status}
		existing := &event.Event{ID: eventID, Status: event.StatusDraft, StartTime: time.Now(), EndTime: time.Now().Add(time.Hour)}

		ts.rbac.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(existing, nil).Once()

		ts.repo.EXPECT().UpdateEvent(ctx, mock.MatchedBy(func(e *event.Event) bool {
			return e.Status == event.StatusPublished
		})).Return(nil).Once()

		// Expect Audit AND Bus Event (since published)
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()

		updated, err := ts.service.UpdateEvent(ctx, eventID, params)
		require.NoError(t, err)
		assert.Equal(t, event.StatusPublished, updated.Status)
	})
}

// --- TICKET MANAGEMENT ---

func TestAdminService_CreateTicket(t *testing.T) {
	adminID := uuid.New()
	ctx := contextWithAdmin(adminID)
	eventID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)
		params := event.CreateTicketParams{
			Name:     "VIP",
			Amount:   5000,
			Currency: "USD",
			IsActive: true,
		}

		ts.rbac.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()

		ts.repo.EXPECT().CreateTicket(ctx, mock.MatchedBy(func(ticket *event.Ticket) bool {
			return ticket.Name == "VIP" && ticket.Price.Amount == 5000
		})).Return(nil).Once()

		ticket, err := ts.service.CreateTicket(ctx, eventID, params)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), ticket.Price.Amount)
	})
}

// --- LOGISTICS ---

func TestAdminService_CheckInUser(t *testing.T) {
	adminID := uuid.New()
	ctx := contextWithAdmin(adminID)
	regID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		ts := setupAdminService(t)
		reg := &event.Registration{ID: regID, Status: event.RegStatusConfirmed}

		ts.rbac.EXPECT().Authorize(ctx, mock.MatchedBy(func(c interface{}) bool { return true })).Return(true, nil).Once()
		ts.repo.EXPECT().GetRegistration(ctx, regID).Return(reg, nil).Once()

		ts.repo.EXPECT().UpdateRegistration(ctx, mock.MatchedBy(func(r *event.Registration) bool {
			return r.Status == event.RegStatusAttended && r.CheckedInAt != nil
		})).Return(nil).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		updated, err := ts.service.CheckInUser(ctx, regID)
		require.NoError(t, err)
		assert.Equal(t, event.RegStatusAttended, updated.Status)
	})

	t.Run("Fail_NotConfirmed", func(t *testing.T) {
		ts := setupAdminService(t)
		reg := &event.Registration{ID: regID, Status: event.RegStatusCancelled}

		ts.rbac.EXPECT().Authorize(ctx, mock.Anything).Return(true, nil).Once()
		ts.repo.EXPECT().GetRegistration(ctx, regID).Return(reg, nil).Once()

		_, err := ts.service.CheckInUser(ctx, regID)
		assert.ErrorIs(t, err, event.ErrInvalidInput)
	})
}
