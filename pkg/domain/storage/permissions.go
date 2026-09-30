package storage

import "github.com/combi/pkg/domain/core"

const (
	// PermStorageFileUpload allows a user to upload files.
	// This can be scoped to a specific owner (e.g., themselves, their organization).
	PermStorageFileUpload core.PermissionName = "storage:file:upload"

	// PermStorageFileRead allows a user to read file metadata and get download links.
	PermStorageFileRead core.PermissionName = "storage:file:read"

	// PermStorageFileDelete allows a user to delete files.
	PermStorageFileDelete core.PermissionName = "storage:file:delete"

	// PermStorageFileSelf is a general permission allowing a user to manage their own files.
	// Specific checks (e.g., "is this user the owner?") are done in the handler/service.
	PermStorageFileSelf core.PermissionName = "storage:file:self"

	// PermStorageAdmin grants full administrative access to the storage system (buckets, quotas).
	PermStorageAdmin core.PermissionName = "storage:admin:manage"
)
