package errors

import (
	stderrors "errors"
)

// FromError extracts an *AfrosError from an error chain. It returns nil when
// the chain contains no AfrosError, letting callers decide on fallback
// behaviour (e.g. the middleware error-normalization interceptor).
//
// Use this instead of a direct type assertion — service code wraps errors
// freely (fmt.Errorf("%w", ...), errors.Internal(...).Wrap(...)) and the
// AfrosError may sit anywhere in the chain.
func FromError(err error) *AfrosError {
	if err == nil {
		return nil
	}
	var ae *AfrosError
	if stderrors.As(err, &ae) {
		return ae
	}
	return nil
}
