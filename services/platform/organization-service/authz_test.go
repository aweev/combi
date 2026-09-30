package organization

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/organization"
	org_mocks "github.com/combi/pkg/domain/organization/mocks"
	"github.com/combi/pkg/apperror"
)

type authorizerTestSuite struct {
	repo       *org_mocks.MockAuthorizerRepo
	authorizer *Authorizer
}

func setupAuthorizerTest(t *testing.T) authorizerTestSuite {
	t.Helper()
	var ts authorizerTestSuite
	ts.repo = org_mocks.NewMockAuthorizerRepo(t)
	ts.authorizer = NewAuthorizer(ts.repo)
	return ts
}

func TestAuthorizer_Can(t *testing.T) {
	ctx := context.Background()
	actor := &auth.Actor{UserID: uuid.New()}
	orgID := uuid.New()
	roleID := uuid.New()
	member := &organization.Member{UserID: actor.UserID, RoleID: roleID}

	t.Run("should allow access when role has direct permission", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		role := &organization.Role{ID: roleID, Permissions: []string{"members:list"}}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(role, nil).Once()

		allowed, err := ts.authorizer.Can(ctx, actor, orgID, organization.PermMembersList)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should allow access when role has org:admin super-permission", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		role := &organization.Role{ID: roleID, Permissions: []string{string(organization.PermOrganizationAdmin)}}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(role, nil).Once()

		allowed, err := ts.authorizer.Can(ctx, actor, orgID, organization.PermMembersRemove)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("should deny access when role lacks permission", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		role := &organization.Role{ID: roleID, Permissions: []string{"members:list"}}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(role, nil).Once()

		allowed, err := ts.authorizer.Can(ctx, actor, orgID, organization.PermMembersRemove)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	// (This test is no longer needed as the field doesn't exist)
	// t.Run("should deny access when role is inactive", ...)

	t.Run("should return ErrNotOrganizationMember if user is not a member", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		// Mock the repository to return the specific "not found" error.
		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(nil, organization.ErrMemberNotFound).Once()

		allowed, err := ts.authorizer.Can(ctx, actor, orgID, organization.PermMembersList)

		// --- CORRECTED ASSERTIONS ---
		assert.False(t, allowed)
		require.Error(t, err) // 1. First, confirm an error was returned.

		// 2. Use errors.As to check if the error is of type AppError.
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr), "Error should be a standard AppError")

		// 3. Assert on the specific properties of the AppError.
		assert.Equal(t, organization.ErrNotOrganizationMember.Type, appErr.Type, "Error type should be NOT_ORGANIZATION_MEMBER")
		assert.Equal(t, organization.ErrNotOrganizationMember.Code, appErr.Code, "Error code should be 403 Forbidden")

		// 4. (Optional but good) Verify that the original error is still in the chain for logging.
		assert.ErrorIs(t, err, organization.ErrMemberNotFound, "The underlying repo error should be wrapped")
	})

	t.Run("should return ErrUnauthorized if actor is nil", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		_, err := ts.authorizer.Can(ctx, nil, orgID, organization.PermMembersList)
		require.ErrorIs(t, err, auth.ErrUnauthorized)
	})

	t.Run("should propagate error if getting role fails", func(t *testing.T) {
		ts := setupAuthorizerTest(t)
		repoErr := errors.New("database connection lost")
		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, roleID).Return(nil, repoErr).Once()

		_, err := ts.authorizer.Can(ctx, actor, orgID, organization.PermMembersList)
		require.ErrorIs(t, err, repoErr)
	})
}
