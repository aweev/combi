package reporting_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	svc "github.com/combi/reporting"

	// Mocks
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	queue_mocks "github.com/combi/pkg/domain/queue/mocks"
	reporting_mocks "github.com/combi/pkg/domain/reporting/mocks"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
)

// MockFetcher implements reporting.DataFetcher
type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) FetchData(ctx context.Context, params map[string]any) (any, error) {
	args := m.Called(ctx, params)
	return args.Get(0), args.Error(1)
}

type serviceSuite struct {
	repo    *reporting_mocks.MockRepository
	queue   *queue_mocks.MockDispatcher
	storage *storage_mocks.MockService
	audit   *audit_mocks.MockEventRecorder
	service reporting.Service
}

func setupTest(t *testing.T) *serviceSuite {
	s := &serviceSuite{
		repo:    reporting_mocks.NewMockRepository(t),
		queue:   queue_mocks.NewMockDispatcher(t),
		storage: storage_mocks.NewMockService(t),
		audit:   audit_mocks.NewMockEventRecorder(t),
	}

	s.service = svc.NewService(
		s.repo,
		s.queue,
		s.storage,
		s.audit,
		&log.NoOpLogger{},
	)
	return s
}

func TestRequestReport(t *testing.T) {
	s := setupTest(t)
	ctx := context.Background()
	slug := "financial-2024"
	requesterID := uuid.New()

	// Schema requires "year" (integer)
	schema := `{"type": "object", "properties": {"year": {"type": "integer"}}, "required": ["year"]}`

	tmpl := &reporting.ReportTemplate{
		ID:          uuid.New(),
		Slug:        slug,
		Name:        "Financials",
		Type:        reporting.TypeFinancialSummary,
		ParamSchema: json.RawMessage(schema),
	}

	t.Run("Success", func(t *testing.T) {
		// 1. Mock Template Fetch
		s.repo.On("GetTemplateBySlug", ctx, slug).Return(tmpl, nil).Once()

		// 2. Mock Create Report (Success)
		s.repo.On("CreateReport", ctx, mock.MatchedBy(func(r *reporting.GeneratedReport) bool {
			return r.Status == reporting.StatusPending && r.RequesterID == requesterID
		})).Return(nil).Once()

		// 3. Mock Queue Dispatch
		s.queue.On("Dispatch", ctx, reporting.JobGenerateReport, mock.Anything, mock.Anything).
			Return(&queue.Job{}, nil).Once()

		// 4. Mock Audit
		s.audit.On("Record", ctx, mock.Anything).Return(nil).Once()

		req := reporting.ReportRequest{
			TemplateSlug: slug,
			RequesterID:  requesterID,
			Format:       reporting.FormatPDF,
			Parameters:   map[string]any{"year": 2024},
		}

		resp, err := s.service.RequestReport(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, reporting.StatusPending, resp.Status)
	})

	t.Run("Validation Failure", func(t *testing.T) {
		s.repo.On("GetTemplateBySlug", ctx, slug).Return(tmpl, nil).Once()

		// Missing "year" parameter
		req := reporting.ReportRequest{
			TemplateSlug: slug,
			RequesterID:  requesterID,
			Format:       reporting.FormatPDF,
			Parameters:   map[string]any{},
		}

		_, err := s.service.RequestReport(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestGetReport(t *testing.T) {
	s := setupTest(t)
	ctx := context.Background()
	reportID := uuid.New()
	fileID := uuid.New()
	fileIDStr := fileID.String()

	t.Run("Get Completed Report with Presigned URL", func(t *testing.T) {
		report := &reporting.GeneratedReport{
			ID:      reportID,
			Status:  reporting.StatusCompleted,
			FileURL: &fileIDStr, // Storing UUID string as planned
		}

		s.repo.On("GetReport", ctx, reportID).Return(report, nil).Once()

		signedURL := &storage.SignedURL{
			URL:       "https://signed.url",
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}

		// Expect call to sign the UUID
		s.storage.On("GetDownloadURL", ctx, fileID, 15*time.Minute).
			Return(signedURL, nil).Once()

		got, err := s.service.GetReport(ctx, reportID)
		require.NoError(t, err)
		assert.Equal(t, "https://signed.url", *got.FileURL)
	})

	t.Run("Get Pending Report", func(t *testing.T) {
		report := &reporting.GeneratedReport{
			ID:     reportID,
			Status: reporting.StatusPending,
		}
		s.repo.On("GetReport", ctx, reportID).Return(report, nil).Once()

		got, err := s.service.GetReport(ctx, reportID)
		require.NoError(t, err)
		assert.Equal(t, reporting.StatusPending, got.Status)
		// Storage should NOT be called
	})
}

func TestRegistry(t *testing.T) {
	// Since registry methods are internal helpers or void, we test them via integration flow logic usually.
	// But we can verify no panic on registration.
	s := setupTest(t)
	fetcher := new(MockFetcher)

	assert.NotPanics(t, func() {
		s.service.RegisterFetcher(reporting.TypeFinancialSummary, fetcher)
	})
}
