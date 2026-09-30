// Ported from api/internal/services/audit/compliance_service_test.go.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/combi/pkg/domain/audit"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/privacy"
	privacy_mocks "github.com/combi/pkg/domain/privacy/mocks"
	"github.com/combi/pkg/domain/queue"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/log"
)

// complianceServiceTestSuite encapsulates all dependencies for the service tests.
type complianceServiceTestSuite struct {
	legalHoldRepo *audit_mocks.MockLegalHoldRepository
	retentionRepo *audit_mocks.MockRetentionPolicyRepository
	dispatcher    *queue_mocks.MockDispatcher
	auditRecorder *audit_mocks.MockEventRecorder
	privacy       *privacy_mocks.MockManager
	service       audit.ComplianceService
}

// setupComplianceService creates a new test suite with fresh mocks for each test.
func setupComplianceService(t *testing.T) complianceServiceTestSuite {
	var ts complianceServiceTestSuite
	ts.legalHoldRepo = audit_mocks.NewMockLegalHoldRepository(t)
	ts.retentionRepo = audit_mocks.NewMockRetentionPolicyRepository(t)
	ts.dispatcher = queue_mocks.NewMockDispatcher(t)
	ts.auditRecorder = audit_mocks.NewMockEventRecorder(t)
	ts.privacy = privacy_mocks.NewMockManager(t)

	ts.service = NewComplianceService(
		ts.legalHoldRepo,
		ts.retentionRepo,
		ts.dispatcher,
		ts.auditRecorder,
		ts.privacy,
		&log.NoOpLogger{},
	)
	return ts
}

// createActorContext is a helper to create a context with a test actor.
func createActorContext() (context.Context, *auth.Actor) {
	actor := &auth.Actor{AccountID: uuid.New()}
	ctx := auth.NewContextWithActor(context.Background(), actor)
	return ctx, actor
}

func TestComplianceService_ApplyLegalHold(t *testing.T) {
	t.Run("should apply legal hold successfully", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, actor := createActorContext()

		params := audit.ApplyLegalHoldParams{
			Name:        "Test Hold",
			HoldSubject: "user_id:" + uuid.NewString(),
		}

		ts.legalHoldRepo.EXPECT().
			CreateLegalHold(ctx, mock.MatchedBy(func(h *audit.LegalHold) bool {
				return h.Name == params.Name &&
					h.HoldSubject == params.HoldSubject &&
					h.CreatedBy == actor.AccountID
			})).
			Return(nil).
			Once()

		ts.auditRecorder.EXPECT().
			Record(context.Background(), mock.AnythingOfType("audit.Event")).
			Return(nil).
			Once()

		hold, err := ts.service.ApplyLegalHold(ctx, params)

		require.NoError(t, err)
		assert.NotNil(t, hold)
		assert.Equal(t, params.Name, hold.Name)
	})

	t.Run("should return error when not authenticated", func(t *testing.T) {
		ts := setupComplianceService(t)
		_, err := ts.service.ApplyLegalHold(context.Background(), audit.ApplyLegalHoldParams{})
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})

	t.Run("should return repository error on failure", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, _ := createActorContext()
		repoErr := errors.New("database error")

		ts.legalHoldRepo.EXPECT().
			CreateLegalHold(ctx, mock.Anything).
			Return(repoErr).
			Once()

		_, err := ts.service.ApplyLegalHold(ctx, audit.ApplyLegalHoldParams{})
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestComplianceService_ReleaseLegalHold(t *testing.T) {
	holdID := uuid.New()

	t.Run("should release an active legal hold successfully", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, actor := createActorContext()

		existingHold := &audit.LegalHold{ID: holdID, Name: "Active Hold", ReleasedAt: nil}

		ts.legalHoldRepo.EXPECT().
			GetLegalHold(ctx, holdID).
			Return(existingHold, nil).
			Once()

		ts.legalHoldRepo.EXPECT().
			UpdateLegalHold(ctx, mock.MatchedBy(func(h *audit.LegalHold) bool {
				return h.ID == holdID &&
					h.ReleasedAt != nil &&
					h.ReleasedBy != nil &&
					*h.ReleasedBy == actor.AccountID
			})).
			Return(nil).
			Once()

		ts.auditRecorder.EXPECT().
			Record(context.Background(), mock.Anything).
			Return(nil).
			Once()

		hold, err := ts.service.ReleaseLegalHold(ctx, holdID)

		require.NoError(t, err)
		assert.NotNil(t, hold.ReleasedAt)
		assert.Equal(t, actor.AccountID, *hold.ReleasedBy)
	})

	t.Run("should be idempotent and not update if already released", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, _ := createActorContext()

		releasedTime := time.Now().Add(-1 * time.Hour).UTC()
		existingHold := &audit.LegalHold{
			ID:         holdID,
			Name:       "Released Hold",
			ReleasedAt: &releasedTime,
		}

		ts.legalHoldRepo.EXPECT().
			GetLegalHold(ctx, holdID).
			Return(existingHold, nil).
			Once()

		hold, err := ts.service.ReleaseLegalHold(ctx, holdID)

		require.NoError(t, err)
		assert.Equal(t, releasedTime, *hold.ReleasedAt)
	})

	t.Run("should return error if hold not found", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, _ := createActorContext()

		ts.legalHoldRepo.EXPECT().
			GetLegalHold(ctx, holdID).
			Return(nil, audit.ErrLegalHoldNotFound).
			Once()

		_, err := ts.service.ReleaseLegalHold(ctx, holdID)
		assert.ErrorIs(t, err, audit.ErrLegalHoldNotFound)
	})
}

