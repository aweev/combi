package user_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/domain/rbac"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/domain/user"
	user_mocks "github.com/combi/pkg/domain/user/mocks"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
	userService "github.com/combi/identity/user"
)

// serviceTestSuite holds dependencies for testing the user-facing Service.
type serviceTestSuite struct {
	repo          *user_mocks.MockRepository
	storageSvc    *storage_mocks.MockService
	auditRecorder *audit_mocks.MockEventRecorder
	authz         *rbac_mocks.MockAuthorizationService
	dispatcher    *queue_mocks.MockDispatcher
	service       *userService.Service // Use concrete type to access unexported fields if needed, or interface
}

// setupService initializes a new test suite for the user service.
func setupService(t *testing.T) serviceTestSuite {
	t.Helper()
	var ts serviceTestSuite
	ts.repo = user_mocks.NewMockRepository(t)
	ts.storageSvc = storage_mocks.NewMockService(t)
	ts.auditRecorder = audit_mocks.NewMockEventRecorder(t)
	ts.authz = rbac_mocks.NewMockAuthorizationService(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)

	ts.service = userService.NewService(
		ts.repo,
		ts.storageSvc,
		ts.auditRecorder,
		ts.authz,
		ts.dispatcher,
		&log.NoOpLogger{},
	)
	return ts
}

// Helper to create a context with an authenticated actor.
func contextWithActor() (context.Context, *auth.Actor) {
	actor := &auth.Actor{
		AccountID: uuid.New(),
		UserID:    uuid.New(),
		Email:     "actor@example.com",
	}
	return auth.NewContextWithActor(context.Background(), actor), actor
}

func TestUserService_CreateUser(t *testing.T) {
	t.Run("should create user and profile transactionally", func(t *testing.T) {
		ts := setupService(t)
		params := user.CreateUserParams{UserID: uuid.New(), Email: core.MustNewEmail(t, "new@user.com")}

		var wg sync.WaitGroup
		wg.Add(1)

		// Mock the transactional block.
		ts.repo.On("WithTx", mock.Anything, mock.AnythingOfType("func(user.Repository) error")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(user.Repository) error)
				ts.repo.On("CreateUser", mock.Anything, mock.Anything).Return(nil).Once()
				ts.repo.On("CreateProfile", mock.Anything, mock.Anything).Return(nil).Once()
				err := fn(ts.repo)
				require.NoError(t, err)
			}).Return(nil).Once()

		ts.auditRecorder.On("Record", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { wg.Done() }).
			Return(nil).Once()

		createdUser, err := ts.service.CreateUser(context.Background(), params)

		require.NoError(t, err)
		assert.Equal(t, params.UserID, createdUser.ID)
		assert.Equal(t, user.StatusPending, createdUser.Status)
		wg.Wait()
	})

	t.Run("should return error if repository transaction fails", func(t *testing.T) {
		ts := setupService(t)
		params := user.CreateUserParams{UserID: uuid.New(), Email: core.MustNewEmail(t, "new@user.com")}
		dbErr := errors.New("database error")

		ts.repo.On("WithTx", mock.Anything, mock.Anything).Return(dbErr).Once()

		_, err := ts.service.CreateUser(context.Background(), params)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))
		// CAST STRING TO TYPE
		assert.Equal(t, apperror.Type("USER_CREATION_FAILED"), appErr.Type)
	})
}

func TestUserService_UpdateProfile(t *testing.T) {
	userCtx, userActor := contextWithActor()
	otherUserID := uuid.New()
	displayName := "Johnny"
	params := user.UpdateProfileParams{DisplayName: &displayName}

	t.Run("should allow user to update their own profile", func(t *testing.T) {
		ts := setupService(t)
		existingProfile := &user.Profile{ID: userActor.UserID, DisplayName: "John"}

		ts.authz.On("Authorize", userCtx, mock.MatchedBy(func(check rbac.PermissionCheck) bool {
			return check.PermissionName == user.PermUserProfileUpdateSelf
		})).Return(true, nil).Once()

		ts.repo.On("WithTx", userCtx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(user.Repository) error)
			ts.repo.On("GetProfile", userCtx, userActor.UserID).Return(existingProfile, nil).Once()
			ts.repo.On("UpdateProfile", userCtx, mock.Anything).Return(nil).Once()
			fn(ts.repo)
		}).Return(nil).Once()

		ts.auditRecorder.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		profile, err := ts.service.UpdateProfile(userCtx, userActor.UserID, userActor.UserID, params)
		require.NoError(t, err)
		assert.Equal(t, displayName, profile.DisplayName)
	})

	t.Run("should deny user from updating another's profile without permission", func(t *testing.T) {
		ts := setupService(t)

		// Authorize: check for the 'any' permission, which now fails.
		ts.authz.On("Authorize", userCtx, mock.MatchedBy(func(check rbac.PermissionCheck) bool {
			return check.PermissionName == user.PermUserManage
		})).Return(false, nil).Once()

		_, err := ts.service.UpdateProfile(userCtx, userActor.UserID, otherUserID, params)
		require.ErrorIs(t, err, user.ErrForbidden)
	})
}

