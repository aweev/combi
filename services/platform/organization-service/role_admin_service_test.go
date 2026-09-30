package organization_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/organization"
	org_mocks "github.com/combi/pkg/domain/organization/mocks"
	"github.com/combi/pkg/log"
	orgService "github.com/combi/organization"
)

type roleAdminServiceTestSuite struct {
	repo    *org_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	service *orgService.RoleService
}

func setupRoleAdminServiceTest(t *testing.T) roleAdminServiceTestSuite {
	t.Helper()
	var ts roleAdminServiceTestSuite
	ts.repo = org_mocks.NewMockRepository(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)

	logger := &log.NoOpLogger{}

	// Partial main service for permission checks
	mainSvc := orgService.NewService(
		ts.repo,
		nil,
		nil,
		ts.audit,
		nil,
		nil,
		logger,
	)

	ts.service = orgService.NewRoleService(
		ts.repo,
		ts.audit,
		logger,
		mainSvc,
	)
	return ts
}

func TestRoleAdminService_CreateRole(t *testing.T) {
	ctx := context.Background()
	actor := &auth.Actor{UserID: uuid.New()}
	ctx = auth.NewContextWithActor(ctx, actor)
	orgID := uuid.New()
	roleID := uuid.New()
	member := &organization.Member{UserID: actor.UserID, RoleID: roleID}
	params := organization.CreateRoleParams{Name: "New Role", Permissions: []string{"members:list"}}

	t.Run("should create role successfully with permission", func(t *testing.T) {
		ts := setupRoleAdminServiceTest(t)
		roleWithPerms := &organization.Role{ID: roleID, Permissions: []string{string(organization.PermRolesCreate)}}

		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(roleWithPerms, nil).Once()
		ts.repo.On("CreateRole", ctx, mock.MatchedBy(func(r *organization.Role) bool {
			return r.Name == params.Name && r.OrganizationID == orgID
		})).Return(nil).Once()

		// Async Audit Expectation
		ts.audit.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		role, err := ts.service.CreateRole(ctx, orgID, params)
		require.NoError(t, err)
		assert.Equal(t, params.Name, role.Name)

		wg.Wait() // Wait for async audit
	})

	t.Run("should fail if user lacks permission", func(t *testing.T) {
		ts := setupRoleAdminServiceTest(t)
		roleWithoutPerms := &organization.Role{ID: roleID, Permissions: []string{}}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(roleWithoutPerms, nil).Once()

		_, err := ts.service.CreateRole(ctx, orgID, params)
		require.ErrorIs(t, err, organization.ErrPermissionDenied)
	})
}

func TestRoleAdminService_UpdateRole(t *testing.T) {
	ctx := context.Background()
	actor := &auth.Actor{UserID: uuid.New()}
	ctx = auth.NewContextWithActor(ctx, actor)
	orgID, roleID := uuid.New(), uuid.New()
	member := &organization.Member{UserID: actor.UserID, RoleID: uuid.New()}

	t.Run("should update role successfully", func(t *testing.T) {
		ts := setupRoleAdminServiceTest(t)
		adminRole := &organization.Role{ID: member.RoleID, Permissions: []string{string(organization.PermRolesUpdate)}}
		roleToUpdate := &organization.Role{ID: roleID, OrganizationID: orgID, IsSystemRole: false, Name: "Old Name"}
		newName := "New Name"
		params := organization.UpdateRoleParams{Name: &newName}

		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(adminRole, nil).Once()

		ts.repo.On("WithTx", ctx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(organization.Repository) error)
			ts.repo.On("GetRoleByID", ctx, roleID).Return(roleToUpdate, nil).Once()
			ts.repo.On("UpdateRole", ctx, mock.MatchedBy(func(r *organization.Role) bool {
				return r.Name == newName
			})).Return(nil).Once()
			fn(ts.repo)
		}).Return(nil).Once()

		ts.audit.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		updated, err := ts.service.UpdateRole(ctx, orgID, roleID, params)
		require.NoError(t, err)
		assert.Equal(t, newName, updated.Name)

		wg.Wait()
	})

	t.Run("should fail to update a system role", func(t *testing.T) {
		ts := setupRoleAdminServiceTest(t)
		adminRole := &organization.Role{ID: member.RoleID, Permissions: []string{string(organization.PermRolesUpdate)}}
		systemRole := &organization.Role{ID: roleID, OrganizationID: orgID, IsSystemRole: true}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(adminRole, nil).Once()

		ts.repo.On("WithTx", ctx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(organization.Repository) error)
			ts.repo.On("GetRoleByID", ctx, roleID).Return(systemRole, nil).Once()
			err := fn(ts.repo)
			assert.ErrorIs(t, err, organization.ErrSystemRoleImmutable)
		}).Return(organization.ErrSystemRoleImmutable).Once()

		_, err := ts.service.UpdateRole(ctx, orgID, roleID, organization.UpdateRoleParams{})
		require.ErrorIs(t, err, organization.ErrSystemRoleImmutable)
	})
}

func TestRoleAdminService_DeleteRole(t *testing.T) {
	ctx := context.Background()
	actor := &auth.Actor{UserID: uuid.New()}
	ctx = auth.NewContextWithActor(ctx, actor)
	orgID, roleID := uuid.New(), uuid.New()
	member := &organization.Member{UserID: actor.UserID, RoleID: uuid.New()}

	t.Run("should delete custom role successfully", func(t *testing.T) {
		ts := setupRoleAdminServiceTest(t)
		adminRole := &organization.Role{ID: member.RoleID, Permissions: []string{string(organization.PermRolesDelete)}}
		roleToDelete := &organization.Role{ID: roleID, OrganizationID: orgID, IsSystemRole: false}

		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(adminRole, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(roleToDelete, nil).Once()
		ts.repo.On("DeleteRole", ctx, roleID).Return(nil).Once()

		ts.audit.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		err := ts.service.DeleteRole(ctx, orgID, roleID)
		require.NoError(t, err)

		wg.Wait()
	})
}
