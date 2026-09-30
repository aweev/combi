package reporting

import (
	"context"

	"github.com/google/uuid"
)

// Service is the primary interface used by other domains.
type Service interface {
	// Registry Pattern
	RegisterFetcher(reportType ReportType, fetcher DataFetcher)

	// Execution
	RequestReport(ctx context.Context, req ReportRequest) (*GeneratedReport, error)
	GetReport(ctx context.Context, id uuid.UUID) (*GeneratedReport, error)
	ListReports(ctx context.Context, requesterID uuid.UUID, limit, offset int) ([]GeneratedReport, int, error)

	// Template Management (Admin)
	CreateTemplate(ctx context.Context, t ReportTemplate) (*ReportTemplate, error)
	GetTemplate(ctx context.Context, slug string) (*ReportTemplate, error)
	ListTemplates(ctx context.Context) ([]ReportTemplate, error)
}

// Repository defines the persistence contract.
type Repository interface {
	// Reports
	CreateReport(ctx context.Context, r *GeneratedReport) error
	GetReport(ctx context.Context, id uuid.UUID) (*GeneratedReport, error)
	UpdateReportStatus(ctx context.Context, id uuid.UUID, status ReportStatus, fileURL *string, errReason *string) error
	ListReports(ctx context.Context, requesterID uuid.UUID, limit, offset int) ([]GeneratedReport, int, error)

	// Templates
	CreateTemplate(ctx context.Context, t *ReportTemplate) error
	GetTemplateBySlug(ctx context.Context, slug string) (*ReportTemplate, error)
	ListTemplates(ctx context.Context) ([]ReportTemplate, error)
}

// DataProvider is an interface that other services must implement if they want to feed data into reports.
// This decouples ReportingService from knowing the internals of FinancialService.
type DataFetcher interface {
	FetchData(ctx context.Context, params map[string]any) (any, error)
}

// DTOs
type ReportRequest struct {
	TemplateSlug string
	RequesterID  uuid.UUID
	Parameters   map[string]any
	Format       OutputFormat
}
