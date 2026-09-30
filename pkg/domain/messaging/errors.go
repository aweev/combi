package messaging

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrEmptyMessage    = apperror.New(http.StatusBadRequest, "EMPTY_MESSAGE", "Message content cannot be empty.", nil)
	ErrInvalidFileType = apperror.New(http.StatusBadRequest, "INVALID_FILE_TYPE", "This file type is not allowed in chat.", nil)

	// 403 Forbidden
	ErrNotMember      = apperror.New(http.StatusForbidden, "NOT_CHANNEL_MEMBER", "You are not a member of this chat channel.", nil)
	ErrMessageDeleted = apperror.New(http.StatusForbidden, "MESSAGE_DELETED", "This message has been deleted.", nil)

	// 404 Not Found
	ErrChannelNotFound = apperror.New(http.StatusNotFound, "CHANNEL_NOT_FOUND", "The specified channel could not be found.", nil)
	ErrMessageNotFound = apperror.New(http.StatusNotFound, "MESSAGE_NOT_FOUND", "The specified message could not be found.", nil)
)
