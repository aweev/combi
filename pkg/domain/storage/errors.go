package storage

import (
	"errors"
	"fmt"
)

// --- Generic Domain Errors ---
var (
	ErrInvalidInput = errors.New("invalid input provided")
	ErrHashMismatch = errors.New("file hash does not match expected value")
	ErrInvalidPath  = errors.New("invalid storage path format")
)

// --- File & Version Errors ---
var (
	ErrFileNotFound         = errors.New("file not found")
	ErrFileAlreadyExists    = errors.New("file already exists at this path")
	ErrFileDeleted          = errors.New("file has been deleted")
	ErrFileExpired          = errors.New("file has expired and is no longer accessible")
	ErrFileNotAvailable     = errors.New("file is not yet available for access (e.g., still processing)")
	ErrFileTooLarge         = errors.New("file size exceeds the maximum allowed by policy or quota")
	ErrInvalidMimeType      = errors.New("the provided MIME type is invalid or not permitted")
	ErrInvalidFileName      = errors.New("the provided file name is invalid")
	ErrVersionNotFound      = errors.New("the requested file version does not exist")
	ErrVersioningNotEnabled = errors.New("versioning is not enabled for this file or bucket")
)

// --- Upload & Download Errors ---
var (
	ErrUploadNotFound         = errors.New("upload session not found")
	ErrUploadExpired          = errors.New("upload session has expired")
	ErrUploadAlreadyCompleted = errors.New("upload has already been completed")
	ErrUploadFailed           = errors.New("upload failed to complete")
	ErrStreamingNotSupported  = errors.New("streaming download is not supported for this file or provider")
)

// --- Access & Permission Errors ---
var (
	ErrAccessDenied      = errors.New("access denied to this resource")
	ErrFileNotPublic     = errors.New("file is not publicly accessible")
	ErrInvalidShareToken = errors.New("invalid or expired share token")
)

// --- Quota & Bucket Errors ---
var (
	ErrQuotaExceeded       = errors.New("storage quota for the owner has been exceeded")
	ErrQuotaNotFound       = errors.New("storage quota configuration not found for the owner")
	ErrBucketNotFound      = errors.New("the specified storage bucket does not exist")
	ErrBucketAlreadyExists = errors.New("a bucket with this name already exists")
	ErrBucketNotEmpty      = errors.New("bucket is not empty and cannot be deleted")
	ErrNoDefaultBucket     = errors.New("no default storage bucket is configured")
)

// --- Provider & External System Errors ---
var (
	ErrProviderNotSupported = errors.New("the configured storage provider is not supported")
	ErrProviderAuthFailed   = errors.New("authentication with the storage provider failed")
)

// ProviderError is a structured error for wrapping issues from an external storage provider (e.g., S3, GCS).
type ProviderError struct {
	Operation string
	Reason    string
	Err       error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("storage provider error during '%s': %s", e.Operation, e.Reason)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError.
func NewProviderError(operation, reason string, err error) error {
	return &ProviderError{Operation: operation, Reason: reason, Err: err}
}

// --- Processing Errors ---
var (
	ErrProcessingFailed          = errors.New("file processing failed")
	ErrProcessingInProgress      = errors.New("file is currently being processed")
	ErrUnsupportedProcessingTask = errors.New("the requested processing task is not supported")
)
