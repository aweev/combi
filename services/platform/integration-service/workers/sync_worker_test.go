package workers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/integration"
	int_mocks "github.com/combi/pkg/domain/integration/mocks"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/combi/integration/workers"
)

func TestSyncWorker_ProcessJob(t *testing.T) {
	ctx := context.Background()
	jobID := uuid.New()

	t.Run("should sync all active integrations successfully", func(t *testing.T) {
		repo := int_mocks.NewMockRepository(t)
		adminSvc := int_mocks.NewMockAdminService(t)
		auditRecorder := mocks.NewMockEventRecorder(t)
		worker := workers.NewSyncWorker(repo, adminSvc, auditRecorder, &log.NoOpLogger{})

		integ1 := &integration.Integration{ID: uuid.New(), Name: "Stripe", Status: integration.StatusActive}
		integ2 := &integration.Integration{ID: uuid.New(), Name: "Twilio", Status: integration.StatusError}

		// 1. List active
		repo.EXPECT().ListIntegrations(ctx, mock.MatchedBy(func(f integration.IntegrationFilter) bool {
			return *f.Status == integration.StatusActive
		})).Return([]*integration.Integration{integ1, integ2}, 2, nil).Once()

		// 2. Test connections
		// Success for Stripe
		adminSvc.EXPECT().TestConnection(ctx, integ1.ID).Return(true, "OK", nil).Once()
		// Success for Twilio (Recovery)
		adminSvc.EXPECT().TestConnection(ctx, integ2.ID).Return(true, "Recovered", nil).Once()

		// 3. Update recovered integration
		repo.EXPECT().UpdateIntegration(ctx, mock.MatchedBy(func(i *integration.Integration) bool {
			return i.ID == integ2.ID && i.Status == integration.StatusActive && i.LastError == nil
		})).Return(nil).Once()

		// 4. Audit
		auditRecorder.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		job := &queue.Job{ID: jobID, Payload: []byte(`{}`)}
		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)

		var results map[string]string
		json.Unmarshal(res, &results)
		assert.Equal(t, "healthy", results["Stripe"])
		assert.Equal(t, "healthy", results["Twilio"])
	})

	t.Run("should mark integration as error on failure", func(t *testing.T) {
		repo := int_mocks.NewMockRepository(t)
		adminSvc := int_mocks.NewMockAdminService(t)
		auditRecorder := mocks.NewMockEventRecorder(t)
		worker := workers.NewSyncWorker(repo, adminSvc, auditRecorder, &log.NoOpLogger{})

		integ := &integration.Integration{ID: uuid.New(), Name: "Brevo", Status: integration.StatusActive}

		repo.EXPECT().ListIntegrations(ctx, mock.Anything).Return([]*integration.Integration{integ}, 1, nil).Once()
		adminSvc.EXPECT().TestConnection(ctx, integ.ID).Return(false, "Auth failed", errors.New("401")).Once()

		repo.EXPECT().UpdateIntegration(ctx, mock.MatchedBy(func(i *integration.Integration) bool {
			return i.Status == integration.StatusError && *i.LastError == "Auth failed"
		})).Return(nil).Once()

		auditRecorder.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

		job := &queue.Job{ID: jobID, Payload: []byte(`{}`)}
		res, err := worker.ProcessJob(ctx, job)
		require.NoError(t, err)

		var results map[string]string
		json.Unmarshal(res, &results)
		assert.Contains(t, results["Brevo"], "failed: Auth failed")
	})
}
