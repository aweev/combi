package matchmaking

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrAlreadyMatched    = apperror.New(http.StatusBadRequest, "ALREADY_MATCHED", "This request has already been matched.", nil)
	ErrInvalidTransition = apperror.New(http.StatusBadRequest, "INVALID_TRANSITION", "Cannot change match status in this way.", nil)

	ErrForbidden = apperror.New(http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource.", nil)
	// 404 Not Found
	ErrRequestNotFound = apperror.New(http.StatusNotFound, "MATCH_REQUEST_NOT_FOUND", "The specified match request could not be found.", nil)
	ErrMatchNotFound   = apperror.New(http.StatusNotFound, "MATCH_NOT_FOUND", "The specified match relationship could not be found.", nil)
)
