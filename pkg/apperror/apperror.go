package apperror

import (
	"errors"
	"fmt"
)

type Type string

const (
	NotFound     Type = "NOT_FOUND"
	InvalidInput Type = "INVALID_INPUT"
	Unauthorized Type = "UNAUTHORIZED"
	Internal     Type = "INTERNAL"
	Conflict     Type = "CONFLICT"
	Forbidden    Type = "FORBIDDEN"
)

// AppError is the standard error type.
type AppError struct {
	Code    int    // HTTP status code
	Type    Type   // String code
	Message string // Human readable
	Err     error  // The internal error (wrapped)
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError.
func New(code int, typ Type, msg string, err error) *AppError {
	return &AppError{
		Code:    code,
		Type:    typ,
		Message: msg,
		Err:     err,
	}
}

// IsType checks if an error is of a specific AppError Type.
func IsType(err error, typ Type) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == typ
	}
	return false
}

func (e *AppError) WithUnderlying(err error) *AppError {
	return &AppError{
		Code:    e.Code,
		Type:    e.Type,
		Message: e.Message,
		Err:     err,
	}
}

func GetType(err error) Type {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}
	return ""
}
