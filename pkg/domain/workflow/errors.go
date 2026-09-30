package workflow

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 404 Not Found
	ErrDefinitionNotFound = apperror.New(http.StatusNotFound, apperror.NotFound, "workflow definition not found", nil)
	ErrInstanceNotFound   = apperror.New(http.StatusNotFound, apperror.NotFound, "workflow instance not found", nil)
	ErrRequestNotFound    = apperror.New(http.StatusNotFound, apperror.NotFound, "approval request not found", nil)

	// 409 Conflict (State issues)
	ErrWorkflowAlreadyExists = apperror.New(http.StatusConflict, apperror.Conflict, "active workflow already exists for this subject", nil)
	ErrWorkflowCompleted     = apperror.New(http.StatusConflict, apperror.Conflict, "workflow is already completed", nil)
	ErrApprovalProcessed     = apperror.New(http.StatusConflict, apperror.Conflict, "approval request has already been decided", nil)

	// 400 Bad Request
	ErrInvalidSignal     = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "invalid signal data", nil)
	ErrInvalidDefinition = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "workflow definition config is invalid", nil)

	// 500 Internal
	ErrTemporalCommunication = apperror.New(http.StatusInternalServerError, apperror.Internal, "failed to communicate with workflow engine", nil)
)
