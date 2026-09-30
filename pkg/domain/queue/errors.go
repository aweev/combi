package queue

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidJobType = apperror.New(http.StatusBadRequest, "INVALID_JOB_TYPE", "Job type cannot be empty.", nil)

	// 404 Not Found
	ErrJobNotFound        = apperror.New(http.StatusNotFound, "JOB_NOT_FOUND", "The requested job could not be found.", nil)
	ErrScheduleNotFound   = apperror.New(http.StatusNotFound, "SCHEDULE_NOT_FOUND", "The requested schedule could not be found.", nil)
	ErrFailedJobNotFound  = apperror.New(http.StatusNotFound, "FAILED_JOB_NOT_FOUND", "The requested failed job could not be found.", nil)
	ErrDLQMessageNotFound = apperror.New(http.StatusNotFound, "DLQ_MESSAGE_NOT_FOUND", "The requested DLQ message could not be found.", nil)
	ErrNoJobAvailable     = apperror.New(http.StatusNotFound, "NO_JOB_AVAILABLE", "No job is currently available in the queue.", nil)

	// 409 Conflict
	ErrWorkerAlreadyRegistered = apperror.New(http.StatusConflict, "WORKER_ALREADY_REGISTERED", "A worker for this job type is already registered.", nil)
	ErrInvalidStateTransition  = apperror.New(http.StatusConflict, "INVALID_STATE_TRANSITION", "The requested operation is not valid for the job's current state.", nil)
	ErrPoolAlreadyRunning      = apperror.New(http.StatusConflict, "POOL_ALREADY_RUNNING", "The worker pool is already running.", nil)
	ErrPoolNotRunning          = apperror.New(http.StatusConflict, "POOL_NOT_RUNNING", "The worker pool is not running.", nil)
	ErrSchedulerAlreadyRunning = apperror.New(http.StatusConflict, "SCHEDULER_ALREADY_RUNNING", "The scheduler is already running.", nil)
	ErrSchedulerNotRunning     = apperror.New(http.StatusConflict, "SCHEDULER_NOT_RUNNING", "The scheduler is not running.", nil)

	// 422 Unprocessable Entity
	ErrWorkerNotRegistered = apperror.New(http.StatusUnprocessableEntity, "WORKER_NOT_REGISTERED", "No worker is registered to handle this job type.", nil)

	// 503 Service Unavailable
	ErrQueuePaused     = apperror.New(http.StatusServiceUnavailable, "QUEUE_PAUSED", "The queue is currently paused and not accepting new jobs.", nil)
	ErrLockNotAcquired = apperror.New(http.StatusServiceUnavailable, "LOCK_NOT_ACQUIRED", "Could not acquire a distributed lock to perform the operation.", nil)
)

// --- Retry Logic Wrappers (These are not AppErrors themselves) ---

type NonRetryableError struct{ Err error }

func (e *NonRetryableError) Error() string { return fmt.Sprintf("non-retryable error: %v", e.Err) }
func (e *NonRetryableError) Unwrap() error { return e.Err }
func NewNonRetryableError(err error) error { return &NonRetryableError{Err: err} }

func IsNonRetryable(err error) bool {
	var target *NonRetryableError
	return errors.As(err, &target)
}

type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return fmt.Sprintf("retryable error: %v", e.Err) }
func (e *RetryableError) Unwrap() error { return e.Err }
func IsRetryable(err error) bool {
	var target *RetryableError
	return errors.As(err, &target)
}
func NewRetryableError(err error) error { return &RetryableError{Err: err} }
