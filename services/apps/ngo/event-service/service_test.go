// internal/services/event/service_test.go
package event_test

import (
	"context"
	"errors"
	"testing"

	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/event"
	event_mocks "github.com/combi/pkg/domain/event/mocks"
	"github.com/combi/pkg/domain/financial"
	fin_mocks "github.com/combi/pkg/domain/financial/mocks"
	event_service "github.com/combi/event"

	"github.com/combi/pkg/apperror"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type serviceTestSuite struct {
	repo       *event_mocks.MockRepository
	audit      *audit_mocks.MockEventRecorder
	bus        *events_mocks.MockBus
	paymentSvc *fin_mocks.MockService
	service    event.Service
}

func setupService(t *testing.T) serviceTestSuite {
	ts := serviceTestSuite{
		repo:       event_mocks.NewMockRepository(t),
		audit:      audit_mocks.NewMockEventRecorder(t),
		bus:        events_mocks.NewMockBus(t),
		paymentSvc: fin_mocks.NewMockService(t),
	}
	ts.service = event_service.NewService(
		ts.repo, ts.audit, ts.bus, ts.paymentSvc, &log.NoOpLogger{},
	)
	return ts
}

func contextWithUser(userID uuid.UUID) context.Context {
	actor := &auth.Actor{UserID: userID, AccountID: userID}
	return auth.NewContextWithActor(context.Background(), actor)
}

// --- REGISTER TESTS ---

func TestService_RegisterForEvent(t *testing.T) {
	userID := uuid.New()
	ctx := contextWithUser(userID)
	eventID := uuid.New()
	ticketID := uuid.New()

	// Base objects
	activeEvent := &event.Event{ID: eventID, Status: event.StatusPublished, Name: "Con"}
	// Ticket with Price > 0
	paidTicket := &event.Ticket{
		ID: ticketID, EventID: eventID, IsActive: true,
		Price: financial.Price{Amount: 1000, Currency: "USD"},
	}
	// Ticket with Price = 0
	freeTicket := &event.Ticket{
		ID: ticketID, EventID: eventID, IsActive: true,
		Price: financial.Price{Amount: 0, Currency: "USD"},
	}

	t.Run("Success_FreeTicket", func(t *testing.T) {
		ts := setupService(t)

		// 1. Mocks for Validation
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(nil, event.ErrRegistrationNotFound).Once()
		ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(freeTicket, nil).Once() // Free

		// 2. Mocks for DB Transaction
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(event.Repository) error) error {
			// Re-fetch inside Tx
			ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(freeTicket, nil).Once()
			ts.repo.EXPECT().CreateRegistration(ctx, mock.Anything).Return(nil).Once()
			ts.repo.EXPECT().IncrementRegistrationsCount(ctx, ticketID, 1).Return(nil).Once()
			return fn(ts.repo) // Execute logic
		}).Once()

		// 3. Mocks for Events/Audit
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()

		// Act
		reg, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{UserID: userID, EventID: eventID, TicketID: ticketID})

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, reg)
		assert.Nil(t, reg.TransactionID) // Free = No Tx
		assert.Equal(t, event.RegStatusConfirmed, reg.Status)
	})

	t.Run("Success_PaidTicket", func(t *testing.T) {
		ts := setupService(t)
		providerToken := "tok_visa"
		txID := uuid.New()
		successTx := &financial.Transaction{ID: txID, Status: financial.StatusSucceeded}

		// 1. Validation
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(nil, event.ErrRegistrationNotFound).Once()
		ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(paidTicket, nil).Once()

		// 2. Financial Service Call
		ts.paymentSvc.EXPECT().CreateOneTimePayment(ctx, mock.MatchedBy(func(p financial.CreatePaymentParams) bool {
			return p.Amount == 1000 && p.Currency == "USD" && *p.ProviderToken == providerToken
		})).Return(successTx, nil).Once()

		// 3. DB Transaction
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(event.Repository) error) error {
			ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(paidTicket, nil).Once()
			ts.repo.EXPECT().CreateRegistration(ctx, mock.MatchedBy(func(r *event.Registration) bool {
				// Verify Tx ID is linked
				return r.TransactionID != nil && *r.TransactionID == txID
			})).Return(nil).Once()
			ts.repo.EXPECT().IncrementRegistrationsCount(ctx, ticketID, 1).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		// 4. Events
		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()
		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()

		// Act
		reg, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{
			UserID: userID, EventID: eventID, TicketID: ticketID, ProviderToken: &providerToken,
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, txID, *reg.TransactionID)
	})

	t.Run("Fail_PaymentDeclined", func(t *testing.T) {
		ts := setupService(t)
		providerToken := "tok_declined"

		// 1. Validation
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(nil, event.ErrRegistrationNotFound).Once()
		ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(paidTicket, nil).Once()

		// 2. Financial Service Call (Fails)
		// Option A: Returns Err
		// Option B: Returns Transaction with StatusFailed.
		// Logic implemented handles Err. Let's simulate error return.
		ts.paymentSvc.EXPECT().CreateOneTimePayment(ctx, mock.Anything).Return(nil, financial.ErrPaymentDeclined).Once()

		// Act
		_, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{
			UserID: userID, EventID: eventID, TicketID: ticketID, ProviderToken: &providerToken,
		})

		// Assert
		assert.ErrorIs(t, err, financial.ErrPaymentDeclined)
		// DB Transaction should NOT be called
		ts.repo.AssertNotCalled(t, "WithTx")
	})

	t.Run("Fail_PaidButMissingToken", func(t *testing.T) {
		ts := setupService(t)
		// Validation passes
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(nil, event.ErrRegistrationNotFound).Once()
		ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(paidTicket, nil).Once()

		// Act (No Token)
		_, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{
			UserID: userID, EventID: eventID, TicketID: ticketID, // Missing ProviderToken
		})

		// Assert
		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperror.Type("PAYMENT_REQUIRED"), appErr.Type)
	})

	t.Run("Fail_DBError_AfterPayment", func(t *testing.T) {
		ts := setupService(t)
		providerToken := "tok_visa"
		txID := uuid.New()
		successTx := &financial.Transaction{ID: txID, Status: financial.StatusSucceeded}
		dbErr := errors.New("db connection lost")

		// 1. Validation
		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(nil, event.ErrRegistrationNotFound).Once()
		ts.repo.EXPECT().GetTicket(ctx, ticketID).Return(paidTicket, nil).Once()

		// 2. Payment Success
		ts.paymentSvc.EXPECT().CreateOneTimePayment(ctx, mock.Anything).Return(successTx, nil).Once()

		// 3. DB Failure
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).Return(dbErr).Once()

		// Act
		_, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{
			UserID: userID, EventID: eventID, TicketID: ticketID, ProviderToken: &providerToken,
		})

		// Assert
		assert.ErrorIs(t, err, dbErr)
		// In a real production test with full logs, we would verify the CRITICAL error log about Refund Needed.
	})

	t.Run("Fail_AlreadyRegistered", func(t *testing.T) {
		ts := setupService(t)
		existingReg := &event.Registration{ID: uuid.New()}

		ts.repo.EXPECT().GetEvent(ctx, eventID).Return(activeEvent, nil).Once()
		ts.repo.EXPECT().GetUserRegistrationForEvent(ctx, userID, eventID).Return(existingReg, nil).Once()

		_, err := ts.service.RegisterForEvent(ctx, event.RegisterParams{UserID: userID, EventID: eventID, TicketID: ticketID})
		assert.ErrorIs(t, err, event.ErrAlreadyRegistered)
	})
}

