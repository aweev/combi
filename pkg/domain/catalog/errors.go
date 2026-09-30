package catalog

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	ErrForbidden = apperror.New(http.StatusForbidden, apperror.Forbidden, "access denied", nil)
	// 404 Not Found
	ErrItemNotFound        = apperror.New(http.StatusNotFound, apperror.NotFound, "catalog item not found", nil)
	ErrCategoryNotFound    = apperror.New(http.StatusNotFound, apperror.NotFound, "category not found", nil)
	ErrApplicationNotFound = apperror.New(http.StatusNotFound, apperror.NotFound, "application not found", nil)
	ErrVariantNotFound     = apperror.New(http.StatusNotFound, apperror.NotFound, "item variant not found", nil)

	// 409 Conflict
	ErrItemAlreadyExists     = apperror.New(http.StatusConflict, apperror.Conflict, "an item with this slug already exists", nil)
	ErrCategoryAlreadyExists = apperror.New(http.StatusConflict, apperror.Conflict, "a category with this slug already exists", nil)
	ErrCategoryInUse         = apperror.New(http.StatusConflict, apperror.Conflict, "category contains items and cannot be deleted", nil)
	ErrAlreadyApplied        = apperror.New(http.StatusConflict, apperror.Conflict, "user has already applied to this item", nil)
	ErrItemNotAvailable      = apperror.New(http.StatusConflict, apperror.Conflict, "item is not available for application", nil)

	// 400 Bad Request / 422 Unprocessable Entity
	ErrCircularCategory     = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "circular category hierarchy detected", nil)
	ErrInvalidStatus        = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "invalid status transition", nil)
	ErrWithdrawalNotAllowed = apperror.New(http.StatusUnprocessableEntity, apperror.InvalidInput, "cannot withdraw application at this stage", nil)
)
