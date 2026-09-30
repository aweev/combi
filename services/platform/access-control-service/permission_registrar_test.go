// Ported from api/internal/services/rbac/permission_registrar_test.go.
package acl

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/log"
)

// registrarTestSuite holds the mocked repository and the registrar under test.
type registrarTestSuite struct {
	repo       *rbac_mocks.MockPermissionRepository
	registrar  *PermissionRegistrar
	def1, def2 rbac.PermissionDefinition
}

// setupRegistrar initializes a new test suite.
func setupRegistrar(t *testing.T) registrarTestSuite {
	t.Helper()
	var ts registrarTestSuite
	ts.repo = rbac_mocks.NewMockPermissionRepository(t)
	ts.registrar = NewPermissionRegistrar(ts.repo, &log.NoOpLogger{})

	// Pre-define some permissions for use in tests.
	ts.def1 = rbac.PermissionDefinition{Name: "users:read", Description: "Read user data", Category: "Users"}
	ts.def2 = rbac.PermissionDefinition{Name: "users:write", Description: "Write user data", Category: "Users"}

	return ts
}

func TestPermissionRegistrar_RegisterAndGet(t *testing.T) {
	ts := setupRegistrar(t)

	// Register two definitions.
	ts.registrar.Register(ts.def1)
	ts.registrar.Register(ts.def2)

	defs := ts.registrar.GetDefinitions()
	require.Len(t, defs, 2)

	// Check that both definitions are present (order is not guaranteed).
	defMap := make(map[core.PermissionName]rbac.PermissionDefinition)
	for _, d := range defs {
		defMap[d.Name] = d
	}
	assert.Equal(t, ts.def1, defMap[ts.def1.Name])
	assert.Equal(t, ts.def2, defMap[ts.def2.Name])
}

func TestPermissionRegistrar_SyncWithDataStore(t *testing.T) {
	ctx := context.Background()

	t.Run("should create new permissions that are not in the database", func(t *testing.T) {
		ts := setupRegistrar(t)
		ts.registrar.Register(ts.def1)
		ts.registrar.Register(ts.def2)

		// --- Arrange Mocks ---
		// 1. Repo returns an empty list of existing permissions.
		ts.repo.On("ListPermissions", ctx, mock.AnythingOfType("rbac.PermissionFilters")).Return([]*rbac.Permission{}, 0, nil).Once()

		// 2. Expect UpsertPermissions to be called with two new permissions.
		ts.repo.On("UpsertPermissions", ctx, mock.MatchedBy(func(perms []*rbac.Permission) bool {
			return len(perms) == 2 && perms[0].IsSystem == true && perms[1].IsSystem == true
		})).Return(nil).Once()

		// --- Act ---
		err := ts.registrar.SyncWithDataStore(ctx)

		// --- Assert ---
		require.NoError(t, err)
		// The mock framework will verify the calls were made as expected.
	})

	t.Run("should update existing permissions with changed descriptions or categories", func(t *testing.T) {
		ts := setupRegistrar(t)
		// New code definitions
		ts.registrar.Register(rbac.PermissionDefinition{Name: "users:read", Description: "NEW Read Desc", Category: "Users"})
		ts.registrar.Register(rbac.PermissionDefinition{Name: "users:write", Description: "Write user data", Category: "NEW Users Category"})

		// --- Arrange Mocks ---
		// DB has the old versions
		existingPerms := []*rbac.Permission{
			{ID: uuid.New(), Name: "users:read", Description: "OLD Read Desc", Category: "Users", IsSystem: true},
			{ID: uuid.New(), Name: "users:write", Description: "Write user data", Category: "Users", IsSystem: true},
		}
		ts.repo.On("ListPermissions", ctx, mock.Anything).Return(existingPerms, 2, nil).Once()

		// Expect UpsertPermissions to be called with the two updated permissions.
		ts.repo.On("UpsertPermissions", ctx, mock.MatchedBy(func(perms []*rbac.Permission) bool {
			if len(perms) != 2 {
				return false
			}
			byName := make(map[core.PermissionName]*rbac.Permission)
			for _, p := range perms {
				byName[p.Name] = p
			}
			return byName["users:read"].Description == "NEW Read Desc" &&
				byName["users:write"].Category == "NEW Users Category"
		})).Return(nil).Once()

		err := ts.registrar.SyncWithDataStore(ctx)
		require.NoError(t, err)
	})

	t.Run("should not change orphaned non-system permissions", func(t *testing.T) {
		ts := setupRegistrar(t)
		ts.registrar.Register(ts.def1) // users:read

		// --- Arrange Mocks ---
		// DB contains a custom (non-system) permission that isn't in the code.
		customPerm := &rbac.Permission{ID: uuid.New(), Name: "custom:perm", Description: "Custom perm", Category: "Custom", IsSystem: false}
		existingPerms := []*rbac.Permission{customPerm}
		ts.repo.On("ListPermissions", ctx, mock.Anything).Return(existingPerms, 1, nil).Once()

		// Expect a new permission to be created.
		ts.repo.On("UpsertPermissions", ctx, mock.AnythingOfType("[]*rbac.Permission")).Return(nil).Once()

		// --- Act ---
		err := ts.registrar.SyncWithDataStore(ctx)
		require.NoError(t, err)

		// CRITICAL: Assert that UpdatePermission was NOT called for the custom permission.
		ts.repo.AssertNotCalled(t, "UpdatePermission")
	})

	t.Run("should handle a mix of create, update, and no-change", func(t *testing.T) {
		ts := setupRegistrar(t)

		// Code definitions
		ts.registrar.Register(rbac.PermissionDefinition{Name: "users:read", Description: "Read user data", Category: "Users"})   // No change
		ts.registrar.Register(rbac.PermissionDefinition{Name: "users:write", Description: "NEW Write Desc", Category: "Users"})  // Update
		ts.registrar.Register(rbac.PermissionDefinition{Name: "posts:create", Description: "Create posts", Category: "Content"}) // Create

		// --- Arrange Mocks ---
		existingPerms := []*rbac.Permission{
			{Name: "users:read", Description: "Read user data", Category: "Users", IsSystem: true},  // Matches code
			{Name: "users:write", Description: "OLD Write Desc", Category: "Users", IsSystem: true}, // Will be updated
			{Name: "orphaned:perm", Description: "Old perm", Category: "Legacy", IsSystem: true},    // Will be orphaned
		}
		ts.repo.On("ListPermissions", ctx, mock.Anything).Return(existingPerms, 3, nil).Once()

		// Expect UpsertPermissions to be called with exactly 2 permissions: the updated and the new one.
		ts.repo.On("UpsertPermissions", ctx, mock.MatchedBy(func(perms []*rbac.Permission) bool {
			if len(perms) != 2 {
				return false
			}
			permMap := make(map[core.PermissionName]*rbac.Permission)
			for _, p := range perms {
				permMap[p.Name] = p
			}
			_, createOk := permMap["posts:create"]
			updatePerm, updateOk := permMap["users:write"]
			return createOk && updateOk && updatePerm.Description == "NEW Write Desc"
		})).Return(nil).Once()

		// Expect UpdatePermission for the orphaned one
		ts.repo.On("UpdatePermission", ctx, mock.MatchedBy(func(p *rbac.Permission) bool {
			return p.Name == "orphaned:perm" && !p.IsSystem
		})).Return(nil).Once()

		// --- Act ---
		err := ts.registrar.SyncWithDataStore(ctx)
		require.NoError(t, err)
	})
}
