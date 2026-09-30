package storage

import "github.com/google/uuid"

// const (
// 	// --- File Lifecycle Events ---
// EventFileUploadInitiated core.EventAction = "storage.file.upload_initiated"
// EventFileUploadCompleted core.EventAction = "storage.file.upload_completed"
// EventFileDeleted         core.EventAction = "storage.file.deleted"
// EventFileMetadataUpdated core.EventAction = "storage.file.metadata_updated"
// EventFileDownloaded      core.EventAction = "storage.file.downloaded"
// EventFileRestored        core.EventAction = "storage.file.restored"

// // --- File Processing Events ---
// EventFileProcessingStarted   core.EventAction = "storage.file.processing_started"
// EventFileProcessingCompleted core.EventAction = "storage.file.processing_completed"
// EventFileProcessingFailed    core.EventAction = "storage.file.processing_failed"

// // --- Administrative Events ---
// EventBucketCreated core.EventAction = "storage.bucket.created"
// EventBucketDeleted core.EventAction = "storage.bucket.deleted"
// EventBucketUpdated core.EventAction = "storage.file.updated"

// EventQuotaUpdated core.EventAction = "storage.quota.updated"
// )

type FileUploadedPayload struct {
	FileID   uuid.UUID `json:"file_id"`
	Uploader uuid.UUID `json:"uploader_id"`
	Bucket   string    `json:"bucket"`
	Key      string    `json:"key"`
	MimeType string    `json:"mime_type"`
	Size     int64     `json:"size"`
	Public   bool      `json:"public"`
}
