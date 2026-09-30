package privacy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	ErrRequestNotFound = apperror.New(
		http.StatusNotFound,
		"PRIVACY_REQUEST_NOT_FOUND",
		"The specified privacy request could not be found.",
		nil,
	)

	ErrRequestPending = apperror.New(
		http.StatusConflict,
		"PRIVACY_REQUEST_PENDING",
		"A privacy request is already in progress for this user.",
		nil,
	)

	ErrExportExpired = apperror.New(
		http.StatusGone,
		"EXPORT_EXPIRED",
		"The requested data export has expired and is no longer available.",
		nil,
	)
)

type ErasureError struct {
	Service string
	Err     error
}

func (e *ErasureError) Error() string {
	return fmt.Sprintf("erasure failed in service '%s': %v", e.Service, e.Err)
}

func (e *ErasureError) Unwrap() error { return e.Err }

type RequestType string
type RequestStatus string

const (
	TypeErasure RequestType = "ERASURE"
	TypeExport  RequestType = "EXPORT"

	StatusPending    RequestStatus = "PENDING"
	StatusProcessing RequestStatus = "PROCESSING"
	StatusCompleted  RequestStatus = "COMPLETED"
	StatusFailed     RequestStatus = "FAILED"
)

// Request represents a formal GDPR/CCPA subject request.
type Request struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	Type           RequestType    `json:"type"`
	Status         RequestStatus  `json:"status"`
	RequestedAt    time.Time      `json:"requested_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	ResultLocation *string        `json:"result_location,omitempty"`
	ErasureDetails *ErasureReport `json:"erasure_details,omitempty"`
}

// ErasureReport summarizes the outcome of a "Right to be Forgotten" operation.
type ErasureReport struct {
	Service           string `json:"service"`            // The name of the service (e.g., "Financial")
	RecordsDeleted    int64  `json:"records_deleted"`    // Actual database rows removed
	RecordsAnonymized int64  `json:"records_anonymized"` // Rows kept but scrubbed (e.g., ledger entries)
	Details           string `json:"details,omitempty"`
}

// Repository defines where Privacy Requests are stored.
type Repository interface {
	SaveRequest(ctx context.Context, req *Request) error
	GetRequest(ctx context.Context, id uuid.UUID) (*Request, error)
	GetPendingRequest(ctx context.Context, userID uuid.UUID, typ RequestType) (*Request, error)
}

type Anonymizer interface {
	AnonymizeUser(ctx context.Context, userID uuid.UUID) (*ErasureReport, error)
}

type Exporter interface {
	ExportUserData(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error)
}

type Manager interface {
	RegisterAnonymizer(a Anonymizer) error
	InitiateErasure(ctx context.Context, userID uuid.UUID, reason string) (*Request, error)
	InitiateExport(ctx context.Context, userID uuid.UUID) (*Request, error)
	GetRequestStatus(ctx context.Context, requestID uuid.UUID) (*Request, error)
	ProcessErasure(ctx context.Context, reqID uuid.UUID) error
}
