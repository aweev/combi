package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers for RBAC ---

// CORRECTED HELPER
func createTestUserForRBAC(t *testing.T, db *sqlx.DB) *user.User {
	t.Helper()
	uniqueID := uuid.NewString()
	ctx := context.Background()

	// --- THE FIX ---
	// The user's ID and the account's ID must be the same to satisfy the foreign key.
	sharedID := uuid.New()
	// A separate UUID for the user_id column in the accounts table.
	userIDForAccountTable := uuid.New()

	// 1. Create the auth.Account record.
	acc := &auth.Account{
		ID:     sharedID,
		UserID: userIDForAccountTable,
		Email:  core.MustNewEmail(t, "user-"+uniqueID+"@example.com"),
		Status: auth.StatusActive,
	}
	_, err := db.NamedExecContext(ctx, `INSERT INTO accounts (id, user_id, email, status) VALUES (:id, :user_id, :email, :status)`, acc)
	require.NoError(t, err)

	// 2. Now create the user.User record using the SAME ID as the account's primary key.
	usr := &user.User{
		ID:        sharedID,
		FirstName: "FirstName",
		LastName:  "LastName",
		Email:     acc.Email,
		Status:    user.StatusActive,
	}
	_, err = db.NamedExecContext(ctx, `INSERT INTO users (id, first_name, last_name, email, status) VALUES (:id, :first_name, :last_name, :email, :status)`, usr)
	require.NoError(t, err)

	return usr
}

