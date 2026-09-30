// internal/services/storage/admin_service.go
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"

	"github.com/google/uuid"
)

type AdminService struct {
	repo          storage.Repository
	dispatcher    queue.Dispatcher
	auditRecorder audit.EventRecorder
	rbacService   rbac.AuthorizationService
	config        storage.ServiceConfig
	logger        log.Logger
}

// A constant for the batch size of expired upload cleanup.
const expiredUploadsBatchSize = 50

func NewAdminService(deps Dependencies, cfg storage.ServiceConfig) *AdminService {
	return &AdminService{
		repo:          deps.Repo,
		dispatcher:    deps.Dispatcher,
		auditRecorder: deps.AuditRecorder,
		rbacService:   deps.RBACService,
		config:        cfg,
		logger:        deps.Logger.With(log.String("service", "storage_admin")),
	}
}

var _ storage.AdminService = (*AdminService)(nil)

// Bucket Management
// ... (no changes in this section) ...
func (s *AdminService) CreateBucket(ctx context.Context, req storage.CreateBucketRequest) (*storage.StorageBucket, error) {
	actor, err := s.authorize(ctx, storage.PermStorageAdmin)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Admin creating new bucket", log.String("name", req.Name), log.String("provider", string(req.Provider)))

	if _, err := s.repo.GetBucketByName(ctx, req.Name); !errors.Is(err, storage.ErrBucketNotFound) {
		if err == nil {
			return nil, storage.ErrBucketAlreadyExists
		}
		return nil, err
	}

	bucket := &storage.StorageBucket{
		ID:       uuid.New(),
		Name:     req.Name,
		Provider: req.Provider,
		Region:   req.Region,
		Endpoint: req.Endpoint,
		IsActive: true,
	}

	if err := s.repo.CreateBucket(ctx, bucket); err != nil {
		return nil, fmt.Errorf("failed to create bucket in repository: %w", err)
	}

	evt := audit.NewEvent(actor.UserID, "user", core.EventBucketCreated).
		WithTarget(bucket.ID, "bucket", bucket.Name).
		WithSuccess().
		WithContext(ctx).
		Build()

	if err := s.auditRecorder.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return bucket, nil
}

func (s *AdminService) GetBucket(ctx context.Context, id uuid.UUID) (*storage.StorageBucket, error) {
	if _, err := s.authorize(ctx, storage.PermStorageAdmin); err != nil {
		return nil, err
	}
	return s.repo.GetBucket(ctx, id)
}

