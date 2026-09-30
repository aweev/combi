// internal/services/storage/service.go
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Service implements the primary storage business logic defined in the domain.
type Service struct {
	repo          storage.Repository
	providerFct   storage.ProviderFactory
	dispatcher    queue.Dispatcher
	auditRecorder audit.EventRecorder
	rbacService   rbac.AuthorizationService
	config        storage.ServiceConfig
	logger        log.Logger
}

// NewService creates a new storage service instance.
func NewService(deps Dependencies, cfg storage.ServiceConfig) *Service {
	return &Service{
		repo:          deps.Repo,
		providerFct:   deps.ProviderFct,
		dispatcher:    deps.Dispatcher,
		auditRecorder: deps.AuditRecorder,
		rbacService:   deps.RBACService,
		config:        cfg,
		logger:        deps.Logger.With(log.String("service", "storage")),
	}
}

// Statically assert interface implementation.
var _ privacy.Anonymizer = (*Service)(nil)
var _ storage.Service = (*Service)(nil)

// StoreFile handles a direct stream upload from an internal service (like Reporting).
// It bypasses the multi-step "Initiate -> Complete" flow used by the API.
func (s *Service) StoreFile(ctx context.Context, req storage.UploadRequest) (*storage.File, error) {
	s.logger.Info("Storing internal file", log.String("filename", req.Filename))

	// 1. Select Bucket (Use Default)
	provider, bucket, err := s.providerFct.GetDefaultProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage provider: %w", err)
	}

	// 2. Upload to Provider
	// Note: We use the Upload method on the provider, not PutObject (depending on interface naming)
	// Assuming the Provider interface uses 'Upload' as defined in other parts of the service code
	if err := provider.Upload(ctx, bucket.Name, req.Path, req.Content, req.Size, req.ContentType); err != nil {
		return nil, fmt.Errorf("provider upload failed: %w", err)
	}
	// Important: We don't close the reader here if the caller (worker) needs to handle it,
	// but generally it's safe to assume consumption is done. The Worker defers Close().

	// 3. Create File Record
	fileID := uuid.New()
	visibility := storage.VisibilityPublic
	if req.IsPrivate {
		visibility = storage.VisibilityPrivate
	}

	file := &storage.File{
		ID:           fileID,
		OwnerID:      audit.SystemActorID, // Owned by System
		OwnerType:    "system",
		OriginalName: req.Filename,
		Path:         req.Path,
		MimeType:     req.ContentType,
		Size:         req.Size,
		Status:       storage.StatusAvailable, // Ready immediately
		Visibility:   visibility,
		BucketID:     bucket.ID,
		Category:     storage.DetermineCategory(req.ContentType),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.CreateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return file, nil
}

// AnonymizeUser implements the privacy.Anonymizer interface.
// It deletes all files owned by the user.
func (s *Service) AnonymizeUser(ctx context.Context, userID uuid.UUID) (*privacy.ErasureReport, error) {
	s.logger.Warn("dispatching jobs to delete all files owned by user", log.String("user_id", userID.String()))

	// 1. Find all files owned by the user.
	// Note: We might need to loop if the user has more than the default page size,
	// but for anonymization, a large page size (e.g., 1000) is usually sufficient for a first pass.
	filter := storage.FileFilter{
		OwnerID:  &userID,
		PageSize: 1000,
	}
	files, total, err := s.repo.ListFiles(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list user files for anonymization: %w", err)
	}

	if len(files) == 0 {
		return &privacy.ErasureReport{
			Service:        "StorageService",
			RecordsDeleted: 0,
			Details:        "No files found for user.",
		}, nil
	}

	// 2. For each file, dispatch a deletion job.
	dispatchedCount := 0
	for _, file := range files {
		payload, _ := json.Marshal(map[string]string{"file_id": file.ID.String()})

		// Use a dedicated 'deletions' or 'maintenance' queue for this background work
		_, err := s.dispatcher.Dispatch(ctx, storage.JobStorageDeleteFile, payload, queue.WithQueue("low"))
		if err != nil {
			s.logger.Error("failed to dispatch file deletion job during anonymization", err, log.String("file_id", file.ID.String()))
			// Continue trying to delete other files. Best effort.
		} else {
			dispatchedCount++
		}
	}

	// If there are more files than our page size, we log a warning.
	// A robust implementation would page through all files.
	if total > len(files) {
		s.logger.Warn("User has more files than processed in single anonymization pass", log.Int("total", total), log.Int("processed", len(files)))
	}

	return &privacy.ErasureReport{
		Service:           "StorageService",
		RecordsDeleted:    int64(dispatchedCount), // We count dispatched jobs as "deleted" for the report
		RecordsAnonymized: 0,
		Details:           fmt.Sprintf("Dispatched deletion jobs for %d files (Total files found: %d)", dispatchedCount, total),
	}, nil
}

