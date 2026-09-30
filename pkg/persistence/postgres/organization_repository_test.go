package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func createSeedUser(t *testing.T, repo *postgres.UserRepository) *user.User {
	t.Helper()
	u := newTestUser(t)
	require.NoError(t, repo.CreateUser(context.Background(), u))
	return u
}

func newTestOrg(t *testing.T, ownerID uuid.UUID) *organization.Organization {
	t.Helper()
	unique := uuid.NewString()[:8]
	return &organization.Organization{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Name:      "Test Org " + unique,
		Slug:      "test-org-" + unique,
		Type:      organization.TypeCompany,
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func newTestRole(t *testing.T, orgID uuid.UUID, name string) *organization.Role {
	return &organization.Role{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Description:    "Test role desc",
		IsSystemRole:   false,
		Permissions:    []string{"read", "write"},
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

// --- Tests ---

func TestOrganizationRepository_CRUD(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	orgRepo := postgres.NewOrganizationRepository(db, &log.NoOpLogger{})

	owner := createSeedUser(t, userRepo)

	t.Run("CreateAndGet", func(t *testing.T) {
		org := newTestOrg(t, owner.ID)
		require.NoError(t, orgRepo.CreateOrganization(ctx, org))

		// Get by ID
		got, err := orgRepo.GetOrganizationByID(ctx, org.ID)
		require.NoError(t, err)
		assert.Equal(t, org.Name, got.Name)

		// Get by Slug
		gotSlug, err := orgRepo.GetOrganizationBySlug(ctx, org.Slug)
		require.NoError(t, err)
		assert.Equal(t, org.ID, gotSlug.ID)
	})

	t.Run("Update", func(t *testing.T) {
		org := newTestOrg(t, owner.ID)
		require.NoError(t, orgRepo.CreateOrganization(ctx, org))

		org.Name = "Updated Name"
		require.NoError(t, orgRepo.UpdateOrganization(ctx, org))

		got, err := orgRepo.GetOrganizationByID(ctx, org.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", got.Name)
		assert.Equal(t, 2, got.Version)
	})

	t.Run("DuplicateSlug", func(t *testing.T) {
		org1 := newTestOrg(t, owner.ID)
		require.NoError(t, orgRepo.CreateOrganization(ctx, org1))

		org2 := newTestOrg(t, owner.ID)
		org2.Slug = org1.Slug // Collision

		err := orgRepo.CreateOrganization(ctx, org2)
		assert.ErrorIs(t, err, organization.ErrOrganizationAlreadyExists)
	})
}

func TestOrganizationRepository_Members(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	orgRepo := postgres.NewOrganizationRepository(db, &log.NoOpLogger{})

	// Setup: Org, Owner, 2nd User, Role
	owner := createSeedUser(t, userRepo)
	memberUser := createSeedUser(t, userRepo)

	org := newTestOrg(t, owner.ID)
	require.NoError(t, orgRepo.CreateOrganization(ctx, org))

	role := newTestRole(t, org.ID, "Member")
	require.NoError(t, orgRepo.CreateRole(ctx, role))

	t.Run("AddMember", func(t *testing.T) {
		m := &organization.Member{
			OrganizationID: org.ID,
			UserID:         memberUser.ID,
			RoleID:         role.ID,
			JoinedAt:       time.Now().UTC(),
		}
		require.NoError(t, orgRepo.CreateMember(ctx, m))

		// Count
		count, err := orgRepo.CountMembers(ctx, org.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Get Details
		details, err := orgRepo.GetMemberWithDetails(ctx, org.ID, memberUser.ID)
		require.NoError(t, err)
		assert.Equal(t, memberUser.Email.String(), details.User.Email.String())
		assert.Equal(t, "Member", details.Role.Name)
	})

	t.Run("ListOrgsForUser", func(t *testing.T) {
		list, err := orgRepo.ListOrganizationsForUser(ctx, memberUser.ID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, org.ID, list[0].ID)
	})

	t.Run("DeleteMember", func(t *testing.T) {
		require.NoError(t, orgRepo.DeleteMember(ctx, org.ID, memberUser.ID))

		_, err := orgRepo.GetMember(ctx, org.ID, memberUser.ID)
		assert.ErrorIs(t, err, organization.ErrMemberNotFound)
	})
}

func TestOrganizationRepository_Roles(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	orgRepo := postgres.NewOrganizationRepository(db, &log.NoOpLogger{})

	owner := createSeedUser(t, userRepo)
	org := newTestOrg(t, owner.ID)
	require.NoError(t, orgRepo.CreateOrganization(ctx, org))

	t.Run("CRUD", func(t *testing.T) {
		role := newTestRole(t, org.ID, "CustomRole")
		require.NoError(t, orgRepo.CreateRole(ctx, role))

		got, err := orgRepo.GetRoleByName(ctx, org.ID, "CustomRole")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"read", "write"}, got.Permissions)

		// Update
		role.Permissions = []string{"read"}
		require.NoError(t, orgRepo.UpdateRole(ctx, role))

		updated, err := orgRepo.GetRoleByID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, updated.Permissions, 1)
	})
}

func TestOrganizationRepository_Invitations(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	orgRepo := postgres.NewOrganizationRepository(db, &log.NoOpLogger{})

	owner := createSeedUser(t, userRepo)
	org := newTestOrg(t, owner.ID)
	require.NoError(t, orgRepo.CreateOrganization(ctx, org))
	role := newTestRole(t, org.ID, "Guest")
	require.NoError(t, orgRepo.CreateRole(ctx, role))

	t.Run("InviteFlow", func(t *testing.T) {
		inviteEmail := "invitee@test.com"
		inv := &organization.Invitation{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			Email:          inviteEmail,
			RoleID:         role.ID,
			InvitedBy:      owner.ID,
			Status:         organization.InvitationStatusPending,
			TokenHash:      "hashed_token_123",
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		require.NoError(t, orgRepo.CreateInvitation(ctx, inv))

		// List
		list, err := orgRepo.ListPendingInvitationsWithDetails(ctx, org.ID)
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, owner.Email.String(), list[0].Inviter.Email.String())

		// Update Status (Accept)
		accepter := createSeedUser(t, userRepo)
		inv.Status = organization.InvitationStatusAccepted
		inv.AcceptedBy = &accepter.ID
		require.NoError(t, orgRepo.UpdateInvitation(ctx, inv))

		// Check gone from pending list
		_, err = orgRepo.GetPendingInvitationsByEmail(ctx, org.ID, inviteEmail)
		assert.ErrorIs(t, err, organization.ErrInvitationNotFound)
	})

	t.Run("GetWithDetails", func(t *testing.T) {
		inv := &organization.Invitation{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			Email:          "details@test.com",
			RoleID:         role.ID,
			InvitedBy:      owner.ID,
			Status:         organization.InvitationStatusPending,
			TokenHash:      "hashed_token_details",
			ExpiresAt:      time.Now().Add(time.Hour),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		require.NoError(t, orgRepo.CreateInvitation(ctx, inv))

		details, err := orgRepo.GetInvitationWithDetails(ctx, inv.ID)
		require.NoError(t, err)
		assert.Equal(t, org.Name, details.Organization.Name)
		assert.Equal(t, "Guest", details.Role.Name)
	})
}

func TestOrganizationRepository_WithTx(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db, &log.NoOpLogger{})
	orgRepo := postgres.NewOrganizationRepository(db, &log.NoOpLogger{})

	owner := createSeedUser(t, userRepo)

	t.Run("AtomicCreate", func(t *testing.T) {
		org := newTestOrg(t, owner.ID)
		role := newTestRole(t, org.ID, "Owner") // Note: Org ID needed before creation usually, but UUIDs generated client side here

		err := orgRepo.WithTx(ctx, func(txRepo organization.Repository) error {
			if err := txRepo.CreateOrganization(ctx, org); err != nil {
				return err
			}
			// If this fails, Org should rollback
			if err := txRepo.CreateRole(ctx, role); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		_, err = orgRepo.GetOrganizationByID(ctx, org.ID)
		require.NoError(t, err)
		_, err = orgRepo.GetRoleByID(ctx, role.ID)
		require.NoError(t, err)
	})
}
