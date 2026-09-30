package consent_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	dconsent "github.com/combi/pkg/domain/consent"
	consent_mocks "github.com/combi/pkg/domain/consent/mocks"
	event_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/consent/consent"
)

type serviceSuite struct {
	repo    *consent_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	bus     *event_mocks.MockBus
	service *consent.Service
}

func setupService(t *testing.T) serviceSuite {
	s := serviceSuite{
		repo:  consent_mocks.NewMockRepository(t),
		audit: audit_mocks.NewMockEventRecorder(t),
		bus:   event_mocks.NewMockBus(t),
	}
	s.service = consent.NewService(s.repo, s.audit, s.bus, &log.NoOpLogger{})
	return s
}

func TestService_GrantConsent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)
		ctx := context.Background()
		uid := uuid.New()
		revID := uuid.New()

		rev := &dconsent.Revision{ID: revID, AgreementID: uuid.New(), IsActive: true}

		ts.repo.EXPECT().GetRevision(ctx, revID).Return(rev, nil)
		ts.repo.EXPECT().CreateUserConsent(ctx, mock.Anything).Return(nil)

		// Async calls (Audit/Bus) - use Maybe or WaitGroup in real impl if needed
		ts.audit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()
		ts.bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()

		err := ts.service.GrantConsent(ctx, uid, revID, "1.2.3.4", "Chrome")
		require.NoError(t, err)
	})

	t.Run("Fail_Inactive", func(t *testing.T) {
		ts := setupService(t)
		ctx := context.Background()
		revID := uuid.New()

		ts.repo.EXPECT().GetRevision(ctx, revID).Return(&dconsent.Revision{IsActive: false}, nil)

		err := ts.service.GrantConsent(ctx, uuid.New(), revID, "", "")
		assert.ErrorIs(t, err, dconsent.ErrRevisionInactive)
	})
}

func TestService_CheckCompliance(t *testing.T) {
	ts := setupService(t)
	ctx := context.Background()
	uid := uuid.New()

	missing := []dconsent.Agreement{{Slug: "tos"}}
	ts.repo.EXPECT().GetMissingMandatoryConsents(ctx, uid).Return(missing, nil)

	status, err := ts.service.CheckCompliance(ctx, uid)
	require.NoError(t, err)
	assert.False(t, status.IsCompliant)
	assert.Equal(t, "tos", status.MissingConsents[0].Slug)
}