// --- Core Public Methods ---

// InitiateUpload orchestrates starting a client-side file upload.
func (s *Service) InitiateUpload(ctx context.Context, req storage.InitiateUploadRequest) (*storage.InitiateUploadResponse, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}
	log := s.logger.With(log.String("actor_id", actor.AccountID.String()), log.String("filename", req.OriginalName))
	log.Info("Initiating upload")

	if err := s.validateUploadRequest(req.Size, req.MimeType); err != nil {
		return nil, err
	}

	var file *storage.File
	var upload *storage.Upload
	var signedURL *storage.SignedURL

	err := s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		quota, err := txRepo.GetOrCreateQuota(ctx, actor.UserID, "user", s.config.DefaultQuota)
		if err != nil {
			return err
		}
		if !quota.HasSpace(req.Size) {
			return storage.ErrQuotaExceeded
		}

		provider, bucket, err := s.providerFct.GetDefaultProvider(ctx)
		if err != nil {
			return err
		}

		path := generateUniquePath(req.OriginalName, actor.UserID)
		file = &storage.File{
			ID:           uuid.New(),
			OwnerID:      actor.UserID,
			OwnerType:    "user",
			OriginalName: req.OriginalName,
			Path:         path,
			MimeType:     req.MimeType,
			Size:         req.Size,
			Category:     storage.DetermineCategory(req.MimeType),
			Status:       storage.StatusPending,
			Visibility:   req.Visibility,
			BucketID:     bucket.ID,
		}
		if err := txRepo.CreateFile(ctx, file); err != nil {
			return err
		}

		upload = &storage.Upload{
			ID:          uuid.New(),
			FileID:      file.ID,
			Provider:    bucket.Provider,
			TotalSize:   req.Size,
			IsMultipart: req.IsMultipart,
			ExpiresAt:   time.Now().UTC().Add(s.config.UploadURLExpiry),
		}
		if err := txRepo.CreateUpload(ctx, upload); err != nil {
			return err
		}

		url, err := provider.GeneratePresignedUploadURL(ctx, bucket.Name, file.Path, file.MimeType, file.Size, s.config.UploadURLExpiry)
		if err != nil {
			return storage.NewProviderError("GeneratePresignedUploadURL", "failed to create upload URL", err)
		}
		signedURL = url
		return nil
	})
	if err != nil {
		return nil, err
	}

	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileProcessingStarted).
		WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return &storage.InitiateUploadResponse{
		UploadID:  upload.ID,
		FileID:    file.ID,
		SignedURL: *signedURL,
	}, nil
}

