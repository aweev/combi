package audit

import (
	"fmt"
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrUnsupportedExportFormat = apperror.New(http.StatusBadRequest, "UNSUPPORTED_EXPORT_FORMAT", "The requested export format is not supported.", nil)

	// 404 Not Found
	ErrEventNotFound           = apperror.New(http.StatusNotFound, "AUDIT_EVENT_NOT_FOUND", "The requested audit event could not be found.", nil)
	ErrLegalHoldNotFound       = apperror.New(http.StatusNotFound, "LEGAL_HOLD_NOT_FOUND", "The specified legal hold could not be found.", nil)
	ErrRetentionPolicyNotFound = apperror.New(http.StatusNotFound, "RETENTION_POLICY_NOT_FOUND", "The specified retention policy could not be found.", nil)

	// 500 Internal Server Error
	ErrDecryptionFailed = apperror.New(http.StatusInternalServerError, "DECRYPTION_FAILED", "Failed to decrypt sensitive audit data.", nil)
)

// RepositoryError is kept as a custom type for wrapping database-specific errors with more context.
// In the service layer, you would return one of the AppErrors above and wrap this one.
// e.g., return audit.ErrEventNotFound.WithUnderlying(&RepositoryError{...})
type RepositoryError struct {
	Operation string
	Err       error
}

func (e *RepositoryError) Error() string {
	return fmt.Sprintf("audit repository error during %s: %v", e.Operation, e.Err)
}

func (e *RepositoryError) Unwrap() error {
	return e.Err
}