func TestUserService_SetAvatar(t *testing.T) {
	userCtx, userActor := contextWithActor()
	targetUserID := userActor.UserID
	fileID := uuid.New()
	imageFile := &storage.File{ID: fileID, OwnerID: targetUserID, Category: storage.CategoryImage}

	t.Run("should successfully set avatar for self", func(t *testing.T) {
		ts := setupService(t)

		ts.authz.On("Authorize", mock.Anything, mock.Anything).Return(true, nil).Once()
		ts.storageSvc.On("GetFile", userCtx, fileID).Return(imageFile, nil).Once()

		// Mock the transaction
		ts.repo.On("WithTx", userCtx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(user.Repository) error)
			ts.repo.On("GetProfile", userCtx, targetUserID).Return(&user.Profile{ID: targetUserID}, nil)
			ts.repo.On("UpdateProfile", userCtx, mock.Anything).Return(nil)
			fn(ts.repo)
		}).Return(nil).Once()

		ts.auditRecorder.On("Record", mock.Anything, mock.Anything).Return(nil).Once()

		err := ts.service.SetAvatar(userCtx, targetUserID, fileID)
		require.NoError(t, err)
	})
}
func TestUserService_AnonymizeUserData(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("should anonymize user, profile, and trigger file deletions", func(t *testing.T) {
		ts := setupService(t)

		// Mock Transaction
		ts.repo.On("WithTx", ctx, mock.AnythingOfType("func(user.Repository) error")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(user.Repository) error)

				ts.repo.On("AnonymizeUser", ctx, userID).Return(nil).Once()

				err := fn(ts.repo)
				require.NoError(t, err)
			}).Return(nil).Once()

		// Fix: Expect Dispatch of cleanup job instead of direct deletion
		ts.dispatcher.On("Dispatch",
			ctx,
			user.JobUserFileCleanup,
			mock.Anything, // Payload
			mock.Anything, // Options
		).Return(nil, nil).Once()

		// Act
		report, err := ts.service.AnonymizeUser(ctx, userID)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.Equal(t, int64(1), report.RecordsAnonymized)
	})

	t.Run("should return error if database transaction fails", func(t *testing.T) {
		ts := setupService(t)
		dbErr := errors.New("db transaction failed")

		ts.repo.On("WithTx", ctx, mock.AnythingOfType("func(user.Repository) error")).Return(dbErr).Once()

		_, err := ts.service.AnonymizeUser(ctx, userID)

		require.Error(t, err)
		var appErr *apperror.AppError
		require.True(t, errors.As(err, &appErr))

		assert.Equal(t, apperror.Type("ANONYMIZATION_FAILED"), appErr.Type)

		ts.storageSvc.AssertNotCalled(t, "ListFiles")
	})
}

func TestUserService_GetPublicProfiles(t *testing.T) {
	ctx := context.Background()
	user1ID, user2ID := uuid.New(), uuid.New()
	avatar1ID, avatar2ID := uuid.New(), uuid.New()
	profile1 := user.PublicProfile{UserID: user1ID, AvatarFileID: &avatar1ID}
	profile2 := user.PublicProfile{UserID: user2ID, AvatarFileID: &avatar2ID}
	signedURL1 := "https://signed.url/avatar1.jpg"
	signedURL2 := "https://signed.url/avatar2.jpg"

	t.Run("should return public profiles and enrich with avatar URLs concurrently", func(t *testing.T) {
		ts := setupService(t)
		userIDs := []uuid.UUID{user1ID, user2ID}

		ts.repo.On("GetPublicProfiles", ctx, userIDs).Return(map[uuid.UUID]user.PublicProfile{
			user1ID: profile1,
			user2ID: profile2,
		}, nil).Once()

		// Set up expectations for concurrent calls
		ts.storageSvc.On("GetDownloadURL", mock.Anything, avatar1ID, 1*time.Hour).
			Return(&storage.SignedURL{URL: signedURL1}, nil)

		ts.storageSvc.On("GetDownloadURL", mock.Anything, avatar2ID, 1*time.Hour).
			Return(&storage.SignedURL{URL: signedURL2}, nil)

		profiles, err := ts.service.GetPublicProfiles(ctx, userIDs)

		require.NoError(t, err)
		require.Len(t, profiles, 2)

		p1 := profiles[user1ID]
		p2 := profiles[user2ID]

		require.NotNil(t, p1.AvatarURL)
		assert.Equal(t, signedURL1, *p1.AvatarURL)
		require.NotNil(t, p2.AvatarURL)
		assert.Equal(t, signedURL2, *p2.AvatarURL)
	})
}

func TestUserService_ListConnections(t *testing.T) {
	ctx, actor := contextWithActor()
	target1ID, target2ID := uuid.New(), uuid.New()
	targetIDs := []uuid.UUID{target1ID, target2ID}
	publicProfiles := map[uuid.UUID]user.PublicProfile{
		target1ID: {UserID: target1ID},
		target2ID: {UserID: target2ID},
	}

	t.Run("should list connections and return public profiles", func(t *testing.T) {
		ts := setupService(t)

		// We mock ListConnectionTargetIDs because ListConnections calls it.
		ts.repo.On("ListConnectionTargetIDs", ctx, actor.UserID, user.ConnFollowing).Return(targetIDs, nil).Once()

		// The service then calls GetPublicProfiles, which we also mock.
		ts.repo.On("GetPublicProfiles", ctx, targetIDs).Return(publicProfiles, nil).Once()

		users, err := ts.service.ListConnections(ctx, actor.UserID, user.ConnFollowing)

		require.NoError(t, err)
		require.Len(t, users, 2)

		// Assertion Update: Check based on UserID since PublicProfile doesn't embed User directly in this test map setup
		userMap := make(map[uuid.UUID]user.PublicProfile)
		for _, u := range users {
			userMap[u.UserID] = u
		}
		assert.Contains(t, userMap, target1ID)
		assert.Contains(t, userMap, target2ID)
	})
}
