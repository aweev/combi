package storage

import (
	"context"
	"io"
	"time"

	"github.com/combi/pkg/domain/privacy"
	"github.com/google/uuid"
)

// ============================================================================
// PRIMARY SERVICE CONTRACTS (Application Layer Ports)
// ============================================================================

// Service defines the primary, high-level business logic for the storage system.
// It orchestrates calls to repositories and providers to fulfill use cases.
type Service interface {
	privacy.Anonymizer
	StoreFile(ctx context.Context, req UploadRequest) (*File, error)
	// InitiateUpload prepares for a client-side upload, creating a file record and returning a pre-signed URL.
	// This is the main entry point for user uploads.
	InitiateUpload(ctx context.Context, req InitiateUploadRequest) (*InitiateUploadResponse, error)

	// CompleteUpload finalizes an upload after the client has sent the file to the pre-signed URL.
	// This transitions the file's status to 'PROCESSING' or 'AVAILABLE'.
	CompleteUpload(ctx context.Context, uploadID uuid.UUID) (*File, error)

	// AbortUpload cancels an in-progress upload session.
	AbortUpload(ctx context.Context, uploadID uuid.UUID) error

	// GetFile retrieves a file's metadata by its ID. It must perform access control checks.
	GetFile(ctx context.Context, fileID uuid.UUID) (*File, error)
	GetFileMetadata(ctx context.Context, fileID uuid.UUID) (*File, error)

	// GetDownloadURL generates a time-limited, pre-signed URL for a client to download a file directly.
	GetDownloadURL(ctx context.Context, fileID uuid.UUID, expiresIn time.Duration) (*SignedURL, error)

	// ListFiles retrieves a paginated and filtered list of files, enforcing access control.
	ListFiles(ctx context.Context, filter FileFilter) ([]*File, int, error)

	// UpdateFileMetadata modifies the metadata of an existing file (e.g., name, tags, description).
	UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, req UpdateFileRequest) (*File, error)

	// DeleteFile moves a file to a 'DELETED' state (soft delete) or removes it permanently.
	DeleteFile(ctx context.Context, fileID uuid.UUID, permanent bool) error

	UploadFile(ctx context.Context, req UploadFileRequest, reader io.Reader) (*File, error)
	// RestoreFile recovers a soft-deleted file.
	RestoreFile(ctx context.Context, fileID uuid.UUID) (*File, error)
}

// AdminService defines administrative and maintenance operations on the storage system.
// Separating this from the core `Service` provides a clearer security boundary.
type AdminService interface {
	// Bucket Management
	CreateBucket(ctx context.Context, req CreateBucketRequest) (*StorageBucket, error)
	GetBucket(ctx context.Context, id uuid.UUID) (*StorageBucket, error)
	ListBuckets(ctx context.Context) ([]*StorageBucket, error)
	UpdateBucket(ctx context.Context, id uuid.UUID, req UpdateBucketRequest) (*StorageBucket, error)
	DeleteBucket(ctx context.Context, id uuid.UUID) error

	// Quota Management
	GetStorageQuota(ctx context.Context, ownerID uuid.UUID, ownerType string) (*StorageQuota, error)
	SetStorageQuota(ctx context.Context, ownerID uuid.UUID, ownerType string, limit int64) (*StorageQuota, error)

	// Maintenance & Cleanup
	CleanupExpiredUploads(ctx context.Context) (int64, error)
	CleanupDeletedFiles(ctx context.Context, olderThan time.Duration) (int64, error)
	ArchiveOldFiles(ctx context.Context, olderThan time.Duration) (int64, error)
}

// WorkerService defines the contract for background processing jobs.
// This interface is implemented by the application layer and called by queue workers.
type WorkerService interface {
	ProcessImage(ctx context.Context, fileID uuid.UUID, variants map[string]string) error
	ProcessVideo(ctx context.Context, fileID uuid.UUID, formats []string, qualities []string) error
	ProcessDocument(ctx context.Context, fileID uuid.UUID, operations []string) error
}