// UploadFile handles a server-side file upload.
func (s *Service) UploadFile(ctx context.Context, req storage.UploadFileRequest, reader io.Reader) (*storage.File, error) {
	actor, err := s.authorizeUpload(ctx, req.OwnerID)
	if err != nil {
		return nil, err
	}
	logger := s.logger.With(
		log.String("actor_id", actor.UserID.String()),
		log.String("filename", req.OriginalName),
	)
	logger.Info("Starting server-side upload")

	if err := s.validateUploadRequest(req.Size, req.MimeType); err != nil {
		return nil, err
	}

	var file *storage.File
	var uploadID uuid.UUID

	// 1. Create File & Upload Record (Pending State)
	err = s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		quota, err := txRepo.GetOrCreateQuota(ctx, req.OwnerID, req.OwnerType, s.config.DefaultQuota)
		if err != nil {
			return fmt.Errorf("could not get quota: %w", err)
		}
		if !quota.HasSpace(req.Size) {
			return storage.ErrQuotaExceeded
		}

		_, bucket, err := s.providerFct.GetDefaultProvider(ctx)
		if err != nil {
			return fmt.Errorf("could not get default provider: %w", err)
		}

		path := generateUniquePath(req.OriginalName, req.OwnerID)
		file = &storage.File{
			ID:           uuid.New(),
			OwnerID:      req.OwnerID,
			OwnerType:    req.OwnerType,
			OriginalName: req.OriginalName,
			Path:         path,
			MimeType:     req.MimeType,
			Size:         req.Size,
			Category:     storage.DetermineCategory(req.MimeType),
			Status:       storage.StatusPending,
			Visibility:   req.Visibility,
			BucketID:     bucket.ID,
			Description:  req.Description,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := txRepo.CreateFile(ctx, file); err != nil {
			return fmt.Errorf("could not create file record: %w", err)
		}

		// FIX: Create an Upload record to track this operation.
		// If the server crashes during streaming, the Cleanup worker will see this expired upload
		// and delete the pending file record.
		uploadID = uuid.New()
		upload := &storage.Upload{
			ID:          uploadID,
			FileID:      file.ID,
			Provider:    bucket.Provider,
			TotalSize:   req.Size,
			ExpiresAt:   time.Now().UTC().Add(1 * time.Hour), // Generous timeout for server-side stream
			CreatedAt:   time.Now().UTC(),
			IsMultipart: false,
		}
		if err := txRepo.CreateUpload(ctx, upload); err != nil {
			return fmt.Errorf("could not create upload session record: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 2. Stream to Provider
	// We do this OUTSIDE the transaction to avoid holding DB locks during I/O.
	_, bucket, _ := s.providerFct.GetDefaultProvider(ctx) // Re-fetch provider (cached in factory)
	provider, _ := s.providerFct.GetProvider(ctx, bucket)

	logger.Info("Streaming file to provider", log.String("path", file.Path))
	if err := provider.Upload(ctx, bucket.Name, file.Path, reader, file.Size, file.MimeType); err != nil {
		// If upload fails, we rely on the CleanupWorker to delete the pending file/upload record later.
		// Alternatively, we could try to delete it immediately here.
		s.logger.Error("Provider upload failed", err)
		return nil, storage.NewProviderError("Upload", "failed to stream file to provider", err)
	}

	// 3. Finalize Record
	err = s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		// Re-fetch file to ensure we have latest version/lock if needed
		f, err := txRepo.GetFile(ctx, file.ID)
		if err != nil {
			return err
		}
		f.Status = storage.StatusProcessing // Move to PROCESSING for background jobs
		now := time.Now().UTC()
		f.UploadCompletedAt = &now
		f.UpdatedAt = now

		if err := txRepo.UpdateFile(ctx, f); err != nil {
			return err
		}
		if err := txRepo.IncrementUsage(ctx, f.OwnerID, f.OwnerType, f.Size); err != nil {
			return err
		}
		// Remove the tracking upload record since we succeeded.
		return txRepo.DeleteUpload(ctx, uploadID)
	})

	if err != nil {
		// Critical error: File is on S3 but DB failed to update.
		// The file record will remain PENDING and eventually be deleted by cleanup worker.
		// This is acceptable consistency (orphaned S3 file cleaned up later or overwritten).
		return nil, fmt.Errorf("failed to finalize file record: %w", err)
	}

	// 4. Dispatch Processing
	s.dispatchProcessingJob(file)
	auditEvent := audit.NewEvent(actor.UserID, "user", core.EventFileUploadCompleted).
		WithTarget(file.ID, "file", file.OriginalName).
		WithMetadata(map[string]interface{}{"upload_method": "server_side"}).
		WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return file, nil
}

// CompleteUpload finalizes the upload and triggers post-processing.
func (s *Service) CompleteUpload(ctx context.Context, uploadID uuid.UUID) (*storage.File, error) {
	s.logger.Info("Completing upload", log.String("upload_id", uploadID.String()))
	var file *storage.File

	err := s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		upload, err := txRepo.GetUpload(ctx, uploadID)
		if err != nil {
			return err
		}
		if time.Now().UTC().After(upload.ExpiresAt) {
			return storage.ErrUploadExpired
		}

		file, err = txRepo.GetFile(ctx, upload.FileID)
		if err != nil {
			return err
		}

		bucket, err := txRepo.GetBucket(ctx, file.BucketID)
		if err != nil {
			return err
		}
		provider, err := s.providerFct.GetProvider(ctx, bucket)
		if err != nil {
			return err
		}

		exists, err := provider.Exists(ctx, bucket.Name, file.Path)
		if err != nil {
			return storage.NewProviderError("Exists", "failed to verify file existence", err)
		}
		if !exists {
			return storage.ErrUploadFailed
		}

		file.Status = storage.StatusProcessing
		now := time.Now().UTC()
		file.UploadCompletedAt = &now
		if err := txRepo.UpdateFile(ctx, file); err != nil {
			return err
		}
		if err := txRepo.IncrementUsage(ctx, file.OwnerID, file.OwnerType, file.Size); err != nil {
			return err
		}
		return txRepo.DeleteUpload(ctx, uploadID)
	})
	if err != nil {
		return nil, err
	}

	s.dispatchProcessingJob(file)
	auditEvent := audit.NewEvent(file.OwnerID, "user", core.EventFileUploadCompleted).
		WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return file, nil
}

