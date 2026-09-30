// audit-log-service — ported from api/internal/services/audit/export_worker.go.
package auditlog

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ queue.JobProcessor = (*ExportWorker)(nil)

// exportJobPayload defines the structure of the data coming from the queue.
type exportJobPayload struct {
	Filters     audit.EventFilters `json:"filters"`
	Format      string             `json:"format"`
	RequesterID uuid.UUID          `json:"requester_id"`
}

// ExportWorker generates audit log export files (CSV/JSONL), uploads them to
// the storage service, and notifies the requester.
type ExportWorker struct {
	repo       audit.EventRepository
	cryptoSvc  security.CryptoService
	storageSvc storage.Service      // nil until the storage module is mounted
	notifier   notification.Service // nil until the notification module is mounted
	logger     log.Logger
}

// NewExportWorker creates a new export worker.
func NewExportWorker(
	repo audit.EventRepository,
	cryptoSvc security.CryptoService,
	storageSvc storage.Service,
	notifier notification.Service,
	logger log.Logger,
) *ExportWorker {
	return &ExportWorker{
		repo:       repo,
		cryptoSvc:  cryptoSvc,
		storageSvc: storageSvc,
		notifier:   notifier,
		logger:     logger.With(log.String("component", "audit_export_worker")),
	}
}

func (w *ExportWorker) JobType() core.JobType {
	return audit.JobAuditExport
}

// SetNotificationService late-binds the notification service (used to alert
// the requester when an export completes). The host calls this after both
// modules are mounted; until then the worker completes exports without
// sending notifications. Mirrors api's "Injected later" wiring and breaks
// the constructor cycle with the notification module (which consumes
// audit.EventRecorder).
func (w *ExportWorker) SetNotificationService(ns notification.Service) {
	w.notifier = ns
}

// SetStorageService late-binds the storage service (used to upload export
// files). Same cycle shape as SetNotificationService: the storage module
// hard-requires audit.EventRecorder at construction. Mirrors api's
// InjectStorageService wiring.
func (w *ExportWorker) SetStorageService(ss storage.Service) {
	w.storageSvc = ss
}

// ProcessJob orchestrates the entire export process.
func (w *ExportWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload exportJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid payload for audit export job: %w", err))
	}

	logger := w.logger.With(
		log.String("job_id", job.ID.String()),
		log.String("format", payload.Format),
		log.String("requester_id", payload.RequesterID.String()),
	)

	logger.Info("starting audit log export")

	if w.storageSvc == nil || w.notifier == nil {
		// The storage/notification modules are not mounted in this host
		// yet. Re-queueing cannot help, so fail non-retryably.
		msg := "storage and/or notification service not configured"
		logger.Error(msg, nil)
		return nil, queue.NewNonRetryableError(fmt.Errorf("%s", msg))
	}

	var buffer bytes.Buffer
	var writer DataWriter
	switch payload.Format {
	case "csv":
		writer = NewCsvWriter(&buffer)
	case "jsonl":
		writer = NewJsonWriter(&buffer)
	default:
		return nil, queue.NewNonRetryableError(fmt.Errorf("format '%s' not supported", payload.Format))
	}

	if err := writer.WriteHeader(); err != nil {
		return nil, fmt.Errorf("could not write export file header: %w", err)
	}

	totalProcessed, err := w.processAndWriteBatches(ctx, payload.Filters, writer, logger)
	if err != nil {
		return nil, err // The error is already contextualized
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize writer: %w", err)
	}

	desc := "Audit log export generated on " + time.Now().UTC().Format(time.RFC3339)
	uploadReq := storage.UploadFileRequest{
		OriginalName: fmt.Sprintf("audit-export-%s.%s", time.Now().UTC().Format("2006-01-02"), payload.Format),
		OwnerID:      payload.RequesterID,
		OwnerType:    "user",
		MimeType:     writer.MimeType(),
		Visibility:   storage.VisibilityPrivate,
		Size:         int64(buffer.Len()),
		Description:  &desc,
	}
	file, err := w.storageSvc.UploadFile(ctx, uploadReq, &buffer)
	if err != nil {
		logger.Error("failed to upload export file to storage service", err)
		return nil, fmt.Errorf("storage service upload failed: %w", err)
	}

	signedURL, err := w.storageSvc.GetDownloadURL(ctx, file.ID, 24*time.Hour)
	if err != nil {
		logger.Error("failed to generate download URL for export file", err)
		return nil, fmt.Errorf("could not get download URL: %w", err)
	}

	dispatchReq := notification.DispatchRequest{
		RecipientID: payload.RequesterID,
		Template: &notification.TemplatePayload{
			Slug: "audit-export-ready",
			Data: map[string]interface{}{
				"download_url": signedURL.URL,
				"expires_at":   signedURL.ExpiresAt.Format(time.RFC1123),
			},
		},
	}
	if _, err := w.notifier.Dispatch(ctx, dispatchReq); err != nil {
		logger.Error("failed to send 'export ready' notification", err)
		// Non-fatal error; the export itself succeeded.
	}

	logger.Info("finished audit log export successfully", log.Int("records_processed", totalProcessed))
	resultPayload, _ := json.Marshal(map[string]string{"file_url": signedURL.URL})
	return resultPayload, nil
}

