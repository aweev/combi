package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/security"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIsAppError(t *testing.T, err error, expected *apperror.AppError) {
	t.Helper()
	require.Error(t, err, "Expected an error but got none")

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr), "Error should be of type *apperror.AppError")

	assert.Equal(t, expected.Code, appErr.Code, "AppError code mismatch")
	assert.Equal(t, expected.Type, appErr.Type, "AppError type mismatch")
}

func TestAuditRepository_EventCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuditRepository(db)
	ctx := context.Background()

	t.Run("CreateEvent and GetEvent", func(t *testing.T) {
		encCtx := &security.EncryptionContext{Nonce: []byte("nonce123")}
		event := &audit.Event{
			ID:                uuid.New(),
			CreatedAt:         time.Now().UTC().Truncate(time.Microsecond),
			ActorID:           uuid.New(),
			ActorType:         "user",
			Action:            core.EventAction("user.login"),
			Outcome:           audit.OutcomeSuccess,
			Sensitive:         audit.SensitiveData{ActorName: "alice@example.com"},
			EncryptedData:     []byte("cipher"),
			EncryptionContext: encCtx,
		}

		err := repo.CreateEvent(ctx, event)
		require.NoError(t, err)

		got, err := repo.GetEvent(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, event.ID, got.ID)
		assert.Equal(t, "user.login", string(got.Action))
		assert.Equal(t, encCtx.Nonce, got.EncryptionContext.Nonce)
	})

	t.Run("CreateBatchEvents creates multiple events", func(t *testing.T) {
		actorID := uuid.New()
		events := []*audit.Event{
			{
				ID:                uuid.New(),
				CreatedAt:         time.Now().UTC().Truncate(time.Microsecond),
				ActorID:           actorID,
				ActorType:         "user",
				Action:            core.EventAction("file.upload"),
				Outcome:           audit.OutcomeSuccess,
				EncryptedData:     []byte("data1"),
				EncryptionContext: &security.EncryptionContext{Nonce: []byte("nonce1")},
			},
			{
				ID:                uuid.New(),
				CreatedAt:         time.Now().UTC().Add(1 * time.Second).Truncate(time.Microsecond),
				ActorID:           actorID,
				ActorType:         "user",
				Action:            core.EventAction("file.download"),
				Outcome:           audit.OutcomeSuccess,
				EncryptedData:     []byte("data2"),
				EncryptionContext: &security.EncryptionContext{Nonce: []byte("nonce2")},
			},
		}

		err := repo.CreateBatchEvents(ctx, events)
		require.NoError(t, err)

		// Verify both events were created
		for _, evt := range events {
			got, err := repo.GetEvent(ctx, evt.ID)
			require.NoError(t, err)
			assert.Equal(t, evt.ID, got.ID)
		}
	})

	t.Run("GetEvent returns ErrEventNotFound for non-existent ID", func(t *testing.T) {
		_, err := repo.GetEvent(ctx, uuid.New())
		assertIsAppError(t, err, audit.ErrEventNotFound)
	})

	t.Run("CreateEvent with optional fields", func(t *testing.T) {
		targetID := uuid.New()
		targetType := "document"
		reason := "Rate limit exceeded"
		ip := "192.168.1.1"
		ua := "Mozilla/5.0"
		trace := "abc123"

		event := &audit.Event{
			ID:                uuid.New(),
			CreatedAt:         time.Now().UTC().Truncate(time.Microsecond),
			ActorID:           uuid.New(),
			ActorType:         "api_key",
			Action:            core.EventAction("api.call"),
			Outcome:           audit.OutcomeFailed,
			OutcomeReason:     &reason,
			TargetID:          &targetID,
			TargetType:        &targetType,
			IPAddress:         &ip,
			UserAgent:         &ua,
			TraceID:           &trace,
			EncryptedData:     []byte("data"),
			EncryptionContext: &security.EncryptionContext{Nonce: []byte("nonce")},
		}

		err := repo.CreateEvent(ctx, event)
		require.NoError(t, err)

		got, err := repo.GetEvent(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, reason, *got.OutcomeReason)
		assert.Equal(t, targetID, *got.TargetID)
		assert.Equal(t, targetType, *got.TargetType)
		assert.Equal(t, ip, *got.IPAddress)
		assert.Equal(t, ua, *got.UserAgent)
		assert.Equal(t, trace, *got.TraceID)
	})
}

