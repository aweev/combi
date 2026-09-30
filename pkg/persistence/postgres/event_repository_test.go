package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/event"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventRepository(t *testing.T) {
	// Setup DB connection (assumes Docker container or local DB available via standard env vars)
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 1. Resources: Venues & Speakers
	// -------------------------------------------------------------------------
	t.Run("Resources_VenueAndSpeaker", func(t *testing.T) {
		// --- Venue ---
		venueID := uuid.New()
		venue := &event.Venue{
			ID:      venueID,
			Name:    "Convention Center",
			Address: "123 Main St",
			City:    "Tech City",
			Country: "Cloudland",
		}
		require.NoError(t, repo.CreateVenue(ctx, venue))

		gotVenue, err := repo.GetVenue(ctx, venueID)
		require.NoError(t, err)
		assert.Equal(t, "Convention Center", gotVenue.Name)

		// --- Speaker ---
		speakerID := uuid.New()
		bio := "Expert in Go"
		speaker := &event.Speaker{
			ID:    speakerID,
			Name:  "Alice Gopher",
			Title: "Senior Engineer",
			Bio:   &bio,
		}
		require.NoError(t, repo.CreateSpeaker(ctx, speaker))

		gotSpeaker, err := repo.GetSpeaker(ctx, speakerID)
		require.NoError(t, err)
		assert.Equal(t, "Alice Gopher", gotSpeaker.Name)
	})

	// -------------------------------------------------------------------------
	// 2. Events (Core)
	// -------------------------------------------------------------------------
	t.Run("Event_Lifecycle", func(t *testing.T) {
		venueID := uuid.New()
		require.NoError(t, repo.CreateVenue(ctx, &event.Venue{ID: venueID, Name: "V", Address: "A", City: "C", Country: "C"}))

		evtID := uuid.New()
		slug := "go-conf-2024"

		// Setup properties JSON
		props := map[string]interface{}{"allow_waitlist": true}

		evt := &event.Event{
			ID:          evtID,
			CreatedBy:   uuid.New(),
			VenueID:     &venueID,
			Name:        "Go Conference",
			Slug:        slug,
			Description: "The big one",
			Status:      event.StatusDraft,
			Format:      event.FormatPhysical,
			StartTime:   time.Now().UTC().Add(24 * time.Hour),
			EndTime:     time.Now().UTC().Add(48 * time.Hour),
			Timezone:    "UTC",
			Properties:  props,
		}

		// Create
		require.NoError(t, repo.CreateEvent(ctx, evt))

		// Duplicate Slug Check
		dupEvt := *evt
		dupEvt.ID = uuid.New()
		err := repo.CreateEvent(ctx, &dupEvt)
		assert.ErrorIs(t, err, event.ErrEventAlreadyExists)

		// Get
		got, err := repo.GetEventBySlug(ctx, slug)
		require.NoError(t, err)
		assert.Equal(t, evtID, got.ID)
		assert.Equal(t, venueID, *got.VenueID)

		// Verify JSONB Property Unmarshaling
		val, ok := got.Properties["allow_waitlist"].(bool)
		assert.True(t, ok, "property type mismatch")
		assert.True(t, val)

		// Update
		got.Status = event.StatusPublished
		got.Name = "Go Conf Updated"
		require.NoError(t, repo.UpdateEvent(ctx, got))

		updated, err := repo.GetEvent(ctx, evtID)
		require.NoError(t, err)
		assert.Equal(t, event.StatusPublished, updated.Status)
		assert.Equal(t, "Go Conf Updated", updated.Name)

		// List
		list, total, err := repo.ListEvents(ctx, event.EventFilter{
			Status: func() *event.EventStatus { s := event.StatusPublished; return &s }(),
			Limit:  10,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		assert.Equal(t, evtID, list[0].ID)
	})

	// -------------------------------------------------------------------------
	// 3. Tickets & Registration
	// -------------------------------------------------------------------------
	t.Run("Tickets_And_Registration", func(t *testing.T) {
		// Setup Event
		eID := uuid.New()
		repo.CreateEvent(ctx, &event.Event{ID: eID, CreatedBy: uuid.New(), Name: "T-Event", Slug: "t-evt", Status: "PUBLISHED", StartTime: time.Now(), EndTime: time.Now()})

		// --- Create Ticket ---
		tID := uuid.New()
		capacity := 100
		ticket := &event.Ticket{
			ID:       tID,
			EventID:  eID,
			Name:     "VIP",
			Price:    financial.Price{Amount: 5000, Currency: "USD"},
			Capacity: &capacity,
			IsActive: true,
		}
		require.NoError(t, repo.CreateTicket(ctx, ticket))

		// Verify List
		tickets, err := repo.ListTicketsForEvent(ctx, eID)
		require.NoError(t, err)
		assert.Len(t, tickets, 1)

		// --- Registration ---
		userID := uuid.New()
		reg := &event.Registration{
			ID:       uuid.New(),
			UserID:   userID,
			EventID:  eID,
			TicketID: tID,
			Status:   event.RegStatusConfirmed,
		}

		// 1. Create Registration
		require.NoError(t, repo.CreateRegistration(ctx, reg))

		// 2. Increment Ticket Count
		require.NoError(t, repo.IncrementRegistrationsCount(ctx, tID, 1))

		// 3. Verify Unique Constraint (Same User, Same Event)
		dupReg := &event.Registration{
			ID:       uuid.New(),
			UserID:   userID,
			EventID:  eID,
			TicketID: tID,
			Status:   event.RegStatusConfirmed,
		}
		err = repo.CreateRegistration(ctx, dupReg)
		assert.ErrorIs(t, err, event.ErrAlreadyRegistered)

		// 4. Retrieve
		gotReg, err := repo.GetUserRegistrationForEvent(ctx, userID, eID)
		require.NoError(t, err)
		assert.Equal(t, reg.ID, gotReg.ID)

		// 5. Check Ticket Count Updated
		gotTicket, _ := repo.GetTicket(ctx, tID)
		assert.Equal(t, 1, gotTicket.RegistrationsCount)
	})

	// -------------------------------------------------------------------------
	// 4. Schedule & Speakers (JSONB Array Handling)
	// -------------------------------------------------------------------------
	t.Run("Schedule_With_Speakers", func(t *testing.T) {
		eID := uuid.New()
		repo.CreateEvent(ctx, &event.Event{ID: eID, CreatedBy: uuid.New(), Name: "S-Event", Slug: "s-evt", Status: "PUBLISHED", StartTime: time.Now(), EndTime: time.Now()})

		// Create 2 Speakers
		s1 := uuid.New()
		s2 := uuid.New()
		repo.CreateSpeaker(ctx, &event.Speaker{ID: s1, Name: "A", Title: "T"})
		repo.CreateSpeaker(ctx, &event.Speaker{ID: s2, Name: "B", Title: "T"})

		// Create Schedule Item
		itemID := uuid.New()
		item := &event.ScheduleItem{
			ID:         itemID,
			EventID:    eID,
			Title:      "Keynote",
			StartTime:  time.Now(),
			EndTime:    time.Now().Add(time.Hour),
			SpeakerIDs: []uuid.UUID{s1, s2}, // Test Array to JSONB
		}
		require.NoError(t, repo.CreateScheduleItem(ctx, item))

		// Retrieve
		gotItem, err := repo.GetScheduleItem(ctx, itemID)
		require.NoError(t, err)
		assert.Equal(t, "Keynote", gotItem.Title)
		assert.Len(t, gotItem.SpeakerIDs, 2)
		assert.Contains(t, gotItem.SpeakerIDs, s1)
		assert.Contains(t, gotItem.SpeakerIDs, s2)

		// Update (Remove one speaker)
		item.SpeakerIDs = []uuid.UUID{s1}
		require.NoError(t, repo.UpdateScheduleItem(ctx, item))

		gotItemUpdated, _ := repo.GetScheduleItem(ctx, itemID)
		assert.Len(t, gotItemUpdated.SpeakerIDs, 1)
		assert.Equal(t, s1, gotItemUpdated.SpeakerIDs[0])
	})

	// -------------------------------------------------------------------------
	// 5. Transactions
	// -------------------------------------------------------------------------
	t.Run("Transaction_Rollback", func(t *testing.T) {
		testID := uuid.New()

		// Attempt to create a venue within a transaction that fails later
		err := repo.WithTx(ctx, func(txRepo event.Repository) error {
			v := &event.Venue{ID: testID, Name: "Tx Venue", Address: "A", City: "C", Country: "C"}
			if err := txRepo.CreateVenue(ctx, v); err != nil {
				return err
			}
			// Simulate failure
			return assert.AnError
		})

		require.ErrorIs(t, err, assert.AnError)

		// Verify Venue was NOT created
		_, err = repo.GetVenue(ctx, testID)
		assert.ErrorIs(t, err, event.ErrVenueNotFound)
	})

	t.Run("Transaction_Commit", func(t *testing.T) {
		testID := uuid.New()

		err := repo.WithTx(ctx, func(txRepo event.Repository) error {
			v := &event.Venue{ID: testID, Name: "Tx Success", Address: "A", City: "C", Country: "C"}
			return txRepo.CreateVenue(ctx, v)
		})

		require.NoError(t, err)

		// Verify Venue WAS created
		_, err = repo.GetVenue(ctx, testID)
		require.NoError(t, err)
	})
}