func (s *AdminService) ListBuckets(ctx context.Context) ([]*storage.StorageBucket, error) {
	if _, err := s.authorize(ctx, storage.PermStorageAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListBuckets(ctx)
}

func (s *AdminService) UpdateBucket(ctx context.Context, id uuid.UUID, req storage.UpdateBucketRequest) (*storage.StorageBucket, error) {
	actor, err := s.authorize(ctx, storage.PermStorageAdmin)
	if err != nil {
		return nil, err
	}

	var updatedBucket *storage.StorageBucket
	err = s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		bucket, err := txRepo.GetBucket(ctx, id)
		if err != nil {
			return err
		}
		updatedBucket = bucket

		var updated bool
		if req.IsActive != nil && bucket.IsActive != *req.IsActive {
			bucket.IsActive = *req.IsActive
			updated = true
		}
		if req.IsDefault != nil && *req.IsDefault && !bucket.IsDefault {
			if currentDefault, err := txRepo.GetDefaultBucket(ctx); err == nil && currentDefault.ID != bucket.ID {
				currentDefault.IsDefault = false
				if err := txRepo.UpdateBucket(ctx, currentDefault); err != nil {
					return fmt.Errorf("failed to unset current default bucket: %w", err)
				}
			}
			bucket.IsDefault = true
			updated = true
		}
		if updated {
			return txRepo.UpdateBucket(ctx, bucket)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if updatedBucket != nil {
		evt := audit.NewEvent(actor.UserID, "user", core.EventBucketUpdated).
			WithTarget(updatedBucket.ID, "bucket", updatedBucket.Name).
			WithSuccess().
			WithContext(ctx).
			Build()

		if err := s.auditRecorder.Record(ctx, evt); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}
	return updatedBucket, nil
}

func (s *AdminService) DeleteBucket(ctx context.Context, id uuid.UUID) error {
	actor, err := s.authorize(ctx, storage.PermStorageAdmin)
	if err != nil {
		return err
	}

	bucket, err := s.repo.GetBucket(ctx, id)
	if err != nil {
		return err
	}
	if bucket.IsDefault {
		return fmt.Errorf("%w: cannot delete the default bucket", storage.ErrInvalidInput)
	}

	_, total, err := s.repo.ListFiles(ctx, storage.FileFilter{PageSize: 1, Page: 0, BucketID: &id})
	if err != nil {
		return fmt.Errorf("failed to check if bucket is empty: %w", err)
	}
	if total > 0 {
		return storage.ErrBucketNotEmpty
	}

	if err := s.repo.DeleteBucket(ctx, id); err != nil {
		return err
	}

	evt := audit.NewEvent(actor.UserID, "user", core.EventBucketDeleted).
		WithTarget(id, "bucket", bucket.Name).
		WithSuccess().
		WithContext(ctx).
		Build()

	if err := s.auditRecorder.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return nil
}

func (s *AdminService) GetStorageQuota(ctx context.Context, ownerID uuid.UUID, ownerType string) (*storage.StorageQuota, error) {
	if _, err := s.authorize(ctx, storage.PermStorageAdmin); err != nil {
		return nil, err
	}
	return s.repo.GetOrCreateQuota(ctx, ownerID, ownerType, s.config.DefaultQuota)
}

func (s *AdminService) SetStorageQuota(ctx context.Context, ownerID uuid.UUID, ownerType string, limit int64) (*storage.StorageQuota, error) {
	actor, err := s.authorize(ctx, storage.PermStorageAdmin)
	if err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("%w: quota limit cannot be negative", storage.ErrInvalidInput)
	}

	quota, err := s.repo.GetOrCreateQuota(ctx, ownerID, ownerType, s.config.DefaultQuota)
	if err != nil {
		return nil, err
	}

	quota.Limit = limit
	if err := s.repo.UpdateQuota(ctx, quota); err != nil {
		return nil, err
	}

	evt := audit.NewEvent(actor.UserID, "user", core.EventQuotaUpdated).
		WithTarget(ownerID, ownerType, "").
		WithMetadata(map[string]interface{}{"new_limit_bytes": limit}).
		WithSuccess().
		WithContext(ctx).
		Build()

	if err := s.auditRecorder.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return quota, nil
}

// Maintenance & Cleanup

func (s *AdminService) CleanupExpiredUploads(ctx context.Context) (int64, error) {
	uploads, err := s.repo.FindExpiredUploads(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to find expired uploads: %w", err)
	}
	if len(uploads) == 0 {
		s.logger.Info("No expired uploads to clean up.")
		return 0, nil
	}
	s.logger.Info("Found expired uploads to process", log.Int("count", len(uploads)))

	var cleanedCount int64
	// --- Start of refactored code ---
	// Replace toolkit.BatchProcessor with a standard for loop.
	for i := 0; i < len(uploads); i += expiredUploadsBatchSize {
		end := i + expiredUploadsBatchSize
		if end > len(uploads) {
			end = len(uploads)
		}
		batch := uploads[i:end]

		for _, upload := range batch {
			err := s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
				if err := txRepo.DeleteFile(ctx, upload.FileID); err != nil && !errors.Is(err, storage.ErrFileNotFound) {
					s.logger.Error("Failed to delete placeholder file for expired upload", err, log.String("file_id", upload.FileID.String()))
				}
				return txRepo.DeleteUpload(ctx, upload.ID)
			})
			if err != nil {
				s.logger.Error("Failed to clean up expired upload", err, log.String("upload_id", upload.ID.String()))
			} else {
				cleanedCount++
			}
		}
	}
	// --- End of refactored code ---

	s.logger.Info("Expired upload cleanup complete", log.Int("cleaned_count", int(cleanedCount)))
	return cleanedCount, nil
}

func (s *AdminService) CleanupDeletedFiles(ctx context.Context, olderThan time.Duration) (int64, error) {
	if _, err := s.authorize(ctx, storage.PermStorageAdmin); err != nil {
		return 0, err
	}
	payload := map[string]interface{}{"older_than_days": int(olderThan.Hours() / 24)}
	job, err := s.dispatcher.Dispatch(ctx, storage.JobStorageCleanupDeleted, nil, queue.WithPayload(payload), queue.WithQueue("low"))
	if err != nil {
		return 0, fmt.Errorf("failed to dispatch cleanup job for deleted files: %w", err)
	}
	s.logger.Info("Dispatched job to clean up deleted files", log.String("job_id", job.ID.String()))
	return 1, nil
}

func (s *AdminService) ArchiveOldFiles(ctx context.Context, olderThan time.Duration) (int64, error) {
	if _, err := s.authorize(ctx, storage.PermStorageAdmin); err != nil {
		return 0, err
	}
	payload := map[string]interface{}{"older_than_days": int(olderThan.Hours() / 24)}
	job, err := s.dispatcher.Dispatch(ctx, storage.JobStorageArchiveOld, nil, queue.WithPayload(payload), queue.WithQueue("low"))
	if err != nil {
		return 0, fmt.Errorf("failed to dispatch archive job: %w", err)
	}
	s.logger.Info("Dispatched job to archive old files", log.String("job_id", job.ID.String()))
	return 1, nil
}

// --- Private Helper Methods ---

// authorize checks if the actor in the context has the required permission.
func (s *AdminService) authorize(ctx context.Context, permission core.PermissionName) (*auth.Actor, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}
	check := rbac.PermissionCheck{
		ActorID:        actor.UserID,
		PermissionName: permission,
	}
	isAllowed, err := s.rbacService.Authorize(ctx, check)
	if err != nil {
		s.logger.Error("RBAC authorization check failed", err)
		return nil, fmt.Errorf("authorization check failed: %w", err)
	}
	if !isAllowed {
		return nil, rbac.ErrAccessDenied
	}
	return actor, nil
}