// --- WITHDRAW TESTS ---

func TestService_WithdrawRegistration(t *testing.T) {
	userID := uuid.New()
	ctx := contextWithUser(userID)
	regID := uuid.New()
	ticketID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)
		activeReg := &event.Registration{ID: regID, UserID: userID, TicketID: ticketID, Status: event.RegStatusConfirmed}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(event.Repository) error) error {
			ts.repo.EXPECT().GetRegistration(ctx, regID).Return(activeReg, nil).Once()
			ts.repo.EXPECT().UpdateRegistration(ctx, mock.MatchedBy(func(r *event.Registration) bool {
				return r.Status == event.RegStatusCancelled
			})).Return(nil).Once()
			ts.repo.EXPECT().IncrementRegistrationsCount(ctx, ticketID, -1).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		_, err := ts.service.WithdrawRegistration(ctx, regID)
		require.NoError(t, err)
	})

	t.Run("Fail_AccessDenied", func(t *testing.T) {
		ts := setupService(t)
		otherUserReg := &event.Registration{ID: regID, UserID: uuid.New(), TicketID: ticketID}

		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(event.Repository) error) error {
			ts.repo.EXPECT().GetRegistration(ctx, regID).Return(otherUserReg, nil).Once()
			return fn(ts.repo)
		}).Once()

		_, err := ts.service.WithdrawRegistration(ctx, regID)
		assert.ErrorIs(t, err, event.ErrAccessDenied)
	})
}
