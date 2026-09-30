// audit-log-service — ported from api/internal/services/audit/compliance_worker.go.
package auditlog

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

var _ queue.JobProcessor = (*ComplianceWorker)(nil)

// ComplianceWorker executes GDPR erasure jobs by delegating to the privacy
// manager (the consent/privacy service owns the anonymizers).
type ComplianceWorker struct {
	logger  log.Logger
	manager privacy.Manager // nil until the privacy module is mounted
}

// NewComplianceWorker takes the Manager interface.
func NewComplianceWorker(logger log.Logger, manager privacy.Manager) *ComplianceWorker {
	return &ComplianceWorker{
		logger:  logger.With(log.String("component", "compliance_worker")),
		manager: manager,
	}
}

func (w *ComplianceWorker) JobType() core.JobType {
	return audit.JobComplianceAnonymizeUser
}

func (w *ComplianceWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	if w.manager == nil {
		// The privacy module is not mounted in this host. Non-retryable:
		// re-queueing cannot help until the privacy service is available.
		w.logger.Error("privacy manager not configured; cannot process anonymization job", nil,
			log.String("job_id", job.ID.String()))
		return nil, queue.NewNonRetryableError(ErrPrivacyManagerNotConfigured)
	}

	var p struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	reqID, err := uuid.Parse(p.RequestID)
	if err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	// Delegate to Manager
	if err := w.manager.ProcessErasure(ctx, reqID); err != nil {
		return nil, err
	}

	return json.Marshal(map[string]string{"status": "completed"})
}
