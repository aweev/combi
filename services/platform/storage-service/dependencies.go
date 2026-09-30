// internal/services/storage/dependencies.go
package storage

import (
	"context"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// ProviderFactory is a contract for a component that can retrieve a configured and ready-to-use
// storage provider implementation based on its type or a specific bucket configuration.
// This abstraction is key to supporting multiple cloud providers (S3, GCS, etc.).
type ProviderFactory interface {
	// GetProvider returns the provider implementation for a given bucket.
	GetProvider(ctx context.Context, bucket *storage.StorageBucket) (storage.Provider, error)

	// GetDefaultProvider is a convenience method to get the provider for the default bucket.
	GetDefaultProvider(ctx context.Context) (storage.Provider, *storage.StorageBucket, error)
}

// ProcessorFactory is a contract for retrieving a file processor based on the task type.
// This allows the worker service to be agnostic of the specific processing libraries.
type ProcessorFactory interface {
	GetProcessor(taskType string) (storage.FileProcessor, error)
}

// Dependencies holds all the external services and repositories that the storage
// application services need to function. This struct is used for dependency injection.
type Dependencies struct {
	// Repo provides access to the storage persistence layer (database).
	Repo storage.Repository

	// ProviderFct is a factory for creating clients for underlying storage backends (S3, GCS, etc.).
	ProviderFct ProviderFactory

	// ProcessorFct is a factory for creating clients for background file processing tasks (image resizing, etc.).
	ProcessorFct storage.ProcessorFactory

	// Dispatcher is used to enqueue background jobs (e.g., for file processing).
	Dispatcher queue.Dispatcher

	// AuditRecorder is used to log all significant actions for security and compliance.
	AuditRecorder audit.EventRecorder

	// RBACService is the central authority for all authorization checks ("can this user do this?").
	RBACService rbac.AuthorizationService // <-- ADDED: For robust security checks.

	// Registrar allows the domain to register its permissions with the system.
	Registrar rbac.PermissionRegistrar
	Registry  core.Registry

	// Logger provides structured, contextual logging.
	Logger log.Logger
}
