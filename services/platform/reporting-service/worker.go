package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

type GenerationWorker struct {
	service *service // Concretely typed to access repo & fetchers
	logger  log.Logger
}

func NewGenerationWorker(s *service, logger log.Logger) *GenerationWorker {
	return &GenerationWorker{
		service: s,
		logger:  logger.With(log.String("worker", string(reporting.JobGenerateReport))),
	}
}

func (w *GenerationWorker) JobType() core.JobType {
	return reporting.JobGenerateReport
}

func (w *GenerationWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// 1. Unmarshal Job Payload
	var payload struct {
		ReportID string `json:"report_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}
	reportID := uuid.MustParse(payload.ReportID)

	w.logger.Info("Starting report generation", log.String("report_id", payload.ReportID))

	// 2. Fetch Report Metadata
	report, err := w.service.repo.GetReport(ctx, reportID)
	if err != nil {
		return nil, queue.NewNonRetryableError(err) // If report deleted, stop
	}

	// Update Status -> Processing
	_ = w.service.repo.UpdateReportStatus(ctx, reportID, reporting.StatusProcessing, nil, nil)

	// 3. Resolve Data Fetcher
	fetcher, err := w.service.getFetcher(report.Type)
	if err != nil {
		return nil, w.fail(ctx, reportID, fmt.Sprintf("System Error: %v", err))
	}

	// 4. Fetch Data
	var params map[string]any
	_ = json.Unmarshal(report.Parameters, &params)

	data, err := fetcher.FetchData(ctx, params)
	if err != nil {
		// Business logic error in the fetcher (e.g., query failed)
		return nil, w.fail(ctx, reportID, fmt.Sprintf("Data Retrieval Failed: %v", err))
	}

	// 5. Render (Strategy Pattern)
	var content []byte
	var mimeType, extension string

	switch report.Format {
	case reporting.FormatJSON:
		content, _ = json.MarshalIndent(data, "", "  ")
		mimeType = "application/json"
		extension = "json"
	case reporting.FormatCSV:
		// Real implementation: Use reflection or a library like `gocarina/gocsv`
		content = []byte("header,value\nmock,data") // Simplified for brevity
		mimeType = "text/csv"
		extension = "csv"
	case reporting.FormatPDF:
		// Real implementation: wkhtmltopdf / gofpdf
		content = []byte("%PDF-1.4 ... (mock pdf content)")
		mimeType = "application/pdf"
		extension = "pdf"
	default:
		return nil, w.fail(ctx, reportID, "Unsupported output format")
	}

	// 6. Upload to Storage (Private Bucket)
	// We create a structured path: private/reports/{UserID}/{ReportID}.{ext}
	storagePath := fmt.Sprintf("private/reports/%s/%s.%s", report.RequesterID, report.ID, extension)

	uploadReq := storage.UploadRequest{
		Filename:    fmt.Sprintf("%s.%s", report.ID, extension), // Simple name for metadata
		ContentType: mimeType,
		Size:        int64(len(content)),
		Content:     io.NopCloser(bytes.NewReader(content)),
		Path:        storagePath,
		IsPrivate:   true, // CRITICAL
	}

	// Direct call to Storage Service (bypassing HTTP)
	storedFile, err := w.service.storage.StoreFile(ctx, uploadReq)
	if err != nil {
		return nil, w.fail(ctx, reportID, fmt.Sprintf("Storage Error: %v", err))
	}

	// 7. Complete
	err = w.service.repo.UpdateReportStatus(ctx, reportID, reporting.StatusCompleted, &storedFile.Path, nil)
	if err != nil {
		return nil, err // Retryable DB error
	}

	// (Optional) Dispatch "Report Ready" Notification Event here

	return nil, nil
}

func (w *GenerationWorker) fail(ctx context.Context, id uuid.UUID, reason string) error {
	_ = w.service.repo.UpdateReportStatus(ctx, id, reporting.StatusFailed, nil, &reason)
	return queue.NewNonRetryableError(fmt.Errorf("%s", reason))
}
