package storage

import "github.com/combi/pkg/domain/core"

const (
	// JobStorageProcessImage triggers image resizing and optimization.
	JobStorageProcessImage core.JobType = "storage:process_image"

	// JobStorageProcessVideo triggers video transcoding for HLS.
	JobStorageProcessVideo core.JobType = "storage:process_video"

	// JobStorageProcessAsset triggers processing for other assets like documents.
	JobStorageProcessAsset core.JobType = "storage:process_asset"

	// JobStorageCleanupDeleted permanently removes files that were soft-deleted.
	JobStorageCleanupDeleted core.JobType = "storage:cleanup_deleted"

	// JobStorageCleanupExpiredUploads removes placeholder records for incomplete uploads.
	JobStorageCleanupExpiredUploads core.JobType = "storage:cleanup_expired_uploads"

	// JobStorageArchiveOld moves old, unused files to a cheaper storage tier.
	JobStorageArchiveOld core.JobType = "storage:archive_old"

	JobStorageDeleteFile core.JobType = "storage:delete_file"
)