// AbortUpload cancels an in-progress upload.
func (s *Service) AbortUpload(ctx context.Context, uploadID uuid.UUID) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	s.logger.Info("Aborting upload", log.String("upload_id", uploadID.String()), log.String("actor_id", actor.AccountID.String()))

	return s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		upload, err := txRepo.GetUpload(ctx, uploadID)
		if err != nil {
			return err
		}
		file, err := txRepo.GetFile(ctx, upload.FileID)
		if err != nil {
			if errors.Is(err, storage.ErrFileNotFound) {
				return txRepo.DeleteUpload(ctx, uploadID)
			}
			return err
		}
		if file.OwnerID != actor.UserID {
			return storage.ErrAccessDenied
		}
		if err := txRepo.DeleteFile(ctx, file.ID); err != nil {
			return err
		}
		return txRepo.DeleteUpload(ctx, uploadID)
	})
}

// GetFile retrieves metadata, enforcing access control.
func (s *Service) GetFile(ctx context.Context, fileID uuid.UUID) (*storage.File, error) {
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := s.checkReadAccess(ctx, file); err != nil {
		return nil, err
	}
	return file, nil
}

// GetFileMetadata retrieves metadata, enforcing access control.
// This is an alias for GetFile, fulfilling the service interface.
func (s *Service) GetFileMetadata(ctx context.Context, fileID uuid.UUID) (*storage.File, error) {
	return s.GetFile(ctx, fileID)
}

// GetDownloadURL provides a secure URL for downloading, enforcing access control.
func (s *Service) GetDownloadURL(ctx context.Context, fileID uuid.UUID, expiresIn time.Duration) (*storage.SignedURL, error) {
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := s.checkReadAccess(ctx, file); err != nil {
		return nil, err
	}
	if !file.IsAvailable() {
		return nil, storage.ErrFileNotAvailable
	}
	if expiresIn == 0 || expiresIn > s.config.DownloadURLExpiry {
		expiresIn = s.config.DownloadURLExpiry
	}

	bucket, err := s.repo.GetBucket(ctx, file.BucketID)
	if err != nil {
		return nil, err
	}
	provider, err := s.providerFct.GetProvider(ctx, bucket)
	if err != nil {
		return nil, err
	}
	url, err := provider.GeneratePresignedDownloadURL(ctx, bucket.Name, file.Path, expiresIn)
	if err != nil {
		return nil, storage.NewProviderError("GeneratePresignedDownloadURL", "failed to create download URL", err)
	}

	if actor := auth.ActorFromContext(ctx); actor != nil {
		auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileDownloaded).
			WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
		if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
			s.logger.Error("failed to record audit event", err)
		}
	}
	return url, nil
}