func TestAuditRepository_SearchEvents(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuditRepository(db)
	ctx := context.Background()

	// Seed test data
	actorID1 := uuid.New()
	actorID2 := uuid.New()
	targetID := uuid.New()
	action1 := core.EventAction("user.login")
	action2 := core.EventAction("file.download")

	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	events := []*audit.Event{
		{
			ID:                uuid.New(),
			CreatedAt:         baseTime.Add(-3 * time.Hour),
			ActorID:           actorID1,
			ActorType:         "user",
			Action:            action1,
			Outcome:           audit.OutcomeSuccess,
			EncryptedData:     []byte("data"),
			EncryptionContext: &security.EncryptionContext{Nonce: []byte("n")},
		},
		{
			ID:                uuid.New(),
			CreatedAt:         baseTime.Add(-2 * time.Hour),
			ActorID:           actorID2,
			ActorType:         "user",
			Action:            action2,
			Outcome:           audit.OutcomeSuccess,
			TargetID:          &targetID,
			EncryptedData:     []byte("data"),
			EncryptionContext: &security.EncryptionContext{Nonce: []byte("n")},
		},
		{
			ID:                uuid.New(),
			CreatedAt:         baseTime.Add(-1 * time.Hour),
			ActorID:           actorID1,
			ActorType:         "user",
			Action:            action1,
			Outcome:           audit.OutcomeFailed,
			EncryptedData:     []byte("data"),
			EncryptionContext: &security.EncryptionContext{Nonce: []byte("n")},
		},
	}

	require.NoError(t, repo.CreateBatchEvents(ctx, events))

	t.Run("search with no filters returns all", func(t *testing.T) {
		list, total, err := repo.SearchEvents(ctx, audit.EventFilters{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 3)
		assert.GreaterOrEqual(t, len(list), 3)
	})

	t.Run("filter by ActorID", func(t *testing.T) {
		filters := audit.EventFilters{ActorID: &actorID1}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
		for _, evt := range list {
			assert.Equal(t, actorID1, evt.ActorID)
		}
	})

	t.Run("filter by TargetID", func(t *testing.T) {
		filters := audit.EventFilters{TargetID: &targetID}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, events[1].ID, list[0].ID)
	})

	t.Run("filter by Action", func(t *testing.T) {
		filters := audit.EventFilters{Action: &action1}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, list, 2)
	})

	t.Run("filter by Outcome", func(t *testing.T) {
		outcome := audit.OutcomeFailed
		filters := audit.EventFilters{Outcome: &outcome}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, audit.OutcomeFailed, list[0].Outcome)
	})

	t.Run("filter by date range", func(t *testing.T) {
		startDate := baseTime.Add(-2*time.Hour - 30*time.Minute)
		endDate := baseTime.Add(-1*time.Hour - 30*time.Minute)
		filters := audit.EventFilters{
			StartDate: &startDate,
			EndDate:   &endDate,
		}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, events[1].ID, list[0].ID)
	})

	t.Run("pagination with limit and offset", func(t *testing.T) {
		filters := audit.EventFilters{Limit: 2, Offset: 0}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 3)
		assert.Len(t, list, 2)

		// Test offset
		filters.Offset = 2
		list2, _, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list2), 1)
		// Ensure no overlap
		for _, e1 := range list {
			for _, e2 := range list2 {
				assert.NotEqual(t, e1.ID, e2.ID)
			}
		}
	})

	t.Run("sort order ascending", func(t *testing.T) {
		filters := audit.EventFilters{SortOrder: "asc", Limit: 3}
		list, _, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 3)
		// Verify ascending order
		for i := 1; i < len(list); i++ {
			assert.True(t, list[i].CreatedAt.After(list[i-1].CreatedAt) || list[i].CreatedAt.Equal(list[i-1].CreatedAt))
		}
	})

	t.Run("sort order descending (default)", func(t *testing.T) {
		filters := audit.EventFilters{Limit: 3}
		list, _, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 3)
		// Verify descending order
		for i := 1; i < len(list); i++ {
			assert.True(t, list[i].CreatedAt.Before(list[i-1].CreatedAt) || list[i].CreatedAt.Equal(list[i-1].CreatedAt))
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		outcome := audit.OutcomeSuccess
		filters := audit.EventFilters{
			ActorID: &actorID1,
			Outcome: &outcome,
		}
		list, total, err := repo.SearchEvents(ctx, filters)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
		assert.Equal(t, events[0].ID, list[0].ID)
	})
}

