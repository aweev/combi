// internal/domain/financial/errors.go
package financial

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	ErrForbidden = apperror.New(http.StatusForbidden, "FORBIDDEN", "Access denied.", nil)
	// 400 Bad Request
	ErrInsufficientFunds = apperror.New(http.StatusBadRequest, "INSUFFICIENT_FUNDS", "The payment source has insufficient funds.", nil)
	ErrCardExpired       = apperror.New(http.StatusBadRequest, "CARD_EXPIRED", "The payment card has expired.", nil)
	ErrInvalidInput      = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Invalid input provided.", nil)

	ErrCustomerNotFound         = apperror.New(http.StatusBadRequest, "CUSTOMER_NOT_FOUND", "The requested customer could not be found.", nil)
	ErrIdempotencyConflict      = apperror.New(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "An identical request has already been processed.", nil)
	ErrDuplicateTransaction     = apperror.New(http.StatusBadRequest, "DUPLICATE_TRANSACTION", "This transaction has already been processed.", nil)
	ErrTransactionNotRefundable = apperror.New(http.StatusBadRequest, "TRANSACTION_NOT_REFUNDABLE", "This transaction cannot be refunded.", nil)

	// 404 Not Found
	ErrTransactionNotFound  = apperror.New(http.StatusNotFound, "TRANSACTION_NOT_FOUND", "The requested transaction could not be found.", nil)
	ErrSubscriptionNotFound = apperror.New(http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "The requested subscription could not be found.", nil)

	// 402 Payment Required (Special case for declined payments)
	ErrPaymentDeclined = apperror.New(http.StatusPaymentRequired, "PAYMENT_DECLINED", "The payment was declined by the provider.", nil)

	// 503 Service Unavailable
	ErrPaymentProviderError = apperror.New(http.StatusServiceUnavailable, "PAYMENT_PROVIDER_ERROR", "The payment provider encountered an error.", nil)
)