// processAndWriteBatches handles the pagination and decryption logic.
func (w *ExportWorker) processAndWriteBatches(ctx context.Context, filters audit.EventFilters, writer DataWriter, logger log.Logger) (int, error) {

	batchSize := 500 // Smaller batch for potentially large event data
	offset := 0
	filters.Limit = batchSize
	totalProcessed := 0

	for {
		filters.Offset = offset
		events, totalInDB, err := w.repo.SearchEvents(ctx, filters)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to query audit events for export: %w", err)
		}

		if len(events) == 0 {
			break // All events have been processed.
		}

		// Decrypt events before writing. This is a crucial step.
		for _, event := range events {
			if err := w.decryptEvent(ctx, event); err != nil {
				logger.Warn("failed to decrypt event during export, skipping record",
					log.String("event_id", event.ID.String()),
					log.ErrorAttr(err),
				)
				// Skip this problematic record and continue the export.
				continue
			}
		}

		if err := writer.WriteBatch(events); err != nil {
			return totalProcessed, fmt.Errorf("failed to write export batch: %w", err)
		}

		logger.Debug("processed export batch", log.Int("count", len(events)), log.Int("total_in_db", totalInDB))
		offset += len(events)
		totalProcessed += len(events)

		if offset >= totalInDB {
			break
		}
	}
	return totalProcessed, nil
}

// decryptEvent is a helper to handle the decryption logic for a single event.
func (w *ExportWorker) decryptEvent(ctx context.Context, event *audit.Event) error {
	if event.EncryptedData == nil || event.EncryptionContext == nil {
		return nil
	}
	plaintext, err := w.cryptoSvc.Decrypt(ctx, event.EncryptedData, event.EncryptionContext)
	if err != nil {
		return audit.ErrDecryptionFailed
	}
	var sensitiveData audit.SensitiveData
	if err := json.Unmarshal(plaintext, &sensitiveData); err != nil {
		return fmt.Errorf("failed to unmarshal decrypted data: %w", err)
	}
	event.Sensitive = sensitiveData
	return nil
}

// --- DataWriter Abstraction ---

type DataWriter interface {
	WriteHeader() error
	WriteBatch(events []*audit.Event) error
	Close() error
	MimeType() string
}

// CsvWriter implements DataWriter for CSV.
type CsvWriter struct{ writer *csv.Writer }

func NewCsvWriter(w io.Writer) *CsvWriter { return &CsvWriter{writer: csv.NewWriter(w)} }
func (cw *CsvWriter) MimeType() string    { return "text/csv" }
func (cw *CsvWriter) WriteHeader() error {
	return cw.writer.Write([]string{
		"id", "created_at", "actor_id", "actor_type", "action", "outcome", "outcome_reason",
		"target_id", "target_type", "actor_name", "target_name", "metadata", "ip_address",
	})
}
func (cw *CsvWriter) Close() error { cw.writer.Flush(); return cw.writer.Error() }

// JsonWriter implements DataWriter for JSONL (JSON Lines).
type JsonWriter struct{ encoder *json.Encoder }

func NewJsonWriter(w io.Writer) *JsonWriter { return &JsonWriter{encoder: json.NewEncoder(w)} }
func (jw *JsonWriter) MimeType() string     { return "application/jsonl" }
func (jw *JsonWriter) WriteHeader() error   { return nil } // No header for JSONL
func (jw *JsonWriter) WriteBatch(events []*audit.Event) error {
	for _, e := range events {
		if err := jw.encoder.Encode(e); err != nil {
			return err
		}
	}
	return nil
}
func (jw *JsonWriter) Close() error { return nil }

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func safeUUIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// WriteBatch renders each event as a CSV record.
func (cw *CsvWriter) WriteBatch(events []*audit.Event) error {
	for _, e := range events {
		// Use helper functions for all pointer types.
		record := []string{
			e.ID.String(),
			e.CreatedAt.Format(time.RFC3339),
			e.ActorID.String(),
			e.ActorType,
			string(e.Action),
			string(e.Outcome),
			safeString(e.OutcomeReason),
			safeUUIDString(e.TargetID),
			safeString(e.TargetType),
			e.Sensitive.ActorName,
			e.Sensitive.TargetName,
			string(e.Sensitive.Metadata),
			safeString(e.IPAddress),
		}
		if err := cw.writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}
