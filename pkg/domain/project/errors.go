package project

import (
	"errors"
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidStatusTransition = apperror.New(http.StatusBadRequest, "INVALID_TRANSITION", "This status transition is not allowed.", nil)
	ErrMilestoneIncomplete     = apperror.New(http.StatusBadRequest, "MILESTONE_INCOMPLETE", "Cannot complete milestone with open tasks.", nil)
	ErrDuplicateProjectName    = apperror.New(http.StatusBadRequest, "DUPLICATE_NAME", "A project with this name already exists in the organization.", nil)
	ErrCannotRemoveLastAdmin   = apperror.New(http.StatusBadRequest, "LAST_ADMIN", "Cannot remove the last admin from the project.", nil)
	ErrFileLimitReached        = apperror.New(http.StatusBadRequest, "FILE_LIMIT_REACHED", "Project file storage limit reached.", nil)

	// 404 Not Found
	ErrProjectNotFound   = apperror.New(http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found.", nil)
	ErrTaskNotFound      = apperror.New(http.StatusNotFound, "TASK_NOT_FOUND", "Task not found.", nil)
	ErrMilestoneNotFound = apperror.New(http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone not found.", nil)
	ErrMemberNotFound    = apperror.New(http.StatusNotFound, "MEMBER_NOT_FOUND", "User is not a member of this project.", nil)
	ErrFileNotFound      = apperror.New(http.StatusNotFound, "FILE_NOT_FOUND", "File not found.", nil)

	// 403 Forbidden
	ErrAccessDenied    = apperror.New(http.StatusForbidden, "ACCESS_DENIED", "You do not have permission to access this resource.", nil)
	ErrProjectArchived = apperror.New(http.StatusForbidden, "PROJECT_ARCHIVED", "Project is archived and read-only.", nil)

	// 409 Conflict
	ErrAlreadyAMember = apperror.New(http.StatusConflict, "ALREADY_MEMBER", "User is already a member of this project.", nil)

	// Internal Errors (Unwrapped)
	ErrInvalidInput = errors.New("invalid input provided")
)
