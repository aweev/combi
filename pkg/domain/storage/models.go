package storage

import (
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

type FileStatus string

const (
	StatusPending    FileStatus = "PENDING"    // File record created, upload not yet complete.
	StatusProcessing FileStatus = "PROCESSING" // Post-upload processing (virus scan, transcoding) is running.
	StatusAvailable  FileStatus = "AVAILABLE"  // File is ready for access.
	StatusArchived   FileStatus = "ARCHIVED"   // File moved to cold storage.
	StatusDeleted    FileStatus = "DELETED"    // Soft-deleted, can be restored.
	StatusError      FileStatus = "ERROR"      // An unrecoverable error occurred (e.g., processing failed).
)

type FileVisibility string

const (
	VisibilityPublic  FileVisibility = "PUBLIC"  // Accessible by anyone with the link, without auth.
	VisibilityPrivate FileVisibility = "PRIVATE" // Accessible only by the owner and explicitly granted users.
)

type StorageProvider string

const (
	ProviderS3         StorageProvider = "S3"
	ProviderGCS        StorageProvider = "GCS"
	ProviderAzure      StorageProvider = "AZURE_BLOB"
	ProviderLocal      StorageProvider = "LOCAL" // For development and testing.
	ProviderCloudinary StorageProvider = "CLOUDINARY"
)

type FileCategory string

const (
	CategoryDocument    FileCategory = "DOCUMENT"
	CategoryImage       FileCategory = "IMAGE"
	CategoryVideo       FileCategory = "VIDEO"
	CategoryAudio       FileCategory = "AUDIO"
	CategoryArchive     FileCategory = "ARCHIVE"
	CategorySpreadsheet FileCategory = "SPREADSHEET"
	CategoryOther       FileCategory = "OTHER"
)

type ProcessingStatus string

const (
	ProcessingPending   ProcessingStatus = "PENDING"
	ProcessingRunning   ProcessingStatus = "RUNNING"
	ProcessingCompleted ProcessingStatus = "COMPLETED"
	ProcessingFailed    ProcessingStatus = "FAILED"
)

// --- Core Models ---

// File is the central entity representing a single stored object and its metadata.
type File struct {
	ID           uuid.UUID      `json:"id"`
	OwnerID      uuid.UUID      `json:"owner_id"`      // The user or system entity that owns this file.
	OwnerType    string         `json:"owner_type"`    // e.g., "user", "organization", "system".
	OriginalName string         `json:"original_name"` // The original filename provided by the user.
	Path         string         `json:"path"`          // The full, unique path within the storage provider (e.g., "user/uuid/file.jpg").
	MimeType     string         `json:"mime_type"`
	Size         int64          `json:"size"` // Size in bytes.
	Category     FileCategory   `json:"category"`
	Status       FileStatus     `json:"status"`
	Visibility   FileVisibility `json:"visibility"`
	BucketID     uuid.UUID      `json:"bucket_id"`      // The bucket this file belongs to.
	Hash         *string        `json:"hash,omitempty"` // e.g., SHA-256 hash for deduplication and integrity.

	// Media-specific metadata
	Width    *int     `json:"width,omitempty"`
	Height   *int     `json:"height,omitempty"`
	Duration *float64 `json:"duration,omitempty"` // In seconds for audio/video.

	// Custom application metadata
	Tags           []string               `json:"tags,omitempty"`
	Description    *string                `json:"description,omitempty"`
	CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`

	// Lifecycle timestamps
	UploadCompletedAt *time.Time `json:"upload_completed_at,omitempty"`
	LastAccessedAt    *time.Time `json:"last_accessed_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"` // For soft deletes.

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileVersion represents an immutable historical version of a File.
type FileVersion struct {
	ID          uuid.UUID `json:"id"`
	FileID      uuid.UUID `json:"file_id"`
	Version     int       `json:"version"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Hash        string    `json:"hash"`
	CreatedBy   uuid.UUID `json:"created_by"` // The actor who created this version.
	CreatedAt   time.Time `json:"created_at"`
	Description *string   `json:"description,omitempty"`
}

// Upload represents an active upload session, crucial for resumable or multipart uploads.
type Upload struct {
	ID            uuid.UUID       `json:"id"`
	FileID        uuid.UUID       `json:"file_id"`
	Provider      StorageProvider `json:"provider"`
	ProviderRef   string          `json:"-"` // Provider-specific upload ID (e.g., S3 UploadId).
	TotalSize     int64           `json:"total_size"`
	BytesUploaded int64           `json:"bytes_uploaded"`
	IsMultipart   bool            `json:"is_multipart"`
	ExpiresAt     time.Time       `json:"expires_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

// FileProcessing represents a background task being performed on a File.
type FileProcessing struct {
	ID        uuid.UUID              `json:"id"`
	FileID    uuid.UUID              `json:"file_id"`
	TaskType  string                 `json:"task_type"` // e.g., "TRANSCODE_MP4", "THUMBNAIL_JPEG_200".
	Status    ProcessingStatus       `json:"status"`
	Progress  int                    `json:"progress"`         // 0-100.
	Result    map[string]interface{} `json:"result,omitempty"` // e.g., {"thumbnail_path": "..."}.
	Error     *string                `json:"error,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// StorageQuota defines storage limits for an owner (e.g., a user or an organization).
type StorageQuota struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   uuid.UUID `json:"owner_id"`
	OwnerType string    `json:"owner_type"`
	Limit     int64     `json:"limit"` // Max storage in bytes. 0 means unlimited.
	Used      int64     `json:"used"`  // Current storage usage in bytes.
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageBucket represents a configured storage container (e.g., an S3 bucket or a local directory).
type StorageBucket struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"` // Must be unique across providers.
	Provider  StorageProvider `json:"provider"`
	Region    string          `json:"region"`
	Endpoint  *string         `json:"endpoint,omitempty"` // For S3-compatible services like MinIO.
	IsDefault bool            `json:"is_default"`
	IsActive  bool            `json:"is_active"`
	// Credentials are NOT part of the domain model; they are an infrastructure concern.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SignedURL represents a time-limited, pre-signed URL for direct access to a file.
type SignedURL struct {
	URL       string    `json:"url"`
	Method    string    `json:"method"` // "GET" for downloads, "PUT" for uploads.
	ExpiresAt time.Time `json:"expires_at"`
}

// ProcessingResult is a standardized DTO returned by a FileProcessor.
type ProcessingResult struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"` // e.g., {"variants": {"thumb": "path"}, "width": 800}
	Error   *string                `json:"error,omitempty"`
}

// --- Helper Methods (Behavior) ---

// IsAvailable checks if the file is in a ready state for access.
func (f *File) IsAvailable() bool {
	return f.Status == StatusAvailable && !f.IsExpired() && !f.IsDeleted()
}

// IsExpired checks if the file's expiration date has passed.
func (f *File) IsExpired() bool {
	return f.ExpiresAt != nil && time.Now().UTC().After(*f.ExpiresAt)
}

// IsDeleted checks if the file has been soft-deleted.
func (f *File) IsDeleted() bool {
	return f.Status == StatusDeleted || f.DeletedAt != nil
}

// HasSpace checks if a new file of a given size can be added without exceeding the quota.
func (q *StorageQuota) HasSpace(fileSize int64) bool {
	if q.Limit == 0 {
		return true // Unlimited quota.
	}
	return (q.Used + fileSize) <= q.Limit
}

func (f *File) DetermineCategory() {
	lowerMime := strings.ToLower(f.MimeType)
	switch {
	case strings.HasPrefix(lowerMime, "image/"):
		f.Category = CategoryImage
	case strings.HasPrefix(lowerMime, "video/"):
		f.Category = CategoryVideo
	case strings.HasPrefix(lowerMime, "audio/"):
		f.Category = CategoryAudio
	case strings.HasPrefix(lowerMime, "application/pdf"):
		f.Category = CategoryDocument
	default:
		f.Category = CategoryOther
	}
}

// ServiceConfig holds configurable parameters for the storage service.
type ServiceConfig struct {
	DefaultQuota        int64         // Default quota in bytes for new users (0 for unlimited).
	UploadURLExpiry     time.Duration // How long pre-signed upload URLs are valid.
	DownloadURLExpiry   time.Duration // Default expiry for download URLs.
	MaxUploadSize       int64         // Maximum allowed file size in bytes.
	AllowedMimeTypePats []string      // List of allowed MIME type patterns (e.g., "image/*", "application/pdf").
	ProcessingQueue     string        // The name of the queue to send processing jobs to.
	SoftDeleteRetention time.Duration // How long to keep soft-deleted files before permanent deletion.
}

func DetermineCategory(mimeType string) FileCategory {
	lowerMime := strings.ToLower(mimeType)
	switch {
	case strings.HasPrefix(lowerMime, "image/"):
		return CategoryImage
	case strings.HasPrefix(lowerMime, "video/"):
		return CategoryVideo
	case strings.HasPrefix(lowerMime, "audio/"):
		return CategoryAudio
	case strings.Contains(lowerMime, "pdf"):
		return CategoryDocument
	case strings.Contains(lowerMime, "spreadsheet") || strings.Contains(lowerMime, "excel"):
		return CategorySpreadsheet
	case strings.Contains(lowerMime, "zip") || strings.Contains(lowerMime, "rar"):
		return CategoryArchive
	default:
		return CategoryOther
	}
}

// UploadRequest is used for internal/backend file uploads (streaming io.Reader).
// This differs from InitiateUploadRequest which is for client-side presigned URLs.
type UploadRequest struct {
	Filename    string
	ContentType string
	Size        int64
	Content     io.ReadCloser // The data stream
	Path        string        // The target path (e.g. "reports/123/file.pdf")
	IsPrivate   bool          // Access control
}
