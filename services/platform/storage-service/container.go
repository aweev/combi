// internal/services/storage/container.go
package storage

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/storage/workers"
)


// Container holds the public-facing services for the Storage domain.
// It is the single entry point for other parts of the application to interact
// with the storage system.
type Container struct {
	Service       storage.Service
	AdminService  storage.AdminService
	WorkerService storage.WorkerService
	ProviderFct   ProviderFactory
	Workers       []queue.JobProcessor
}

// NewContainer is the factory function that builds and wires the entire Storage service.
// It also registers its required permissions with the central RBAC registrar.
func NewContainer(cfg *config.Config, deps Dependencies) (*Container, error) {
	if deps.Repo == nil || deps.Logger == nil || deps.AuditRecorder == nil || deps.Registrar == nil || deps.Dispatcher == nil || deps.RBACService == nil {
		return nil, fmt.Errorf("cannot build Storage container with nil dependencies")
	}

	// --- 1. Register Storage Domain Permissions ---
	deps.Registrar.Register(rbac.PermissionDefinition{Name: storage.PermStorageFileUpload, Description: "Can upload files on behalf of an owner.", Category: "Storage"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: storage.PermStorageFileRead, Description: "Can read file metadata and get download URLs.", Category: "Storage"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: storage.PermStorageFileDelete, Description: "Can delete files.", Category: "Storage"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: storage.PermStorageFileSelf, Description: "Allows a user to manage their own files.", Category: "Storage"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: storage.PermStorageAdmin, Description: "Full administrative access to storage system.", Category: "Storage"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: "storage:file:list_all", Description: "Allows listing files for any user, bypassing ownership checks.", Category: "Storage"})

	deps.Registry.RegisterService(BuildManifest())

	// --- 2. Build Service Configuration from Global Config ---
	// This isolates the storage service from the global config structure.
	serviceConfig := storage.ServiceConfig{
		DefaultQuota:        cfg.Storage.DefaultQuotaBytes,
		UploadURLExpiry:     cfg.Storage.UploadURLExpiry,
		DownloadURLExpiry:   cfg.Storage.DownloadURLExpiry,
		MaxUploadSize:       cfg.Storage.MaxUploadSizeBytes,
		AllowedMimeTypePats: cfg.Storage.AllowedMimePatterns,
		ProcessingQueue:     cfg.Storage.ProcessingQueue,
		SoftDeleteRetention: 30 * 24 * time.Hour, // Example: 30 days
	}

	// --- 3. Instantiate Services ---
	// The internal components are created and wired together here, hidden from main.go.
	service := NewService(deps, serviceConfig)
	adminService := NewAdminService(deps, serviceConfig)
	workerService := NewWorkerService(deps, serviceConfig)

	workers := []queue.JobProcessor{
		workers.NewImageWorker(workerService, deps.Logger),
		workers.NewVideoWorker(workerService, deps.Logger),
		workers.NewDocumentWorker(workerService, deps.Logger),
		workers.NewDeleteFileWorker(service, deps.Logger),
		workers.NewCleanupUploadsWorker(deps.Repo, deps.Logger),            // Uses repo directly
		workers.NewCleanupWorker(deps.Repo, deps.ProviderFct, deps.Logger), // For soft deletes
	}

	// --- 4. Assemble and return the final container ---
	// This is the public-facing "product" of the storage module.
	container := &Container{
		Service:       service,
		AdminService:  adminService,
		WorkerService: workerService,
		ProviderFct:   deps.ProviderFct,
		Workers:       workers,
	}

	return container, nil
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "storage-service",
		DisplayName: "File Storage",
		Description: "Manages file uploads, buckets, quotas, and media processing.",
		Icon:        "hard-drive", // Lucide icon
		Events: []core.EventDefinition{
			{
				Key:         core.EventFileUploadCompleted,
				Label:       "File Uploaded",
				Description: "A file has been successfully uploaded and is ready for processing.",
				Schema: map[string]string{
					"file_id":   "uuid",
					"file_name": "string",
					"size":      "int",
					"mime_type": "string",
				},
			},
			{
				Key:         core.EventFileProcessingCompleted,
				Label:       "Processing Completed",
				Description: "Background processing (e.g. resizing) for a file has finished.",
				Schema: map[string]string{
					"file_id": "uuid",
					"status":  "string",
				},
			},
			{
				Key:         core.EventFileDeleted,
				Label:       "File Deleted",
				Description: "A file has been soft-deleted.",
				Schema: map[string]string{
					"file_id": "uuid",
				},
			},
		},
		Permissions: []core.PermDefinition{
			{
				Key:         storage.PermStorageFileUpload,
				Label:       "Upload Files",
				Category:    "Storage",
				Description: "Allows uploading files to owned buckets.",
			},
			{
				Key:         storage.PermStorageFileRead,
				Label:       "Read Files",
				Category:    "Storage",
				Description: "Allows viewing file metadata and downloading files.",
			},
			{
				Key:         storage.PermStorageAdmin,
				Label:       "Administer Storage",
				Category:    "Storage",
				Description: "Full access to buckets, quotas, and all files.",
				Dangerous:   true,
			},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         storage.JobStorageProcessImage,
				Label:       "Process Image",
				Description: "Generates thumbnails and webp variants for images.",
			},
			{
				Key:         storage.JobStorageCleanupExpiredUploads,
				Label:       "Cleanup Uploads",
				Description: "Removes stale upload sessions and partial files.",
			},
			{
				Key:         storage.JobStorageCleanupDeleted,
				Label:       "Purge Deleted Files",
				Description: "Permanently removes soft-deleted files after retention period.",
			},
		},
	}
}
