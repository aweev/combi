package storage

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	FileRepository
	UploadRepository
	BucketRepository
	QuotaRepository
}

type FileRepository interface {
	CreateFile(ctx context.Context, file *File) error
	GetFile(ctx context.Context, id uuid.UUID) (*File, error)
	ListFiles(ctx context.Context, filter FileFilter) ([]*File, int, error)
	UpdateFile(ctx context.Context, file *File) error
	DeleteFile(ctx context.Context, id uuid.UUID) error // Permanent delete from DB
	FindSoftDeletedBefore(ctx context.Context, cutoff time.Time) ([]*File, error)
}

type UploadRepository interface {
	CreateUpload(ctx context.Context, upload *Upload) error
	GetUpload(ctx context.Context, id uuid.UUID) (*Upload, error)
	DeleteUpload(ctx context.Context, id uuid.UUID) error
	FindExpiredUploads(ctx context.Context) ([]*Upload, error)
}

type BucketRepository interface {
	CreateBucket(ctx context.Context, bucket *StorageBucket) error
	GetBucket(ctx context.Context, id uuid.UUID) (*StorageBucket, error)
	GetBucketByName(ctx context.Context, name string) (*StorageBucket, error)
	GetDefaultBucket(ctx context.Context) (*StorageBucket, error)
	ListBuckets(ctx context.Context) ([]*StorageBucket, error)
	UpdateBucket(ctx context.Context, bucket *StorageBucket) error
	DeleteBucket(ctx context.Context, id uuid.UUID) error
}

type QuotaRepository interface {
	GetOrCreateQuota(ctx context.Context, ownerID uuid.UUID, ownerType string, defaultLimit int64) (*StorageQuota, error)
	UpdateQuota(ctx context.Context, quota *StorageQuota) error
	IncrementUsage(ctx context.Context, ownerID uuid.UUID, ownerType string, size int64) error
	DecrementUsage(ctx context.Context, ownerID uuid.UUID, ownerType string, size int64) error
}

type Provider interface {
	// GeneratePresignedUploadURL creates a URL that a client can use to PUT an object directly.
	GeneratePresignedUploadURL(ctx context.Context, bucket, path, mimeType string, size int64, expiresIn time.Duration) (*SignedURL, error)

	// GeneratePresignedDownloadURL creates a URL that a client can use to GET an object directly.
	GeneratePresignedDownloadURL(ctx context.Context, bucket, path string, expiresIn time.Duration) (*SignedURL, error)

	// Upload directly streams data from the server to the provider.
	Upload(ctx context.Context, bucket, path string, reader io.Reader, size int64, mimeType string) error

	// Download streams an object from the provider to the server.
	Download(ctx context.Context, bucket, path string) (io.ReadCloser, error)

	// Delete removes an object from the provider.
	Delete(ctx context.Context, bucket, path string) error

	// Exists checks if an object exists at the given path.
	Exists(ctx context.Context, bucket, path string) (bool, error)
}

// ProviderFactory is a contract for a component that can retrieve a configured and ready-to-use
// storage provider implementation based on its type or a specific bucket configuration.
// This abstraction is key to supporting multiple cloud providers (S3, GCS, etc.).
type ProviderFactory interface {
	// GetProvider returns the provider implementation for a given bucket.
	GetProvider(ctx context.Context, bucket *StorageBucket) (Provider, error)

	// GetDefaultProvider is a convenience method to get the provider for the default bucket.
	GetDefaultProvider(ctx context.Context) (Provider, *StorageBucket, error)
}

// FileProcessor defines the contract for a component that can perform a specific
// transformation or analysis on a file (e.g., resizing an image, transcoding a video).
type FileProcessor interface {
	// Process takes an input file path, performs its task, and is responsible for
	// uploading any generated artifacts using the provided provider.
	// It returns a result containing metadata about the processed files.
	Process(ctx context.Context, inputPath string, options map[string]interface{}) (*ProcessingResult, error)

	// GetSupportedTasks returns the list of task types this processor can handle.
	GetSupportedTasks() []string
}
