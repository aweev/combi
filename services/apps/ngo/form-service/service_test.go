package form_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/form"
	form_mocks "github.com/combi/pkg/domain/form/mocks"
	rbac_mocks "github.com/combi/pkg/domain/rbac/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	form_service "github.com/combi/form"
)

type serviceTestSuite struct {
	repo    *form_mocks.MockRepository
	audit   *audit_mocks.MockEventRecorder
	bus     *events_mocks.MockBus
	authz   *rbac_mocks.MockAuthorizationService
	service *form_service.Service
}

func setupService(t *testing.T) serviceTestSuite {
	ts := serviceTestSuite{
		repo:  form_mocks.NewMockRepository(t),
		audit: audit_mocks.NewMockEventRecorder(t),
		bus:   events_mocks.NewMockBus(t),
		authz: rbac_mocks.NewMockAuthorizationService(t),
	}
	ts.service = form_service.NewService(
		ts.repo, ts.audit, ts.bus, ts.authz, &log.NoOpLogger{},
	)
	return ts
}

func contextWithUser() (context.Context, uuid.UUID) {
	id := uuid.New()
	actor := &auth.Actor{UserID: id, AccountID: id}
	return auth.NewContextWithActor(context.Background(), actor), id
}

func TestService_Submit(t *testing.T) {
	ctx, userID := contextWithUser()
	formID := uuid.New()

	// Schema: Required "name" field
	schema := `[{"id":"name","type":"TEXT","required":true}]`
	def := &form.FormDefinition{
		ID: formID, IsActive: true, Schema: json.RawMessage(schema),
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)
		data := map[string]any{"name": "Alice"}

		ts.repo.EXPECT().GetForm(ctx, formID).Return(def, nil).Once()
		ts.repo.EXPECT().CreateSubmission(ctx, mock.MatchedBy(func(s *form.Submission) bool {
			return s.UserID != nil && *s.UserID == userID && s.FormID == formID
		})).Return(nil).Once()

		ts.bus.EXPECT().Publish(ctx, mock.Anything).Return(nil).Once()

		sub, err := ts.service.Submit(ctx, formID, &userID, data)
		require.NoError(t, err)
		assert.NotNil(t, sub)
	})

	t.Run("Fail_Validation", func(t *testing.T) {
		ts := setupService(t)
		data := map[string]any{} // Missing "name"

		ts.repo.EXPECT().GetForm(ctx, formID).Return(def, nil).Once()

		_, err := ts.service.Submit(ctx, formID, &userID, data)
		assert.ErrorIs(t, err, form.ErrInvalidSubmission)
	})
}

func TestService_CastVote(t *testing.T) {
	ctx, userID := contextWithUser()
	formID := uuid.New()
	schema := `[{"id":"vote","type":"RADIO","options":["Yes","No"]}]`
	def := &form.FormDefinition{
		ID: formID, Type: form.TypeVote, IsActive: true, Schema: json.RawMessage(schema), IsAnonymous: true,
	}

	t.Run("Success", func(t *testing.T) {
		ts := setupService(t)
		data := map[string]any{"vote": "Yes"}

		ts.repo.EXPECT().GetForm(ctx, formID).Return(def, nil).Once()

		// Transaction logic
		ts.repo.EXPECT().WithTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(form.Repository) error) error {
			ts.repo.EXPECT().CheckVoteParticipation(ctx, formID, userID).Return(false, nil).Once()
			ts.repo.EXPECT().RecordVoteParticipation(ctx, formID, userID).Return(nil).Once()
			// Expect anonymous submission (UserID is nil)
			ts.repo.EXPECT().CreateSubmission(ctx, mock.MatchedBy(func(s *form.Submission) bool {
				return s.UserID == nil
			})).Return(nil).Once()
			return fn(ts.repo)
		}).Once()

		ts.audit.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		err := ts.service.CastVote(ctx, formID, userID, data)
		require.NoError(t, err)
	})
}
