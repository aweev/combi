package privacy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/privacy"
	privacy_mocks "github.com/combi/pkg/domain/privacy/mocks"
	"github.com/combi/pkg/log"
	priv_service "github.com/combi/consent/privacy"
)

func TestPrivacyManager_InitiateErasure(t *testing.T) {
	repo := privacy_mocks.NewMockRepository(t)
	manager := priv_service.NewPrivacyManager(repo, &log.NoOpLogger{})
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		// Expect check for existing request -> None found (nil, nil)
		repo.EXPECT().GetPendingRequest(ctx, userID, privacy.TypeErasure).Return(nil, nil).Once()

		// Expect save of new request
		repo.EXPECT().SaveRequest(ctx, mock.MatchedBy(func(r *privacy.Request) bool {
			return r.Status == privacy.StatusPending && r.Type == privacy.TypeErasure
		})).Return(nil).Once()

		req, err := manager.InitiateErasure(ctx, userID, "GDPR")
		require.NoError(t, err)
		assert.NotNil(t, req)
	})

	t.Run("Pending Exists", func(t *testing.T) {
		// FIX: Return a non-nil pointer to simulate an existing request
		existingReq := &privacy.Request{ID: uuid.New(), Status: privacy.StatusPending}

		repo.EXPECT().GetPendingRequest(ctx, userID, privacy.TypeErasure).Return(existingReq, nil).Once()

		_, err := manager.InitiateErasure(ctx, userID, "GDPR")
		assert.ErrorIs(t, err, privacy.ErrRequestPending)
	})
}

func TestPrivacyManager_ProcessErasure(t *testing.T) {
	repo := privacy_mocks.NewMockRepository(t)
	manager := priv_service.NewPrivacyManager(repo, &log.NoOpLogger{})
	ctx := context.Background()
	reqID := uuid.New()
	userID := uuid.New()

	req := &privacy.Request{ID: reqID, UserID: userID, Status: privacy.StatusPending}

	// Mock Anonymizer
	anon := privacy_mocks.NewMockAnonymizer(t)
	manager.RegisterAnonymizer(anon)

	t.Run("Success", func(t *testing.T) {
		repo.EXPECT().GetRequest(ctx, reqID).Return(req, nil).Once()
		repo.EXPECT().SaveRequest(ctx, mock.MatchedBy(func(r *privacy.Request) bool {
			return r.Status == privacy.StatusProcessing
		})).Return(nil).Once()

		// Service call
		anon.EXPECT().AnonymizeUser(ctx, userID).Return(&privacy.ErasureReport{RecordsDeleted: 5}, nil).Once()

		// Final save
		repo.EXPECT().SaveRequest(ctx, mock.MatchedBy(func(r *privacy.Request) bool {
			return r.Status == privacy.StatusCompleted && r.ErasureDetails.RecordsDeleted == 5
		})).Return(nil).Once()

		err := manager.ProcessErasure(ctx, reqID)
		require.NoError(t, err)
	})

	t.Run("Service Failure (Partial)", func(t *testing.T) {
		repo.EXPECT().GetRequest(ctx, reqID).Return(req, nil).Once()
		repo.EXPECT().SaveRequest(ctx, mock.Anything).Return(nil).Once() // Processing

		// Service fails
		anon.EXPECT().AnonymizeUser(ctx, userID).Return(nil, errors.New("svc failed")).Once()

		// Should mark as FAILED but not crash
		repo.EXPECT().SaveRequest(ctx, mock.MatchedBy(func(r *privacy.Request) bool {
			return r.Status == privacy.StatusFailed
		})).Return(nil).Once()

		err := manager.ProcessErasure(ctx, reqID)
		require.NoError(t, err) // Function succeeds, request marked failed in DB
	})
}