func TestAuditRepository_LegalHoldCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuditRepository(db)
	ctx := context.Background()

	t.Run("Create and Get LegalHold", func(t *testing.T) {
		hold := &audit.LegalHold{
			ID:          uuid.New(),
			Name:        "Investigation Hold",
			Description: "Legal hold for investigation #123",
			HoldSubject: "user_id:" + uuid.New().String(),
			CreatedBy:   uuid.New(),
			IssuedAt:    time.Now().UTC().Truncate(time.Microsecond),
		}

		err := repo.CreateLegalHold(ctx, hold)
		require.NoError(t, err)

		got, err := repo.GetLegalHold(ctx, hold.ID)
		require.NoError(t, err)
		assert.Equal(t, hold.ID, got.ID)
		assert.Equal(t, hold.Name, got.Name)
		assert.Equal(t, hold.HoldSubject, got.HoldSubject)
		assert.Nil(t, got.ReleasedAt)
	})

	t.Run("GetLegalHold returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.GetLegalHold(ctx, uuid.New())
		assertIsAppError(t, err, audit.ErrLegalHoldNotFound)
	})

	t.Run("UpdateLegalHold to release", func(t *testing.T) {
		hold := &audit.LegalHold{
			ID:          uuid.New(),
			Name:        "Active Hold",
			Description: "Hold to be released",
			HoldSubject: "case:456",
			CreatedBy:   uuid.New(),
			IssuedAt:    time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.CreateLegalHold(ctx, hold))

		// Release it
		releasedTime := time.Now().UTC().Truncate(time.Microsecond)
		releasedBy := uuid.New()
		hold.ReleasedAt = &releasedTime
		hold.ReleasedBy = &releasedBy

		err := repo.UpdateLegalHold(ctx, hold)
		require.NoError(t, err)

		got, err := repo.GetLegalHold(ctx, hold.ID)
		require.NoError(t, err)
		assert.NotNil(t, got.ReleasedAt)
		assert.Equal(t, releasedTime, *got.ReleasedAt)
		assert.Equal(t, releasedBy, *got.ReleasedBy)
	})

	t.Run("ListLegalHolds with pagination", func(t *testing.T) {
		// Create multiple holds
		for i := 0; i < 5; i++ {
			hold := &audit.LegalHold{
				ID:          uuid.New(),
				Name:        "Hold " + string(rune('A'+i)),
				HoldSubject: "test:" + uuid.NewString(),
				CreatedBy:   uuid.New(),
				IssuedAt:    time.Now().UTC(),
			}
			require.NoError(t, repo.CreateLegalHold(ctx, hold))
		}

		holds, total, err := repo.ListLegalHolds(ctx, 3, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 5)
		assert.Len(t, holds, 3)

		// Test offset
		holds2, _, err := repo.ListLegalHolds(ctx, 3, 3)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(holds2), 2)
	})
}