func TestComplianceService_DefineRetentionPolicy(t *testing.T) {
	t.Run("should define retention policy successfully", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, actor := createActorContext()

		params := audit.DefineRetentionPolicyParams{
			Name:     "Test Policy",
			DataType: "auth.*",
			Duration: 365 * 24 * time.Hour,
			IsActive: true,
		}

		ts.retentionRepo.EXPECT().
			CreateRetentionPolicy(ctx, mock.MatchedBy(func(p *audit.RetentionPolicy) bool {
				return p.Name == params.Name && p.CreatedBy == actor.AccountID
			})).
			Return(nil).
			Once()

		ts.auditRecorder.EXPECT().
			Record(mock.Anything, mock.Anything).
			Return(nil).
			Once()

		policy, err := ts.service.DefineRetentionPolicy(ctx, params)

		require.NoError(t, err)
		assert.NotNil(t, policy)
		assert.Equal(t, params.Name, policy.Name)
	})
}

func TestComplianceService_AnonymizeUser(t *testing.T) {
	userID := uuid.New()

	t.Run("should dispatch anonymization job successfully", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, actor := createActorContext()
		expectedJob := &queue.Job{ID: uuid.New()}

		ts.privacy.EXPECT().InitiateErasure(ctx, userID, mock.Anything).
			Return(&privacy.Request{ID: uuid.New()}, nil).
			Once()

		ts.dispatcher.EXPECT().
			Dispatch(
				ctx,
				audit.JobComplianceAnonymizeUser,
				mock.AnythingOfType("json.RawMessage"),
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).
			Return(expectedJob, nil).
			Once()

		ts.auditRecorder.EXPECT().
			Record(context.Background(), mock.MatchedBy(func(e audit.Event) bool {
				var meta map[string]string
				err := json.Unmarshal(e.Sensitive.Metadata, &meta)
				return err == nil &&
					meta["job_id"] == expectedJob.ID.String() &&
					e.ActorID == actor.AccountID
			})).
			Return(nil).
			Once()

		job, err := ts.service.AnonymizeUser(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedJob, job)
	})

	t.Run("should return nil job when a duplicate request is detected", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, actor := createActorContext()

		ts.privacy.EXPECT().InitiateErasure(ctx, userID, mock.Anything).
			Return(&privacy.Request{ID: uuid.New()}, nil).
			Once()

		ts.dispatcher.EXPECT().
			Dispatch(
				ctx,
				audit.JobComplianceAnonymizeUser,
				mock.AnythingOfType("json.RawMessage"),
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).
			Return(nil, nil).
			Once()

		ts.auditRecorder.EXPECT().
			Record(context.Background(), mock.MatchedBy(func(e audit.Event) bool {
				var meta map[string]string
				err := json.Unmarshal(e.Sensitive.Metadata, &meta)
				return err == nil &&
					e.ActorID == actor.AccountID &&
					meta["note"] == "Duplicate request ignored"
			})).
			Return(nil).
			Once()

		job, err := ts.service.AnonymizeUser(ctx, userID)
		require.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("should return error if dispatcher fails", func(t *testing.T) {
		ts := setupComplianceService(t)
		ctx, _ := createActorContext()
		dispatcherErr := errors.New("queue is down")

		ts.privacy.EXPECT().InitiateErasure(ctx, userID, mock.Anything).
			Return(&privacy.Request{ID: uuid.New()}, nil).
			Once()

		ts.dispatcher.EXPECT().
			Dispatch(
				ctx,
				audit.JobComplianceAnonymizeUser,
				mock.AnythingOfType("json.RawMessage"),
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).
			Return(nil, dispatcherErr).
			Once()

		_, err := ts.service.AnonymizeUser(ctx, userID)
		assert.ErrorIs(t, err, dispatcherErr)
	})

	t.Run("should return not-configured error when privacy manager or dispatcher is nil", func(t *testing.T) {
		// Neither the privacy module nor a queue host is mounted in this
		// fx graph yet: AnonymizeUser must fail honestly, not panic.
		ts := setupComplianceService(t)
		ctx, _ := createActorContext()

		noPrivacy := NewComplianceService(ts.legalHoldRepo, ts.retentionRepo, ts.dispatcher, ts.auditRecorder, nil, &log.NoOpLogger{})
		_, err := noPrivacy.AnonymizeUser(ctx, userID)
		assert.ErrorIs(t, err, ErrPrivacyManagerNotConfigured)

		noDispatcher := NewComplianceService(ts.legalHoldRepo, ts.retentionRepo, nil, ts.auditRecorder, ts.privacy, &log.NoOpLogger{})
		ts.privacy.EXPECT().InitiateErasure(ctx, userID, mock.Anything).
			Return(&privacy.Request{ID: uuid.New()}, nil).
			Once()
		_, err = noDispatcher.AnonymizeUser(ctx, userID)
		assert.ErrorIs(t, err, ErrDispatcherNotConfigured)
	})
}
