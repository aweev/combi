package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

var _ reporting.Service = (*service)(nil)

type service struct {
	repo    reporting.Repository
	queue   queue.Dispatcher
	storage storage.Service // Required for Presigned URLs
	audit   audit.EventRecorder
	logger  log.Logger

	// Registry State
	fetchers   map[reporting.ReportType]reporting.DataFetcher
	fetchersMu sync.RWMutex
}

func NewService(
	repo reporting.Repository,
	queue queue.Dispatcher,
	storageSvc storage.Service,
	audit audit.EventRecorder,
	logger log.Logger,
) reporting.Service {
	return &service{
		repo:     repo,
		queue:    queue,
		storage:  storageSvc,
		audit:    audit,
		logger:   logger.With(log.String("component", "reporting_service")),
		fetchers: make(map[reporting.ReportType]reporting.DataFetcher),
	}
}

// --- Registry Implementation ---

func (s *service) RegisterFetcher(rt reporting.ReportType, fetcher reporting.DataFetcher) {
	s.fetchersMu.Lock()
	defer s.fetchersMu.Unlock()
	s.fetchers[rt] = fetcher
	s.logger.Info("Registered data fetcher for report type", log.String("type", string(rt)))
}

// Internal helper for the Worker to retrieve fetchers
func (s *service) getFetcher(rt reporting.ReportType) (reporting.DataFetcher, error) {
	s.fetchersMu.RLock()
	defer s.fetchersMu.RUnlock()
	f, ok := s.fetchers[rt]
	if !ok {
		return nil, fmt.Errorf("no data fetcher registered for report type: %s", rt)
	}
	return f, nil
}

// --- Report Request Logic ---

func (s *service) RequestReport(ctx context.Context, req reporting.ReportRequest) (*reporting.GeneratedReport, error) {
	// 1. Get Template
	tmpl, err := s.repo.GetTemplateBySlug(ctx, req.TemplateSlug)
	if err != nil {
		return nil, err
	}

	// 2. Validate Parameters against JSON Schema
	if err := s.validateParams(tmpl.ParamSchema, req.Parameters); err != nil {
		return nil, fmt.Errorf("%w: %v", reporting.ErrInvalidParameters, err)
	}

	// 3. Create "Pending" Record
	paramBytes, _ := json.Marshal(req.Parameters)
	reportID := uuid.New()

	report := &reporting.GeneratedReport{
		ID:          reportID,
		TemplateID:  &tmpl.ID,
		RequesterID: req.RequesterID,
		Type:        tmpl.Type,
		Status:      reporting.StatusPending,
		Format:      req.Format,
		Parameters:  paramBytes,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   Ptr(time.Now().UTC().Add(7 * 24 * time.Hour)), // 7 Day Retention
	}

	if err := s.repo.CreateReport(ctx, report); err != nil {
		return nil, err
	}

	// 4. Dispatch Job to Queue
	// The worker will pick this up to perform the heavy lifting
	jobPayload := map[string]string{"report_id": reportID.String()}
	jobBytes, _ := json.Marshal(jobPayload)

	_, err = s.queue.Dispatch(ctx, reporting.JobGenerateReport, jobBytes, queue.WithPriority(5))
	if err != nil {
		// Fallback: If queue fails, fail the report immediately
		msg := "Failed to queue generation job"
		_ = s.repo.UpdateReportStatus(ctx, reportID, reporting.StatusFailed, nil, &msg)
		return nil, err
	}

	// 5. Audit Log
	if err := s.audit.Record(ctx, audit.NewEvent(req.RequesterID, "user", core.EventReportRequested).
		WithTarget(reportID, "report", tmpl.Name).
		WithSuccess().
		Build()); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return report, nil
}

// --- Retrieval (Presigned URLs) ---

func (s *service) GetReport(ctx context.Context, id uuid.UUID) (*reporting.GeneratedReport, error) {
	report, err := s.repo.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	// SECURITY: If completed, generate a short-lived (15m) Presigned URL.
	// We DO NOT store public URLs in the database.
	if report.Status == reporting.StatusCompleted && report.FileURL != nil {
		// report.FileURL here acts as the "Storage Key" (e.g., "reports/123/file.pdf")
		if fileID, err := uuid.Parse(*report.FileURL); err == nil {
			signed, err := s.storage.GetDownloadURL(ctx, fileID, 15*time.Minute)
			if err != nil {
				s.logger.Error("Failed to sign URL", err)
				return nil, fmt.Errorf("storage access error")
			}
			report.FileURL = &signed.URL
		} else {
			s.logger.Warn("Report file reference is not a UUID, returning raw path", log.String("ref", *report.FileURL))
		}
	}

	return report, nil
}

// --- Helpers ---

func (s *service) validateParams(schemaJSON json.RawMessage, params map[string]any) error {
	// Empty schema means no validation required
	if len(schemaJSON) <= 2 {
		return nil
	}

	loader := gojsonschema.NewBytesLoader(schemaJSON)
	dataLoader := gojsonschema.NewGoLoader(params)

	result, err := gojsonschema.Validate(loader, dataLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		// Aggregate errors into readable string
		msg := ""
		for _, desc := range result.Errors() {
			msg += fmt.Sprintf("- %s (Field: %s)\n", desc.Description(), desc.Field())
		}
		return fmt.Errorf("parameter validation failed:\n%s", msg)
	}

	return nil
}

func Ptr[T any](v T) *T { return &v }

// Passthrough methods
func (s *service) ListReports(ctx context.Context, requesterID uuid.UUID, limit, offset int) ([]reporting.GeneratedReport, int, error) {
	return s.repo.ListReports(ctx, requesterID, limit, offset)
}
func (s *service) CreateTemplate(ctx context.Context, t reporting.ReportTemplate) (*reporting.ReportTemplate, error) {
	if err := s.repo.CreateTemplate(ctx, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
func (s *service) GetTemplate(ctx context.Context, slug string) (*reporting.ReportTemplate, error) {
	return s.repo.GetTemplateBySlug(ctx, slug)
}
func (s *service) ListTemplates(ctx context.Context) ([]reporting.ReportTemplate, error) {
	return s.repo.ListTemplates(ctx)
}
