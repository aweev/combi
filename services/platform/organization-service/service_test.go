package organization_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/organization"
	org_mocks "github.com/combi/pkg/domain/organization/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/domain/user"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	"github.com/combi/pkg/log"
	security_mocks "github.com/combi/pkg/security/mocks"
	orgService "github.com/combi/organization"
)

type serviceTestSuite struct {
	repo        *org_mocks.MockRepository
	dispatcher  *queue_mocks.MockDispatcher
	audit       *audit_mocks.MockEventRecorder
	tokenGen    *security_mocks.MockTokenGenerator
	userService *user_mocks.MockService
	service     *orgService.Service
}

func setupServiceTest(t *testing.T) serviceTestSuite {
	t.Helper()
	var ts serviceTestSuite
	ts.repo = org_mocks.NewMockRepository(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.audit = audit_mocks.NewMockEventRecorder(t)
	ts.tokenGen = security_mocks.NewMockTokenGenerator(t)
	ts.userService = user_mocks.NewMockService(t)

	ts.service = orgService.NewService(
		ts.repo,
		ts.userService,
		nil,
		ts.audit,
		ts.dispatcher,
		ts.tokenGen,
		&log.NoOpLogger{},
	)
	return ts
}

func contextWithActor(email string) (context.Context, *auth.Actor) {
	actor := &auth.Actor{UserID: uuid.New(), Email: email}
	return auth.NewContextWithActor(context.Background(), actor), actor
}

func TestService_InviteMember(t *testing.T) {
	ctx, actor := contextWithActor("inviter@example.com")
	orgID := uuid.New()
	roleID := uuid.New()

	member := &organization.Member{UserID: actor.UserID, RoleID: uuid.New()}
	roleWithPerms := &organization.Role{ID: member.RoleID, Permissions: []string{string(organization.PermMembersInvite)}}

	params := organization.InviteMemberParams{
		Email:  core.MustNewEmail(t, "new.member@example.com"),
		RoleID: roleID,
	}

	t.Run("should invite member successfully", func(t *testing.T) {
		ts := setupServiceTest(t)
		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(roleWithPerms, nil).Once()
		ts.userService.On("GetPublicProfileByEmail", ctx, params.Email.String()).Return(nil, user.ErrUserNotFound).Once()
		ts.repo.On("GetPendingInvitationsByEmail", ctx, orgID, params.Email.String()).Return(nil, organization.ErrInvitationNotFound).Once()
		ts.tokenGen.On("Generate").Return("raw-token-123", "hashed-token-456", nil).Once()

		ts.repo.On("CreateInvitation", ctx, mock.MatchedBy(func(inv *organization.Invitation) bool {
			return inv.TokenHash == "hashed-token-456" && inv.Email == params.Email.String()
		})).Return(nil).Once()

		ts.dispatcher.On("Dispatch", ctx, organization.JobOrgSendInviteEmail, mock.MatchedBy(func(payload []byte) bool {
			s := string(payload)
			return contains(s, "raw-token-123")
		})).Return(nil, nil).Once()

		ts.audit.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		invitation, err := ts.service.InviteMember(ctx, orgID, params)

		require.NoError(t, err)
		require.NotNil(t, invitation)
		assert.Equal(t, params.Email.String(), invitation.Email)
		assert.Equal(t, "hashed-token-456", invitation.TokenHash)

		wg.Wait()
	})

	t.Run("should still invite if user exists but is not a member of this org", func(t *testing.T) {
		ts := setupServiceTest(t)
		existingUser := &user.PublicProfile{UserID: uuid.New()}
		var wg sync.WaitGroup
		wg.Add(1)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(roleWithPerms, nil).Once()
		ts.userService.On("GetPublicProfileByEmail", ctx, params.Email.String()).Return(existingUser, nil).Once()
		ts.repo.On("GetMember", ctx, orgID, existingUser.UserID).Return(nil, organization.ErrMemberNotFound).Once()
		ts.repo.On("GetPendingInvitationsByEmail", ctx, orgID, params.Email.String()).Return(nil, organization.ErrInvitationNotFound).Once()

		ts.tokenGen.On("Generate").Return("raw", "hash", nil).Once()
		ts.repo.On("CreateInvitation", ctx, mock.Anything).Return(nil).Once()
		ts.dispatcher.On("Dispatch", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()

		ts.audit.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		_, err := ts.service.InviteMember(ctx, orgID, params)
		require.NoError(t, err)
		wg.Wait()
	})

	t.Run("should fail if inviter lacks permission", func(t *testing.T) {
		ts := setupServiceTest(t)
		roleWithoutPerms := &organization.Role{ID: member.RoleID, Permissions: []string{}}

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(roleWithoutPerms, nil).Once()

		_, err := ts.service.InviteMember(ctx, orgID, params)
		require.ErrorIs(t, err, organization.ErrPermissionDenied)
	})

	t.Run("should fail if token generation fails", func(t *testing.T) {
		ts := setupServiceTest(t)

		ts.repo.On("GetMember", ctx, orgID, actor.UserID).Return(member, nil).Once()
		ts.repo.On("GetRoleByID", ctx, member.RoleID).Return(roleWithPerms, nil).Once()
		ts.userService.On("GetPublicProfileByEmail", ctx, params.Email.String()).Return(nil, user.ErrUserNotFound).Once()
		ts.repo.On("GetPendingInvitationsByEmail", ctx, orgID, params.Email.String()).Return(nil, organization.ErrInvitationNotFound).Once()

		ts.tokenGen.On("Generate").Return("", "", errors.New("entropy error")).Once()

		_, err := ts.service.InviteMember(ctx, orgID, params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_GEN_FAILED")
	})
}

// Helper
func contains(s, substr string) bool {
	// Simple containment check
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