func TestAuditRepository_RetentionPolicyCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuditRepository(db)
	ctx := context.Background()

	t.Run("Create and Get RetentionPolicy", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		policy := &audit.RetentionPolicy{
			ID:          uuid.New(),
			Name:        "Auth Logs",
			Description: "Retention for authentication logs",
			DataType:    "auth.*",
			Duration:    365 * 24 * time.Hour, // 1 year
			IsActive:    true,
			CreatedBy:   uuid.New(),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		err := repo.CreateRetentionPolicy(ctx, policy)
		require.NoError(t, err)

		got, err := repo.GetRetentionPolicy(ctx, policy.ID)
		require.NoError(t, err)
		assert.Equal(t, policy.ID, got.ID)
		assert.Equal(t, policy.Name, got.Name)
		assert.Equal(t, 365, int(got.Duration.Hours()/24))
		assert.True(t, got.IsActive)
	})

	t.Run("GetRetentionPolicy returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.GetRetentionPolicy(ctx, uuid.New())
		assertIsAppError(t, err, audit.ErrRetentionPolicyNotFound)
	})

	t.Run("UpdateRetentionPolicy", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		policy := &audit.RetentionPolicy{
			ID:          uuid.New(),
			Name:        "Temp Policy",
			Description: "Temporary policy",
			DataType:    "temp.*",
			Duration:    30 * 24 * time.Hour,
			IsActive:    true,
			CreatedBy:   uuid.New(),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		require.NoError(t, repo.CreateRetentionPolicy(ctx, policy))

		// Update it
		policy.Name = "Updated Policy"
		policy.IsActive = false
		policy.Duration = 60 * 24 * time.Hour

		err := repo.UpdateRetentionPolicy(ctx, policy)
		require.NoError(t, err)

		got, err := repo.GetRetentionPolicy(ctx, policy.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Policy", got.Name)
		assert.False(t, got.IsActive)
		assert.Equal(t, 60, int(got.Duration.Hours()/24))
	})

	t.Run("UpdateRetentionPolicy returns error for non-existent ID", func(t *testing.T) {
		policy := &audit.RetentionPolicy{
			ID:        uuid.New(),
			Name:      "Non-existent",
			DataType:  "test",
			Duration:  30 * 24 * time.Hour,
			CreatedBy: uuid.New(),
		}
		err := repo.UpdateRetentionPolicy(ctx, policy)
		assert.ErrorIs(t, err, audit.ErrRetentionPolicyNotFound)
	})

	t.Run("DeleteRetentionPolicy", func(t *testing.T) {
		policy := &audit.RetentionPolicy{
			ID:          uuid.New(),
			Name:        "To Delete",
			Description: "Will be deleted",
			DataType:    "delete.*",
			Duration:    7 * 24 * time.Hour,
			IsActive:    true,
			CreatedBy:   uuid.New(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(t, repo.CreateRetentionPolicy(ctx, policy))

		err := repo.DeleteRetentionPolicy(ctx, policy.ID)
		require.NoError(t, err)

		_, err = repo.GetRetentionPolicy(ctx, policy.ID)
		assertIsAppError(t, err, audit.ErrRetentionPolicyNotFound)
	})

	t.Run("DeleteRetentionPolicy returns error for non-existent ID", func(t *testing.T) {
		err := repo.DeleteRetentionPolicy(ctx, uuid.New())
		assert.ErrorIs(t, err, audit.ErrRetentionPolicyNotFound)
	})

	t.Run("ListRetentionPolicies with pagination", func(t *testing.T) {
		// Create multiple policies
		for i := 0; i < 5; i++ {
			policy := &audit.RetentionPolicy{
				ID:        uuid.New(),
				Name:      "Policy " + string(rune('A'+i)),
				DataType:  "test.*",
				Duration:  30 * 24 * time.Hour,
				IsActive:  true,
				CreatedBy: uuid.New(),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			require.NoError(t, repo.CreateRetentionPolicy(ctx, policy))
		}

		policies, total, err := repo.ListRetentionPolicies(ctx, 3, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 5)
		assert.Len(t, policies, 3)

		// Verify sorted by name ASC
		for i := 1; i < len(policies); i++ {
			assert.True(t, policies[i].Name >= policies[i-1].Name)
		}
	})
}

func TestAuditRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewAuditRepository(db)
	ctx := context.Background()

	t.Run("transaction commits on success", func(t *testing.T) {
		holdID := uuid.New()
		hold := &audit.LegalHold{
			ID:          holdID,
			Name:        "TX Success",
			HoldSubject: "user:" + uuid.NewString(),
			CreatedBy:   uuid.New(),
			IssuedAt:    time.Now().UTC(),
		}

		err := repo.WithTx(ctx, func(txRepo audit.Repository) error {
			return txRepo.CreateLegalHold(ctx, hold)
		})
		require.NoError(t, err)

		// Verify the hold exists
		got, err := repo.GetLegalHold(ctx, holdID)
		require.NoError(t, err)
		assert.Equal(t, holdID, got.ID)
	})

	t.Run("transaction rolls back on error", func(t *testing.T) {
		holdID := uuid.New()
		hold := &audit.LegalHold{
			ID:          holdID,
			Name:        "TX Rollback",
			HoldSubject: "user:" + uuid.NewString(),
			CreatedBy:   uuid.New(),
			IssuedAt:    time.Now().UTC(),
		}

		err := repo.WithTx(ctx, func(txRepo audit.Repository) error {
			if err := txRepo.CreateLegalHold(ctx, hold); err != nil {
				return err
			}
			// Force rollback
			return errors.New("intentional error")
		})
		require.Error(t, err)
		assert.Equal(t, "intentional error", err.Error())

		// Verify the hold does NOT exist
		_, err = repo.GetLegalHold(ctx, holdID)
		assertIsAppError(t, err, audit.ErrLegalHoldNotFound)
	})

	t.Run("transaction handles panic and rolls back", func(t *testing.T) {
		holdID := uuid.New()
		hold := &audit.LegalHold{
			ID:          holdID,
			Name:        "TX Panic",
			HoldSubject: "user:" + uuid.NewString(),
			CreatedBy:   uuid.New(),
			IssuedAt:    time.Now().UTC(),
		}

		assert.Panics(t, func() {
			_ = repo.WithTx(ctx, func(txRepo audit.Repository) error {
				_ = txRepo.CreateLegalHold(ctx, hold)
				panic("something went wrong")
			})
		})

		// Verify the hold does NOT exist
		_, err := repo.GetLegalHold(ctx, holdID)
		assertIsAppError(t, err, audit.ErrLegalHoldNotFound)
	})

	t.Run("transaction can create multiple entities", func(t *testing.T) {
		eventID := uuid.New()
		holdID := uuid.New()

		err := repo.WithTx(ctx, func(txRepo audit.Repository) error {
			event := &audit.Event{
				ID:                eventID,
				CreatedAt:         time.Now().UTC(),
				ActorID:           uuid.New(),
				ActorType:         "user",
				Action:            core.EventAction("test.action"),
				Outcome:           audit.OutcomeSuccess,
				EncryptedData:     []byte("data"),
				EncryptionContext: &security.EncryptionContext{Nonce: []byte("n")},
			}
			if err := txRepo.CreateEvent(ctx, event); err != nil {
				return err
			}

			hold := &audit.LegalHold{
				ID:          holdID,
				Name:        "Multi Entity TX",
				HoldSubject: "test:" + uuid.NewString(),
				CreatedBy:   uuid.New(),
				IssuedAt:    time.Now().UTC(),
			}
			return txRepo.CreateLegalHold(ctx, hold)
		})
		require.NoError(t, err)

		// Verify both were created
		_, err = repo.GetEvent(ctx, eventID)
		require.NoError(t, err)

		_, err = repo.GetLegalHold(ctx, holdID)
		require.NoError(t, err)
	})

	t.Run("nested transaction returns error", func(t *testing.T) {
		err := repo.WithTx(ctx, func(txRepo audit.Repository) error {
			return txRepo.WithTx(ctx, func(nestedTxRepo audit.Repository) error {
				return nil
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot begin transaction from within another transaction")
	})
}