// ProcessorFactory is a contract for retrieving a file processor based on the task type.
// This allows the worker service to be agnostic of the specific processing libraries.
type ProcessorFactory interface {
	GetProcessor(taskType string) (FileProcessor, error)
}

// ============================================================================
// DATA TRANSFER OBJECTS (Service Method Parameters)
// ============================================================================

type InitiateUploadRequest struct {
	OwnerID      uuid.UUID      `json:"owner_id" validate:"required"`
	OwnerType    string         `json:"owner_type" validate:"required,min=3"`
	OriginalName string         `json:"original_name" validate:"required,min=1,max=255"`
	MimeType     string         `json:"mime_type" validate:"required"`
	Size         int64          `json:"size" validate:"required,gt=0"`
	Visibility   FileVisibility `json:"visibility" validate:"required,oneof=PUBLIC PRIVATE"`
	IsMultipart  bool           `json:"is_multipart"`
}

type InitiateUploadResponse struct {
	UploadID  uuid.UUID `json:"upload_id"`
	FileID    uuid.UUID `json:"file_id"`
	SignedURL SignedURL `json:"signed_url"`
}

type UploadFileRequest struct {
	OwnerID      uuid.UUID      `json:"owner_id" validate:"required"`
	OwnerType    string         `json:"owner_type" validate:"required"`
	OriginalName string         `json:"original_name" validate:"required"`
	MimeType     string         `json:"mime_type" validate:"required"`
	Size         int64          `json:"size" validate:"required,gt=0"`
	Visibility   FileVisibility `json:"visibility" validate:"required"`
	Description  *string        `json:"description,omitempty"`
}
type UpdateFileRequest struct {
	OriginalName *string                `json:"original_name,omitempty" validate:"omitempty,min=1,max=255"`
	Description  *string                `json:"description,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Visibility   *FileVisibility        `json:"visibility,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type FileFilter struct {
	OwnerID   *uuid.UUID    `query:"owner_id"`
	OwnerType *string       `query:"owner_type"`
	BucketID  *uuid.UUID    `query:"bucket_id"`
	Category  *FileCategory `query:"category" validate:"omitempty,oneof=DOCUMENT IMAGE VIDEO AUDIO ARCHIVE SPREADSHEET OTHER"`
	Status    *string       `query:"status" validate:"omitempty,oneof=PENDING PROCESSING AVAILABLE ARCHIVED DELETED ERROR"`
	Tags      []string      `query:"tags"`
	Search    string        `query:"search"`
	Page      int           `query:"page" validate:"gte=1"`
	PageSize  int           `query:"pageSize" validate:"gte=1,lte=100"`
	SortBy    string        `query:"sortBy" validate:"omitempty,oneof=created_at original_name size"`
	SortOrder string        `query:"sortOrder" validate:"omitempty,oneof=asc desc"`
}

type CreateBucketRequest struct {
	Name      string          `json:"name" validate:"required,min=3,max=63"`
	Provider  StorageProvider `json:"provider" validate:"required,oneof=S3 GCS AZURE_BLOB LOCAL"`
	Region    string          `json:"region" validate:"required"`
	Endpoint  *string         `json:"endpoint,omitempty" validate:"omitempty,url"`
	IsDefault bool            `json:"is_default" validate:"required"`
}

type UpdateBucketRequest struct {
	IsDefault *bool `json:"is_default,omitempty"`
	IsActive  *bool `json:"is_active,omitempty"`
}

// // --- DTOs for Service Methods ---

// // File represents the metadata of a single stored object.
// type File struct {
// 	ID          uuid.UUID
// 	OwnerID     uuid.UUID
// 	Provider    string
// 	Bucket      string
// 	Path        string
// 	FileName    string
// 	FileSize    int64
// 	ContentType string
// 	CreatedAt   time.Time
// }

// type GenerateUploadURLParams struct {
// 	UploaderID  uuid.UUID
// 	FileName    string
// 	FileSize    int64
// 	ContentType string
// }

// type UploadURLResult struct {
// 	FileID    uuid.UUID
// 	UploadURL string
// 	Method    string // e.g., "PUT", "POST"
// 	Headers   map[string]string
// }