func createTestRole(t *testing.T, repo *postgres.RBACRepository, name string, roleType rbac.RoleType) *rbac.Role {
	t.Helper()
	role := &rbac.Role{
		ID:        uuid.New(),
		Name:      name,
		Slug:      name,
		Type:      roleType,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.CreateRole(context.Background(), role)
	require.NoError(t, err)
	return role
}

func createTestPermission(t *testing.T, repo *postgres.RBACRepository, name core.PermissionName) *rbac.Permission {
	t.Helper()
	perm := &rbac.Permission{
		ID:          uuid.New(),
		Name:        name,
		Description: string(name),
		Category:    "testing",
		IsSystem:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := repo.UpsertPermissions(context.Background(), []*rbac.Permission{perm})
	require.NoError(t, err)

	finalPerm, err := repo.GetPermissionByName(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, finalPerm)

	return finalPerm
}

// --- Main Test Suite ---

func TestRBACRepository_Integration(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewRBACRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Permissions", func(t *testing.T) {
		t.Run("Upsert and GetByName", func(t *testing.T) {
			perm1 := &rbac.Permission{
				ID: uuid.New(), Name: "p1", Description: "d1", Category: "c1", IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			require.NoError(t, repo.UpsertPermissions(ctx, []*rbac.Permission{perm1}))

			got, err := repo.GetPermissionByName(ctx, "p1")
			require.NoError(t, err)
			assert.Equal(t, "d1", got.Description)

			perm1.Description = "d1-updated"
			require.NoError(t, repo.UpsertPermissions(ctx, []*rbac.Permission{perm1}))

			got, err = repo.GetPermissionByName(ctx, "p1")
			require.NoError(t, err)
			assert.Equal(t, "d1-updated", got.Description)
		})
	})

	t.Run("Roles", func(t *testing.T) {
		t.Run("Create and Get", func(t *testing.T) {
			role := createTestRole(t, repo, "test-role", rbac.RoleTypeCustom)

			gotByID, err := repo.GetRole(ctx, role.ID)
			require.NoError(t, err)
			assert.Equal(t, role.Name, gotByID.Name)

			gotBySlug, err := repo.GetRoleBySlug(ctx, role.Slug)
			require.NoError(t, err)
			assert.Equal(t, role.ID, gotBySlug.ID)
		})

		t.Run("Delete", func(t *testing.T) {
			role := createTestRole(t, repo, "deletable-role", rbac.RoleTypeCustom)
			require.NoError(t, repo.DeleteRole(ctx, role.ID))

			_, err := repo.GetRole(ctx, role.ID)
			assert.ErrorIs(t, err, rbac.ErrRoleNotFound)
		})

		t.Run("Add and Remove Permissions", func(t *testing.T) {
			role := createTestRole(t, repo, "perm-role", rbac.RoleTypeCustom)
			p1 := createTestPermission(t, repo, "perm1")
			p2 := createTestPermission(t, repo, "perm2")

			require.NoError(t, repo.AddPermissionsToRole(ctx, role.ID, []uuid.UUID{p1.ID, p2.ID}))

			names, err := repo.GetRolePermissionNames(ctx, role.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, []core.PermissionName{"perm1", "perm2"}, names)

			require.NoError(t, repo.RemovePermissionsFromRole(ctx, role.ID, []uuid.UUID{p1.ID}))

			names, err = repo.GetRolePermissionNames(ctx, role.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, []core.PermissionName{"perm2"}, names)
		})
	})

	t.Run("Role Assignments", func(t *testing.T) {
		user1 := createTestUserForRBAC(t, db)
		user2 := createTestUserForRBAC(t, db)
		roleAdmin := createTestRole(t, repo, "admin", rbac.RoleTypeSystem)
		roleMember := createTestRole(t, repo, "member", rbac.RoleTypeCustom)

		t.Run("Assign and Get", func(t *testing.T) {
			assignment := &rbac.RoleAssignment{ID: uuid.New(), UserID: user1.ID, RoleID: roleAdmin.ID}
			require.NoError(t, repo.AssignRoleToUser(ctx, assignment))

			roleIDs, err := repo.GetUserRoleIDs(ctx, user1.ID)
			require.NoError(t, err)
			assert.Equal(t, []uuid.UUID{roleAdmin.ID}, roleIDs)

			roles, err := repo.GetUserRoles(ctx, user1.ID)
			require.NoError(t, err)
			require.Len(t, roles, 1)
			assert.Equal(t, roleAdmin.ID, roles[0].ID)
		})

		t.Run("Revoke", func(t *testing.T) {
			require.NoError(t, repo.RevokeRoleFromUser(ctx, user1.ID, roleAdmin.ID))

			roles, err := repo.GetUserRoles(ctx, user1.ID)
			require.NoError(t, err)
			assert.Empty(t, roles)
		})

		t.Run("Count and List Users in Role", func(t *testing.T) {
			require.NoError(t, repo.AssignRoleToUser(ctx, &rbac.RoleAssignment{ID: uuid.New(), UserID: user1.ID, RoleID: roleMember.ID}))
			require.NoError(t, repo.AssignRoleToUser(ctx, &rbac.RoleAssignment{ID: uuid.New(), UserID: user2.ID, RoleID: roleMember.ID}))

			count, err := repo.CountUsersInRole(ctx, roleMember.ID)
			require.NoError(t, err)
			assert.Equal(t, 2, count)

			usersInRole, err := repo.ListUsersByRoleSlug(ctx, roleMember.Slug)
			require.NoError(t, err)
			require.Len(t, usersInRole, 2)
			userIDsInRole := []uuid.UUID{usersInRole[0].UserID, usersInRole[1].UserID}
			assert.Contains(t, userIDsInRole, user1.ID)
			assert.Contains(t, userIDsInRole, user2.ID)
		})
	})

	t.Run("Effective Permissions", func(t *testing.T) {
		testUser := createTestUserForRBAC(t, db)

		role1 := createTestRole(t, repo, "role1", rbac.RoleTypeCustom)
		role2 := createTestRole(t, repo, "role2", rbac.RoleTypeCustom)
		inactiveRole := createTestRole(t, repo, "inactive", rbac.RoleTypeCustom)
		inactiveRole.IsActive = false
		require.NoError(t, repo.UpdateRole(ctx, inactiveRole))

		p1 := createTestPermission(t, repo, "p1")
		p2 := createTestPermission(t, repo, "p2")
		p3 := createTestPermission(t, repo, "p3")
		p4 := createTestPermission(t, repo, "p4")

		require.NoError(t, repo.AddPermissionsToRole(ctx, role1.ID, []uuid.UUID{p1.ID, p2.ID}))
		require.NoError(t, repo.AddPermissionsToRole(ctx, role2.ID, []uuid.UUID{p2.ID, p3.ID}))
		require.NoError(t, repo.AddPermissionsToRole(ctx, inactiveRole.ID, []uuid.UUID{p4.ID}))

		require.NoError(t, repo.AssignRoleToUser(ctx, &rbac.RoleAssignment{ID: uuid.New(), UserID: testUser.ID, RoleID: role1.ID}))
		require.NoError(t, repo.AssignRoleToUser(ctx, &rbac.RoleAssignment{ID: uuid.New(), UserID: testUser.ID, RoleID: role2.ID}))
		require.NoError(t, repo.AssignRoleToUser(ctx, &rbac.RoleAssignment{ID: uuid.New(), UserID: testUser.ID, RoleID: inactiveRole.ID}))

		effectivePerms, err := repo.GetUserEffectivePermissionNames(ctx, testUser.ID)
		require.NoError(t, err)

		assert.Len(t, effectivePerms, 3)
		assert.ElementsMatch(t, []core.PermissionName{"p1", "p2", "p3"}, effectivePerms)
	})
}