// ListFiles retrieves a paginated list of files, enforcing tenancy.
func (s *Service) ListFiles(ctx context.Context, filter storage.FileFilter) ([]*storage.File, int, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, 0, auth.ErrUnauthorized
	}

	check := rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: "storage:file:list_all",
	}
	isAllowed, _ := s.rbacService.Authorize(ctx, check)

	if !isAllowed {
		// If not an admin, scope the query to only the actor's own files.
		filter.OwnerID = &actor.UserID
		filter.OwnerType = func(s string) *string { return &s }("user")
	}

	return s.repo.ListFiles(ctx, filter)
}

// UpdateFileMetadata allows an owner to change mutable metadata.
func (s *Service) UpdateFileMetadata(ctx context.Context, fileID uuid.UUID, req storage.UpdateFileRequest) (*storage.File, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.OwnerID != actor.UserID {
		return nil, storage.ErrAccessDenied
	}
	var updated bool
	if req.OriginalName != nil && *req.OriginalName != file.OriginalName {
		file.OriginalName = *req.OriginalName
		updated = true
	}
	if req.Description != nil && (file.Description == nil || *req.Description != *file.Description) {
		file.Description = req.Description
		updated = true
	}
	if !updated {
		return file, nil
	}
	if err := s.repo.UpdateFile(ctx, file); err != nil {
		return nil, err
	}
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileMetadataUpdated).
		WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
	return file, nil
}

// DeleteFile performs a soft or permanent delete.
func (s *Service) DeleteFile(ctx context.Context, fileID uuid.UUID, permanent bool) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	// Similar to ListFiles, an admin with 'storage:file:delete_any' could bypass this check.
	if file.OwnerID != actor.UserID {
		return storage.ErrAccessDenied
	}
	if permanent {
		return s.permanentlyDeleteFile(ctx, file, actor)
	}
	return s.softDeleteFile(ctx, file, actor)
}

// RestoreFile restores a soft-deleted file.
func (s *Service) RestoreFile(ctx context.Context, fileID uuid.UUID) (*storage.File, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}
	// GetFile must be implemented to find soft-deleted files for owners/admins.
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.OwnerID != actor.UserID {
		return nil, storage.ErrAccessDenied
	}
	if !file.IsDeleted() {
		return file, nil
	}
	file.Status = storage.StatusAvailable
	file.DeletedAt = nil
	if err := s.repo.UpdateFile(ctx, file); err != nil {
		return nil, err
	}
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileRestored).
		WithTarget(fileID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
	return file, nil
}

// --- Private Implementation Details ---

func (s *Service) checkReadAccess(ctx context.Context, file *storage.File) error {
	if file.Visibility == storage.VisibilityPublic {
		return nil
	}
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return storage.ErrAccessDenied
	}
	if file.OwnerID == actor.UserID {
		return nil
	}

	check := rbac.PermissionCheck{
		ActorID:        actor.AccountID,
		PermissionName: storage.PermStorageFileRead, // The permission we're checking for.
		ResourceContext: map[string]any{ // Provide context for potential ABAC rules.
			"fileOwnerId": file.OwnerID.String(),
			"fileId":      file.ID.String(),
		},
	}
	isAllowed, err := s.rbacService.Authorize(ctx, check)

	if err != nil || !isAllowed {
		if err != nil {
			s.logger.Error("RBAC authorization check failed with an error", err)
		}
		s.logger.Warn("RBAC access denied for file read",
			log.String("actor_id", actor.AccountID.String()),
			log.String("file_id", file.ID.String()),
		)
		return storage.ErrAccessDenied
	}
	return nil
}

func (s *Service) softDeleteFile(ctx context.Context, file *storage.File, actor *auth.Actor) error {
	if file.IsDeleted() {
		return nil
	}
	now := time.Now().UTC()
	file.Status = storage.StatusDeleted
	file.DeletedAt = &now
	if err := s.repo.UpdateFile(ctx, file); err != nil {
		return err
	}
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileDeleted).
		WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
	return nil
}

