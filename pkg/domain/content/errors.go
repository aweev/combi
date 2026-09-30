package content

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrValidationFailed       = apperror.New(http.StatusBadRequest, "VALIDATION_FAILED", "Entry content failed validation against the model schema.", nil)
	ErrMissingRequiredField   = apperror.New(http.StatusBadRequest, "MISSING_REQUIRED_FIELD", "A required field is missing from the entry.", nil)
	ErrInvalidStateTransition = apperror.New(http.StatusBadRequest, "INVALID_STATE_TRANSITION", "The requested status change is not allowed.", nil)
	ErrInvalidModelSchema     = apperror.New(http.StatusBadRequest, "INVALID_MODEL_SCHEMA", "The model schema definition is invalid.", nil)
	ErrFieldNotFoundInModel   = apperror.New(http.StatusBadRequest, "FIELD_NOT_FOUND_IN_MODEL", "Field not found in the content model.", nil)
	ErrLocaleNotConfigured    = apperror.New(http.StatusBadRequest, "LOCALE_NOT_CONFIGURED", "The requested locale is not configured for this space.", nil)
	ErrDefaultLocaleMissing   = apperror.New(http.StatusBadRequest, "DEFAULT_LOCALE_MISSING", "Content for the default locale must be provided.", nil)
	ErrNothingToPublish       = apperror.New(http.StatusBadRequest, "NOTHING_TO_PUBLISH", "The draft is identical to the published version.", nil)
	ErrNotPublished           = apperror.New(http.StatusBadRequest, "NOT_PUBLISHED", "The entry has no published version.", nil)

	// 403 Forbidden
	ErrSystemModelImmutable        = apperror.New(http.StatusForbidden, "SYSTEM_MODEL_IMMUTABLE", "System-defined content models cannot be modified or deleted.", nil)
	ErrDefaultSpaceCannotBeDeleted = apperror.New(http.StatusForbidden, "DEFAULT_SPACE_IMMUTABLE", "The default content space cannot be deleted.", nil)

	ErrUnauthorized = apperror.New(http.StatusForbidden, "UNAUTHORIZED", "You are not authorized to perform this action.", nil)

	// 404 Not Found
	ErrSpaceNotFound   = apperror.New(http.StatusNotFound, "SPACE_NOT_FOUND", "The specified content space could not be found.", nil)
	ErrModelNotFound   = apperror.New(http.StatusNotFound, "MODEL_NOT_FOUND", "The specified content model could not be found.", nil)
	ErrEntryNotFound   = apperror.New(http.StatusNotFound, "ENTRY_NOT_FOUND", "The specified content entry could not be found.", nil)
	ErrVersionNotFound = apperror.New(http.StatusNotFound, "VERSION_NOT_FOUND", "The specified version could not be found.", nil)

	// 409 Conflict
	ErrSpaceAlreadyExists    = apperror.New(http.StatusConflict, "SPACE_ALREADY_EXISTS", "A space with this key already exists.", nil)
	ErrModelAlreadyExists    = apperror.New(http.StatusConflict, "MODEL_ALREADY_EXISTS", "A model with this API key already exists.", nil)
	ErrModelInUse            = apperror.New(http.StatusConflict, "MODEL_IN_USE", "Model is in use by entries and cannot be modified in a breaking way.", nil)
	ErrConflict              = apperror.New(http.StatusConflict, "EDIT_CONFLICT", "The entry has been updated since you last read it.", nil)
	ErrHasUnpublishedChanges = apperror.New(http.StatusConflict, "UNPUBLISHED_CHANGES", "Cannot perform action because the entry has unpublished changes.", nil)
)
