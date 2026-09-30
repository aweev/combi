package automation

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 404 Not Found
	ErrRuleNotFound      = apperror.New(http.StatusNotFound, apperror.NotFound, "automation rule not found", nil)
	ErrExecutionNotFound = apperror.New(http.StatusNotFound, apperror.NotFound, "automation execution not found", nil)

	// 409 Conflict (State issues)
	ErrRuleInactive  = apperror.New(http.StatusConflict, apperror.Conflict, "rule is inactive and cannot be triggered", nil)
	ErrWorkflowPause = apperror.New(http.StatusConflict, apperror.Conflict, "workflow is paused and cannot be resumed", nil)

	// 400 Bad Request (Configuration issues)
	ErrInvalidAction    = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "action configuration is invalid", nil)
	ErrInvalidCondition = apperror.New(http.StatusBadRequest, apperror.InvalidInput, "condition configuration is invalid", nil)

	// 500 Internal / 422 Unprocessable (Runtime issues)
	ErrActionExecution  = apperror.New(http.StatusInternalServerError, apperror.Internal, "action execution failed", nil)
	ErrTriggerThrottled = apperror.New(http.StatusTooManyRequests, apperror.Conflict, "rule trigger was throttled", nil)
)