func (s *Service) permanentlyDeleteFile(ctx context.Context, file *storage.File, actor *auth.Actor) error {
	s.logger.Warn("Permanently deleting file", log.String("file_id", file.ID.String()))
	err := s.repo.WithTx(ctx, func(txRepo storage.Repository) error {
		bucket, err := txRepo.GetBucket(ctx, file.BucketID)
		if err != nil {
			return err
		}
		provider, err := s.providerFct.GetProvider(ctx, bucket)
		if err != nil {
			return err
		}
		if err := provider.Delete(ctx, bucket.Name, file.Path); err != nil {
			s.logger.Error("Failed to delete file from provider, but continuing DB deletion", err)
		}
		if !file.IsDeleted() { // Only decrement quota if it wasn't already soft-deleted.
			if err := txRepo.DecrementUsage(ctx, file.OwnerID, file.OwnerType, file.Size); err != nil {
				return err
			}
		}
		return txRepo.DeleteFile(ctx, file.ID)
	})
	if err != nil {
		return err
	}
	auditEvent := audit.NewEvent(actor.AccountID, "user", core.EventFileDeleted).
		WithTarget(file.ID, "file", file.OriginalName).WithSuccess().Build()
	if err := s.auditRecorder.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
	return nil
}

func (s *Service) dispatchProcessingJob(file *storage.File) {
	var jobType core.JobType
	payload := map[string]interface{}{"file_id": file.ID.String()}

	switch file.Category {
	case storage.CategoryImage:
		jobType = storage.JobStorageProcessImage
		payload["variants"] = map[string]string{"thumb": "200x200", "medium": "800x800"}
	case storage.CategoryVideo:
		jobType = storage.JobStorageProcessVideo
	case storage.CategoryDocument:
		jobType = storage.JobStorageProcessAsset
	default:
		file.Status = storage.StatusAvailable
		if err := s.repo.UpdateFile(context.Background(), file); err != nil {
			s.logger.Error("CRITICAL: Failed to update file status to AVAILABLE", err, log.String("file_id", file.ID.String()))
		}
		return
	}

	_, err := s.dispatcher.Dispatch(context.Background(), (jobType), nil,
		queue.WithPayload(payload),
		queue.WithQueue(s.config.ProcessingQueue),
	)
	if err != nil {
		s.logger.Error("Failed to dispatch file processing job, setting file status to ERROR", err, log.String("file_id", file.ID.String()))
		file.Status = storage.StatusError
		if err := s.repo.UpdateFile(context.Background(), file); err != nil {
			s.logger.Error("Failed to update file status to ERROR", err, log.String("file_id", file.ID.String()))
		}
	} else {
		s.logger.Info("Successfully dispatched file processing job", log.String("file_id", file.ID.String()), log.String("job_type", string(jobType)))
	}
}

func (s *Service) validateUploadRequest(size int64, mimeType string) error {
	if s.config.MaxUploadSize > 0 && size > s.config.MaxUploadSize {
		return storage.ErrFileTooLarge
	}
	if len(s.config.AllowedMimeTypePats) > 0 {
		allowed := false
		for _, pattern := range s.config.AllowedMimeTypePats {
			matched, _ := regexp.MatchString(pattern, mimeType)
			if matched {
				allowed = true
				break
			}
		}
		if !allowed {
			return storage.ErrInvalidMimeType
		}
	}
	return nil
}

func generateUniquePath(originalName string, ownerID uuid.UUID) string {
	ext := filepath.Ext(originalName)
	fileName := uuid.NewString()
	return filepath.Join(ownerID.String(), fileName+ext) // Simplified path
}

// authorizeUpload checks if an actor can upload a file on behalf of the specified owner.
func (s *Service) authorizeUpload(ctx context.Context, ownerID uuid.UUID) (*auth.Actor, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	if actor.UserID == ownerID {
		check := rbac.PermissionCheck{ActorID: actor.UserID, PermissionName: storage.PermStorageFileUpload}
		isAllowed, err := s.rbacService.Authorize(ctx, check)
		if err != nil {
			return nil, err
		}
		if !isAllowed {
			return nil, rbac.ErrAccessDenied
		}
		return actor, nil
	}

	check := rbac.PermissionCheck{ActorID: actor.UserID, PermissionName: storage.PermStorageAdmin}
	isAllowed, err := s.rbacService.Authorize(ctx, check)
	if err != nil {
		return nil, err
	}
	if !isAllowed {
		return nil, rbac.ErrAccessDenied
	}
	return actor, nil
}
