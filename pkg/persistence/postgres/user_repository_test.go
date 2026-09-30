package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* ---------- Test Helpers ---------- */

func newTestUser(t *testing.T) *user.User {
	t.Helper()
	uniqueID := uuid.NewString()[:8]
	email, err := core.NewEmail("user-" + uniqueID + "@example.com")
	require.NoError(t, err)

	return &user.User{
		ID:        uuid.New(),
		FirstName: "Test",
		LastName:  "User",
		Email:     email,
		Status:    user.StatusActive,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

func newTestProfile(t *testing.T, userID uuid.UUID) *user.Profile {
	t.Helper()
	uniqueStr := uuid.NewString()[:8]
	return &user.Profile{
		ID:          userID, // 1:1 relationship
		Handle:      "handle_" + uniqueStr,
		DisplayName: "Display " + uniqueStr,
		Preferences: user.UserPreferences{
			Locale: "en-US",
			Theme:  "dark",
		},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

/* ---------- Main Tests ---------- */

func TestUserRepository_UserCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("CreateAndGet", func(t *testing.T) {
		u := newTestUser(t)
		require.NoError(t, repo.CreateUser(ctx, u))

		// Get by ID
		got, err := repo.GetUser(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
		assert.Equal(t, u.Email, got.Email)
		assert.Equal(t, u.Status, got.Status)

		// Get by Email
		gotByEmail, err := repo.GetUserByEmail(ctx, u.Email.String())
		require.NoError(t, err)
		assert.Equal(t, u.ID, gotByEmail.ID)
	})

	t.Run("DuplicateEmail", func(t *testing.T) {
		u1 := newTestUser(t)
		require.NoError(t, repo.CreateUser(ctx, u1))

		u2 := newTestUser(t)
		u2.Email = u1.Email // Collision

		err := repo.CreateUser(ctx, u2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user already exists")
	})

	t.Run("UpdateUser", func(t *testing.T) {
		u := newTestUser(t)
		require.NoError(t, repo.CreateUser(ctx, u))

		u.FirstName = "UpdatedName"
		u.Status = user.StatusDeactivated
		time.Sleep(time.Millisecond) // Ensure timestamp differs

		require.NoError(t, repo.UpdateUser(ctx, u))

		updated, err := repo.GetUser(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "UpdatedName", updated.FirstName)
		assert.Equal(t, user.StatusDeactivated, updated.Status)
		assert.True(t, updated.UpdatedAt.After(u.CreatedAt))
	})

	t.Run("AnonymizeUser", func(t *testing.T) {
		u := newTestUser(t)
		require.NoError(t, repo.CreateUser(ctx, u))

		require.NoError(t, repo.AnonymizeUser(ctx, u.ID))

		anon, err := repo.GetUser(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, user.StatusAnonymized, anon.Status)
		assert.Equal(t, "Deleted", anon.FirstName)
		assert.NotEqual(t, u.Email, anon.Email)
		assert.Contains(t, anon.Email.String(), "@deleted.local")
	})
}

// TestUserRepository_ListUsers is separated to ensure a clean DB state
func TestUserRepository_ListUsers(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Create a specific set of users for listing
	u1 := newTestUser(t)
	u1.Status = user.StatusPending
	u2 := newTestUser(t)
	u2.Status = user.StatusActive
	u2.FirstName = "SearchTarget"
	u3 := newTestUser(t)
	u3.Status = user.StatusActive

	require.NoError(t, repo.CreateUser(ctx, u1))
	require.NoError(t, repo.CreateUser(ctx, u2))
	require.NoError(t, repo.CreateUser(ctx, u3))

	// Filter by Status
	status := user.StatusActive
	list, total, err := repo.ListUsers(ctx, user.ListUsersFilter{
		Status: &status,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, list, 2)

	// Filter by Search
	list, total, err = repo.ListUsers(ctx, user.ListUsersFilter{
		Search: "SearchTarget",
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, u2.ID, list[0].ID)
}

func TestUserRepository_ProfileCRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Profile requires a parent User
	u := newTestUser(t)
	require.NoError(t, repo.CreateUser(ctx, u))

	t.Run("CreateAndGet", func(t *testing.T) {
		p := newTestProfile(t, u.ID)
		require.NoError(t, repo.CreateProfile(ctx, p))

		got, err := repo.GetProfile(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, p.Handle, got.Handle)
		assert.Equal(t, "en-US", got.Preferences.Locale)
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		p, err := repo.GetProfile(ctx, u.ID)
		require.NoError(t, err)

		p.DisplayName = "New Display Name"
		p.Preferences.Theme = "light"

		require.NoError(t, repo.UpdateProfile(ctx, p))

		updated, err := repo.GetProfile(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Display Name", updated.DisplayName)
		assert.Equal(t, "light", updated.Preferences.Theme)
	})

	t.Run("DuplicateHandle", func(t *testing.T) {
		u2 := newTestUser(t)
		require.NoError(t, repo.CreateUser(ctx, u2))

		existing, _ := repo.GetProfile(ctx, u.ID)

		p2 := newTestProfile(t, u2.ID)
		p2.Handle = existing.Handle // Collision

		err := repo.CreateProfile(ctx, p2)
		assert.ErrorIs(t, err, user.ErrHandleTaken)
	})

	t.Run("GetProfile_NotFound", func(t *testing.T) {
		_, err := repo.GetProfile(ctx, uuid.New())
		assert.ErrorIs(t, err, user.ErrProfileNotFound)
	})
}

func TestUserRepository_Connections(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	// Create 3 users
	uA := newTestUser(t)
	uB := newTestUser(t)
	uC := newTestUser(t)

	require.NoError(t, repo.CreateUser(ctx, uA))
	require.NoError(t, repo.CreateUser(ctx, uB))
	require.NoError(t, repo.CreateUser(ctx, uC))

	t.Run("CreateAndList", func(t *testing.T) {
		// A follows B
		conn1 := &user.Connection{
			ID:           uuid.New(),
			SourceUserID: uA.ID,
			TargetUserID: uB.ID,
			Type:         user.ConnFollowing,
			CreatedAt:    time.Now(),
		}
		// A follows C
		conn2 := &user.Connection{
			ID:           uuid.New(),
			SourceUserID: uA.ID,
			TargetUserID: uC.ID,
			Type:         user.ConnFollowing,
			CreatedAt:    time.Now(),
		}

		require.NoError(t, repo.CreateConnection(ctx, conn1))
		require.NoError(t, repo.CreateConnection(ctx, conn2))

		// List A's following
		list, err := repo.ListConnections(ctx, uA.ID, user.ConnFollowing)
		require.NoError(t, err)
		assert.Len(t, list, 2)

		// Check Target IDs specifically
		ids, err := repo.ListConnectionTargetIDs(ctx, uA.ID, user.ConnFollowing)
		require.NoError(t, err)
		assert.Contains(t, ids, uB.ID)
		assert.Contains(t, ids, uC.ID)
	})

	t.Run("GetAndDelete", func(t *testing.T) {
		// Get specific connection A->B
		conn, err := repo.GetConnection(ctx, uA.ID, uB.ID, user.ConnFollowing)
		require.NoError(t, err)
		assert.Equal(t, uA.ID, conn.SourceUserID)

		// Delete it
		require.NoError(t, repo.DeleteConnection(ctx, conn.ID))

		// Verify deleted
		_, err = repo.GetConnection(ctx, uA.ID, uB.ID, user.ConnFollowing)
		assert.Error(t, err) // Should be not found
	})

	t.Run("Idempotency", func(t *testing.T) {
		// Create A->C again (already exists)
		conn := &user.Connection{
			ID:           uuid.New(),
			SourceUserID: uA.ID,
			TargetUserID: uC.ID,
			Type:         user.ConnFollowing,
			CreatedAt:    time.Now(),
		}
		// Should not error due to ON CONFLICT DO NOTHING
		require.NoError(t, repo.CreateConnection(ctx, conn))
	})
}

func TestUserRepository_PublicProfiles(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	u1 := newTestUser(t)
	u2 := newTestUser(t)
	require.NoError(t, repo.CreateUser(ctx, u1))
	require.NoError(t, repo.CreateUser(ctx, u2))

	// u1 has a profile, u2 does not
	p1 := newTestProfile(t, u1.ID)
	p1.DisplayName = "Public Name"
	require.NoError(t, repo.CreateProfile(ctx, p1))

	t.Run("FetchBatch", func(t *testing.T) {
		mapResult, err := repo.GetPublicProfiles(ctx, []uuid.UUID{u1.ID, u2.ID})
		require.NoError(t, err)
		assert.Len(t, mapResult, 2)

		// Check u1 (Complete profile)
		prof1 := mapResult[u1.ID]
		//assert.Equal(t, u1.Email.String(), prof1.Email)
		assert.NotNil(t, prof1.DisplayName)
		assert.Equal(t, "Public Name", *prof1.DisplayName)

		// Check u2 (User exists, but no profile table entry)
		prof2 := mapResult[u2.ID]
		//assert.Equal(t, u2.Email.String(), prof2.Email)
		assert.Nil(t, prof2.DisplayName) // Should be nil
	})
}

func TestUserRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("RollbackOnError", func(t *testing.T) {
		u := newTestUser(t)

		// Attempt transaction
		err := repo.WithTx(ctx, func(txRepo user.Repository) error {
			if err := txRepo.CreateUser(ctx, u); err != nil {
				return err
			}
			// Force an error
			return assert.AnError
		})
		assert.Error(t, err)

		// Verify User was NOT created
		_, err = repo.GetUser(ctx, u.ID)
		assert.ErrorIs(t, err, user.ErrProfileNotFound)
	})

	t.Run("CommitOnSuccess", func(t *testing.T) {
		u := newTestUser(t)
		p := newTestProfile(t, u.ID)

		err := repo.WithTx(ctx, func(txRepo user.Repository) error {
			if err := txRepo.CreateUser(ctx, u); err != nil {
				return err
			}
			if err := txRepo.CreateProfile(ctx, p); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// Verify both exist
		_, err = repo.GetUser(ctx, u.ID)
		require.NoError(t, err)
		_, err = repo.GetProfile(ctx, u.ID)
		require.NoError(t, err)
	})
}
