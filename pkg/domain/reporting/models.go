package reporting

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type ReportType string

const (
	TypeFinancialSummary ReportType = "FINANCIAL_SUMMARY"
	TypeImpactReport     ReportType = "IMPACT_REPORT"
	TypeBoardPacket      ReportType = "BOARD_PACKET"
	TypeUserActivity     ReportType = "USER_ACTIVITY"
	TypeDonationReceipt  ReportType = "DONATION_RECEIPT"
	TypeProjectStatus    ReportType = "PROJECT_STATUS"
	TypeEventAttendance  ReportType = "EVENT_ATTENDANCE"
	TypeFormResponses    ReportType = "FORM_RESPONSES"
	TypeAuditLogs        ReportType = "AUDIT_LOGS"
)

type OutputFormat string

const (
	FormatPDF  OutputFormat = "PDF"
	FormatCSV  OutputFormat = "CSV"
	FormatXLSX OutputFormat = "XLSX"
	FormatJSON OutputFormat = "JSON"
)

type ReportStatus string

const (
	StatusPending    ReportStatus = "PENDING"
	StatusProcessing ReportStatus = "PROCESSING"
	StatusCompleted  ReportStatus = "COMPLETED"
	StatusFailed     ReportStatus = "FAILED"
	StatusExpired    ReportStatus = "EXPIRED" // File deleted from storage
)

// --- Core Models ---

// ReportTemplate defines the structure and data queries for a specific report type.
type ReportTemplate struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"` // e.g., "Annual Impact Report 2024"
	Slug        string     `json:"slug" db:"slug"`
	Type        ReportType `json:"type" db:"type"`
	Description string     `json:"description" db:"description"`

	// Template definition (e.g., HTML/Handlebars string or path to file)
	Content string `json:"content" db:"content"`

	// Schema for parameters required to run this report (JSON Schema)
	ParamSchema json.RawMessage `json:"param_schema" db:"param_schema"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// GeneratedReport represents a specific execution instance.
type GeneratedReport struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	TemplateID  *uuid.UUID   `json:"template_id" db:"template_id"` // Nullable for ad-hoc reports
	RequesterID uuid.UUID    `json:"requester_id" db:"requester_id"`
	Type        ReportType   `json:"type" db:"type"`
	Status      ReportStatus `json:"status" db:"status"`

	// Inputs provided by the user (e.g., { "start_date": "2023-01-01" })
	Parameters json.RawMessage `json:"parameters" db:"parameters"`

	// Output
	Format      OutputFormat `json:"format" db:"format"`
	FileURL     *string      `json:"file_url" db:"file_url"` // Secure link from StorageService
	FileSize    int64        `json:"file_size" db:"file_size"`
	ErrorReason *string      `json:"error_reason" db:"error_reason"`

	ExpiresAt   *time.Time `json:"expires_at" db:"expires_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}
