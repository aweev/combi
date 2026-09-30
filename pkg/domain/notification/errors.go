package notification

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidInput            = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Invalid input provided for notification.", nil)
	ErrInvalidRecipient        = apperror.New(http.StatusBadRequest, "INVALID_RECIPIENT", "The recipient address is invalid.", nil)
	ErrEmptyRecipientList      = apperror.New(http.StatusBadRequest, "EMPTY_RECIPIENT_LIST", "A recipient is required.", nil)
	ErrMissingTemplateVariable = apperror.New(http.StatusBadRequest, "MISSING_TEMPLATE_VARIABLE", "A required variable for the template was not provided.", nil)

	// 403 Forbidden
	ErrUnauthorized    = apperror.New(http.StatusForbidden, "UNAUTHORIZED", "You do not have permission to perform this action.", nil)
	ErrUserHasOptedOut = apperror.New(http.StatusForbidden, "USER_OPTED_OUT", "The user has opted out of receiving this type of notification.", nil)

	// 404 Not Found
	ErrNotificationNotFound = apperror.New(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "The specified notification could not be found.", nil)
	ErrTemplateNotFound     = apperror.New(http.StatusNotFound, "TEMPLATE_NOT_FOUND", "The specified template could not be found.", nil)
	ErrTranslationNotFound  = apperror.New(http.StatusNotFound, "TRANSLATION_NOT_FOUND", "A translation for the requested locale does not exist for this template.", nil)
	ErrProviderNotFound     = apperror.New(http.StatusNotFound, "PROVIDER_NOT_FOUND", "The specified notification provider could not be found.", nil)

	// 409 Conflict
	ErrTemplateAlreadyExists   = apperror.New(http.StatusConflict, "TEMPLATE_ALREADY_EXISTS", "A template with this slug already exists.", nil)
	ErrNotificationAlreadySent = apperror.New(http.StatusConflict, "NOTIFICATION_ALREADY_SENT", "This notification has already been sent.", nil)
	ErrNotificationCancelled   = apperror.New(http.StatusConflict, "NOTIFICATION_CANCELLED", "This notification has been cancelled and cannot be sent.", nil)
	ErrTemplateInUse           = apperror.New(http.StatusConflict, "TEMPLATE_IN_USE", "This template is in use and cannot be deleted.", nil)
	ErrProviderInUse           = apperror.New(http.StatusConflict, "PROVIDER_IN_USE", "This provider is in use and cannot be deleted.", nil)
	ErrTemplateInactive        = apperror.New(http.StatusConflict, "TEMPLATE_INACTIVE", "The selected template is not active.", nil)

	// 500 Internal Server Error
	ErrTemplateRenderFailed = apperror.New(http.StatusInternalServerError, "TEMPLATE_RENDER_FAILED", "Failed to render the notification template.", nil)

	// 503 Service Unavailable
	ErrDeliveryFailed            = apperror.New(http.StatusServiceUnavailable, "DELIVERY_FAILED", "The notification provider failed to deliver the message.", nil)
	ErrProviderRateLimitExceeded = apperror.New(http.StatusServiceUnavailable, "PROVIDER_RATE_LIMITED", "The notification provider has rate limited the request.", nil)
	ErrNoProviderConfigured      = apperror.New(http.StatusServiceUnavailable, "NO_PROVIDER_CONFIGURED", "No active provider is configured for this notification channel.", nil)
)

// --- Worker-Specific Error Wrappers ---

// NonRetryableError wraps a permanent failure from a provider (e.g., hard bounce).
type NonRetryableError struct{ Err error }

func (e *NonRetryableError) Error() string { return fmt.Sprintf("non-retryable error: %v", e.Err) }
func (e *NonRetryableError) Unwrap() error { return e.Err }
func NewNonRetryableError(err error) error { return &NonRetryableError{Err: err} }
func IsNonRetryable(err error) bool {
	var target *NonRetryableError
	return errors.As(err, &target)
}
