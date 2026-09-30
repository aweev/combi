package user

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 404
	ErrProfileNotFound = apperror.New(http.StatusNotFound, "USER_PROFILE_NOT_FOUND", "User profile does not exist.", nil)

	ErrInvalidAvatarType   = apperror.New(http.StatusUnprocessableEntity, "INVALID_AVATAR_TYPE", "Invalid avatar type.", nil)
	ErrForbidden           = apperror.New(http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource.", nil)
	ErrUserNotFound        = apperror.New(http.StatusNotFound, "USER_NOT_FOUND", "User does not exist.", nil)
	ErrCannotConnectToSelf = apperror.New(http.StatusUnprocessableEntity, "CANNOT_CONNECT_TO_SELF", "You cannot connect to yourself.", nil)
	ErrConnectionNotFound  = apperror.New(http.StatusNotFound, "CONNECTION_NOT_FOUND", "Connection does not exist.", nil)

	// 409
	ErrHandleTaken = apperror.New(http.StatusConflict, "USER_HANDLE_TAKEN", "This username/handle is already claimed.", nil)
)
