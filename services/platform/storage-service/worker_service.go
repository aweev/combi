// internal/services/storage/worker_service.go
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// workerService implements the storage.WorkerService interface.
type workerService struct {
	repo         storage.Repository
	providerFct  ProviderFactory
	processorFct ProcessorFactory
	audit        audit.EventRecorder
	logger       log.Logger
	config       storage.ServiceConfig
}

// NewWorkerService creates a new instance of the service that handles background processing jobs.
func NewWorkerService(deps Dependencies, cfg storage.ServiceConfig) storage.WorkerService {
	return &workerService{
		repo:         deps.Repo,
		providerFct:  deps.ProviderFct,
		processorFct: deps.ProcessorFct,
		audit:        deps.AuditRecorder,
		logger:       deps.Logger.With(log.String("service", "storage_worker")),
		config:       cfg,
	}
}

// Statically verify interface implementation.
var _ storage.WorkerService = (*workerService)(nil)

// ProcessImage is the entry point for the image processing worker.
func (s *workerService) ProcessImage(ctx context.Context, fileID uuid.UUID, variants map[string]string) error {
	log := s.logger.With(log.String("file_id", fileID.String()), log.String("task_type", "IMAGE"))
	log.Info("Starting image processing")

	return s.processFile(ctx, fileID, "IMAGE_PROCESSOR", map[string]interface{}{
		"variants": variants,
	})
}

// ProcessVideo is the entry point for the video processing worker.
func (s *workerService) ProcessVideo(ctx context.Context, fileID uuid.UUID, formats []string, qualities []string) error {
	log := s.logger.With(log.String("file_id", fileID.String()), log.String("task_type", "VIDEO"))
	log.Info("Starting video processing")

	return s.processFile(ctx, fileID, "VIDEO_PROCESSOR", map[string]interface{}{
		"formats":   formats,
		"qualities": qualities,
	})
}

// ProcessDocument is the entry point for the document processing worker.
func (s *workerService) ProcessDocument(ctx context.Context, fileID uuid.UUID, operations []string) error {
	log := s.logger.With(log.String("file_id", fileID.String()), log.String("task_type", "DOCUMENT"))
	log.Info("Starting document processing")

	return s.processFile(ctx, fileID, "DOCUMENT_PROCESSOR", map[string]interface{}{
		"operations": operations,
	})
}

// processFile is the generic, core logic for all processing tasks.
func (s *workerService) processFile(ctx context.Context, fileID uuid.UUID, taskType string, options map[string]interface{}) error {
	// 1. Fetch file and provider details.
	file, provider, bucket, err := s.getFileAndProvider(ctx, fileID)
	if err != nil {
		return err // Error is already logged by the helper.
	}

	// 2. Create a temporary local file to work with.
	tempFile, err := os.CreateTemp("", fmt.Sprintf("storage-process-%s-*%s", file.ID, filepath.Ext(file.OriginalName)))
	if err != nil {
		return s.handleProcessingError(ctx, file, "failed to create temp file", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 3. Download the original file from the provider to our local temp file.
	reader, err := provider.Download(ctx, bucket.Name, file.Path)
	if err != nil {
		return s.handleProcessingError(ctx, file, "failed to download original file for processing", err)
	}
	defer reader.Close()

	if _, err := io.Copy(tempFile, reader); err != nil {
		return s.handleProcessingError(ctx, file, "failed to write to temp file", err)
	}

	// 4. Get the correct processor for the task.
	processor, err := s.processorFct.GetProcessor(taskType)
	if err != nil {
		return s.handleProcessingError(ctx, file, "processor not found for task", err)
	}

	// 5. Execute the processing task.
	// We inject the provider, bucket, and base path so the processor can upload variants.
	options["provider"] = provider
	options["bucket"] = bucket.Name
	options["basePath"] = file.Path
	options["file"] = file // Pass the full file object for context
	result, err := processor.Process(ctx, tempFile.Name(), options)
	if err != nil {
		return s.handleProcessingError(ctx, file, "file processing execution failed", err)
	}

	// 6. Update the file record in the database with the processing results.
	return s.finalizeProcessing(ctx, file, result)
}

// getFileAndProvider is a helper to fetch common objects needed for processing.
func (s *workerService) getFileAndProvider(ctx context.Context, fileID uuid.UUID) (*storage.File, storage.Provider, *storage.StorageBucket, error) {
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		s.logger.Error("Could not find file for processing", err, log.String("file_id", fileID.String()))
		return nil, nil, nil, err
	}
	bucket, err := s.repo.GetBucket(ctx, file.BucketID)
	if err != nil {
		return nil, nil, nil, s.handleProcessingError(ctx, file, "could not find bucket for file", err)
	}
	provider, err := s.providerFct.GetProvider(ctx, bucket)
	if err != nil {
		return nil, nil, nil, s.handleProcessingError(ctx, file, "could not get provider for bucket", err)
	}
	return file, provider, bucket, nil
}

// finalizeProcessing updates the file's metadata and status in the database.
func (s *workerService) finalizeProcessing(ctx context.Context, file *storage.File, result *storage.ProcessingResult) error {
	s.logger.Info("Finalizing file processing", log.String("file_id", file.ID.String()))

	// Merge the results from the processor into the file's metadata.
	if file.CustomMetadata == nil {
		file.CustomMetadata = make(map[string]interface{})
	}
	for key, value := range result.Data {
		file.CustomMetadata[key] = value
	}

	file.Status = storage.StatusAvailable
	file.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateFile(ctx, file); err != nil {
		// This is a critical error. The file was processed, but we couldn't update the DB.
		// The job will retry, but this indicates a potential DB issue.
		s.logger.Error("CRITICAL: Failed to update file status after successful processing", err, log.String("file_id", file.ID.String()))
		return err
	}

	// Record a successful audit event.
	payloadBytes, _ := json.Marshal(result.Data)
	auditEvent := audit.NewEvent(file.OwnerID, "system", core.EventFileProcessingCompleted).
		WithTarget(file.ID, "file", file.OriginalName).
		WithSuccess().
		WithMetadata(map[string]interface{}{"result": string(payloadBytes)}).
		Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event for processing completion", err)
	}

	s.logger.Info("Successfully finalized file processing", log.String("file_id", file.ID.String()))
	return nil
}

// handleProcessingError is a centralized helper to mark a file as errored and log details.
func (s *workerService) handleProcessingError(ctx context.Context, file *storage.File, message string, procErr error) error {
	s.logger.Error(message, procErr, log.String("file_id", file.ID.String()))

	file.Status = storage.StatusError
	if err := s.repo.UpdateFile(ctx, file); err != nil {
		s.logger.Error("CRITICAL: Failed to update file status to ERROR after a processing failure", err, log.String("file_id", file.ID.String()))
		// Return the original error to ensure the job is retried.
		return procErr
	}

	auditEvent := audit.NewEvent(file.OwnerID, "system", core.EventFileProcessingFailed).
		WithTarget(file.ID, "file", file.OriginalName).
		WithFailure(procErr.Error()).
		Build()
	if err := s.audit.Record(context.Background(), auditEvent); err != nil {
		s.logger.Error("failed to record audit event for processing failure", err)
	}

	return procErr // Return the original error to let the queue manager handle retries.
}
