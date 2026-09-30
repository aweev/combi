package integration

import (
	"fmt"
	"net/http"

	"github.com/combi/pkg/apperror"
)

// --- AppError Definitions ---
// These errors can be returned directly to the HTTP layer for a consistent response.
var (
	// 400 Bad Request
	ErrInvalidInput         = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Invalid input provided for the integration operation.", nil)
	ErrInvalidConfiguration = apperror.New(http.StatusBadRequest, "INVALID_CONFIGURATION", "The provided configuration for the integration is invalid.", nil)
	ErrUnsupportedEventType = apperror.New(http.StatusBadRequest, "UNSUPPORTED_EVENT_TYPE", "The received webhook event type is not supported.", nil)
	ErrInvalidSignature     = apperror.New(http.StatusBadRequest, "INVALID_SIGNATURE", "The webhook signature is invalid.", nil)

	// 403 Forbidden
	ErrPermissionDenied = apperror.New(http.StatusForbidden, "PERMISSION_DENIED", "The configured credentials do not have permission to perform this action.", nil)

	// 404 Not Found
	ErrIntegrationNotFound = apperror.New(http.StatusNotFound, "INTEGRATION_NOT_FOUND", "The requested integration configuration could not be found.", nil)
	ErrWebhookNotFound     = apperror.New(http.StatusNotFound, "WEBHOOK_NOT_FOUND", "The requested webhook event could not be found.", nil)

	// 409 Conflict
	ErrDuplicateWebhook    = apperror.New(http.StatusConflict, "DUPLICATE_WEBHOOK", "This webhook event has already been received and is being processed.", nil)
	ErrIntegrationInactive = apperror.New(http.StatusConflict, "INTEGRATION_INACTIVE", "The integration is not active and cannot be used.", nil)

	// 429 Too Many Requests
	ErrRateLimitExceeded = apperror.New(http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "The rate limit for the external service has been exceeded.", nil)

	// 500 Internal Server Error (for issues within our control)
	ErrMissingCredentials    = apperror.New(http.StatusInternalServerError, "MISSING_CREDENTIALS", "Required credentials for the integration are not configured.", nil)
	ErrCryptoOperationFailed = apperror.New(http.StatusInternalServerError, "CRYPTO_ERROR", "Failed to encrypt or decrypt integration credentials.", nil)
	ErrOperationNotSupported = apperror.New(http.StatusInternalServerError, "OPERATION_NOT_SUPPORTED", "This operation is not supported by the integration provider.", nil)

	// 503 Service Unavailable (for issues with the external provider)
	ErrConnectionFailed     = apperror.New(http.StatusServiceUnavailable, "PROVIDER_CONNECTION_FAILED", "Failed to connect to the external service.", nil)
	ErrAuthenticationFailed = apperror.New(http.StatusServiceUnavailable, "PROVIDER_AUTH_FAILED", "Authentication with the external service failed (e.g., invalid API key).", nil)
	ErrProviderError        = apperror.New(http.StatusServiceUnavailable, "PROVIDER_SERVER_ERROR", "The external service returned a server-side error (5xx).", nil)
	ErrRequestTimeout       = apperror.New(http.StatusServiceUnavailable, "PROVIDER_TIMEOUT", "The request to the external service timed out.", nil)

	// 502 Bad Gateway (for malformed responses from the provider)
	ErrUnexpectedResponse = apperror.New(http.StatusBadGateway, "PROVIDER_UNEXPECTED_RESPONSE", "The external service returned an unexpected response format.", nil)
	ErrBadRequest         = apperror.New(http.StatusBadGateway, "PROVIDER_BAD_REQUEST", "The external service rejected our request as invalid (4xx). This may indicate a bug in our adapter.", nil)
)

// --- Non-AppError Wrappers ---
// These are for internal signaling, primarily to the queue system.

// NonRetryableError wraps a permanent failure from a provider.
type NonRetryableError struct{ Err error }

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable integration error: %v", e.Err)
}
func (e *NonRetryableError) Unwrap() error { return e.Err }
func NewNonRetryableError(err error) error { return &NonRetryableError{Err: err} }

func IsNonRetryable(err error) bool {
	_, ok := err.(*NonRetryableError)
	return ok
}

func NewNonRetryable(err error) error {
	return &NonRetryableError{Err: err}
}

func NewNonRetryablef(format string, args ...interface{}) error {
	return &NonRetryableError{Err: fmt.Errorf(format, args...)}
}
