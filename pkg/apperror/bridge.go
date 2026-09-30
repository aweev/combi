// bridge.go normalises the legacy apperror.AppError onto the canonical
// errors.AfrosError wire format.
//
// pkg/apperror predates pkg/errors and large parts of the ported service code
// still construct AppError values. Rather than rewriting every call site at
// once, this bridge lets the shared middleware convert both types to the same
// canonical gRPC/HTTP representation. New code must use pkg/errors directly;
// apperror is expected to shrink as services are ported and eventually
// disappear.

package apperror

import (
	stderrors "errors"
	"net/http"

	pkgerrors "github.com/combi/pkg/errors"
)

// afrosCode maps an AppError Type onto the canonical AfrosError code.
// Unknown types map to INTERNAL so nothing escapes normalization.
func afrosCode(t Type) pkgerrors.Code {
	switch t {
	case NotFound:
		return pkgerrors.CodeNotFound
	case InvalidInput:
		return pkgerrors.CodeInvalidInput
	case Unauthorized:
		return pkgerrors.CodeUnauthorized
	case Forbidden:
		return pkgerrors.CodeForbidden
	case Conflict:
		return pkgerrors.CodeConflict
	case Internal:
		return pkgerrors.CodeInternal
	default:
		return pkgerrors.CodeInternal
	}
}

// ToAfros converts an AppError into the canonical *errors.AfrosError carrying
// the same type, message, and (for validation failures) field details.
//
// The original error is preserved as the cause so server-side logs keep the
// full chain; AfrosError's own serialization already strips causes before
// anything is sent to callers.
func ToAfros(e *AppError) *pkgerrors.AfrosError {
	afros := &pkgerrors.AfrosError{
		Code:    afrosCode(e.Type),
		Message: e.Message,
	}
	if e.Err != nil {
		afros = afros.Wrap(e.Err)
	}
	return afros
}

// AsAfros returns the canonical AfrosError for any error whose chain contains
// an AppError, and nil otherwise. This is what middleware normalization uses:
//
//	if afros := apperror.AsAfros(err); afros != nil { ... }
func AsAfros(err error) *pkgerrors.AfrosError {
	var appErr *AppError
	if !stderrors.As(err, &appErr) {
		return nil
	}
	return ToAfros(appErr)
}

// httpStatusFor mirrors apperror's HTTP codes for compatibility checks.
func httpStatusFor(t Type) int {
	switch t {
	case NotFound:
		return http.StatusNotFound
	case InvalidInput:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case Conflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
