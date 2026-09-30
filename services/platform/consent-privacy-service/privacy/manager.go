package privacy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
)

type PrivacyManager struct {
	repo        privacy.Repository
	anonymizers []privacy.Anonymizer
	exporters   []privacy.Exporter
	logger      log.Logger
}

func NewPrivacyManager(repo privacy.Repository, logger log.Logger) *PrivacyManager {
	return &PrivacyManager{
		repo:        repo,
		anonymizers: make([]privacy.Anonymizer, 0),
		exporters:   make([]privacy.Exporter, 0),
		logger:      logger.With(log.String("component", "privacy_manager")),
	}
}

func (m *PrivacyManager) RegisterAnonymizer(a privacy.Anonymizer) error {
	m.anonymizers = append(m.anonymizers, a)
	return nil
}

func (m *PrivacyManager) InitiateErasure(ctx context.Context, userID uuid.UUID, reason string) (*privacy.Request, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, apperror.New(http.StatusBadRequest, "INVALID_INPUT", "A reason for erasure is required.", nil)
	}

	// 1. Check for existing pending request
	existing, err := m.repo.GetPendingRequest(ctx, userID, privacy.TypeErasure)
	if err != nil && !apperror.IsType(err, apperror.NotFound) {
		// Real DB error
		return nil, apperror.New(http.StatusInternalServerError, "DB_ERROR", "Failed to check existing requests.", err)
	}
	if existing != nil {
		return nil, privacy.ErrRequestPending
	}

	// 2. Create Request
	req := &privacy.Request{
		ID:          uuid.New(),
		UserID:      userID,
		Type:        privacy.TypeErasure,
		Status:      privacy.StatusPending,
		RequestedAt: time.Now(),
		Reason:      reason,
	}

	if err := m.repo.SaveRequest(ctx, req); err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "DB_ERROR", "Failed to save privacy request.", err)
	}

	m.logger.Info("Privacy erasure request initiated", log.String("user_id", userID.String()), log.String("req_id", req.ID.String()))

	return req, nil
}

// ProcessErasure is called by the Worker to actually do the work.
func (m *PrivacyManager) ProcessErasure(ctx context.Context, reqID uuid.UUID) error {
	req, err := m.repo.GetRequest(ctx, reqID)
	if err != nil {
		return apperror.New(http.StatusNotFound, "REQUEST_NOT_FOUND", "Privacy request not found.", err)
	}

	// Update state to processing
	req.Status = privacy.StatusProcessing
	if err := m.repo.SaveRequest(ctx, req); err != nil {
		return err
	}

	totalDeleted := int64(0)
	totalAnon := int64(0)
	var errorList []string

	// Iterate over all registered services
	// Strategy: Best Effort. If one fails, we continue, but mark the request as requiring attention.
	for _, a := range m.anonymizers {
		report, err := a.AnonymizeUser(ctx, req.UserID)
		if err != nil {
			errMsg := fmt.Sprintf("Service failed: %v", err)
			errorList = append(errorList, errMsg)
			m.logger.Error("Anonymizer service failed", err,
				log.String("req_id", req.ID.String()),
				log.String("user_id", req.UserID.String()))
			continue
		}
		totalDeleted += report.RecordsDeleted
		totalAnon += report.RecordsAnonymized
	}

	now := time.Now()
	req.CompletedAt = &now

	details := &privacy.ErasureReport{
		Service:           "Aggregated",
		RecordsDeleted:    totalDeleted,
		RecordsAnonymized: totalAnon,
	}

	if len(errorList) > 0 {
		// If errors occurred, we might mark it as FAILED or leave it for manual review.
		// For legal compliance, we often treat partial failures as failures.
		req.Status = privacy.StatusFailed
		details.Details = "Partial failures: " + strings.Join(errorList, "; ")
	} else {
		req.Status = privacy.StatusCompleted
		details.Details = "Success"
	}

	req.ErasureDetails = details

	if err := m.repo.SaveRequest(ctx, req); err != nil {
		return apperror.New(http.StatusInternalServerError, "DB_ERROR", "Failed to save completion status.", err)
	}

	return nil
}

func (m *PrivacyManager) GetRequestStatus(ctx context.Context, requestID uuid.UUID) (*privacy.Request, error) {
	req, err := m.repo.GetRequest(ctx, requestID)
	if err != nil {
		return nil, apperror.New(http.StatusNotFound, "REQUEST_NOT_FOUND", "Privacy request not found.", err)
	}
	return req, nil
}

func (m *PrivacyManager) InitiateExport(ctx context.Context, userID uuid.UUID) (*privacy.Request, error) {
	// 1. Check for existing pending request
	existing, err := m.repo.GetPendingRequest(ctx, userID, privacy.TypeExport)
	if err != nil && !apperror.IsType(err, apperror.NotFound) {
		return nil, apperror.New(http.StatusInternalServerError, "DB_ERROR", "Failed to check existing requests.", err)
	}
	if existing != nil {
		return nil, privacy.ErrRequestPending
	}

	// 2. Create Request
	req := &privacy.Request{
		ID:          uuid.New(),
		UserID:      userID,
		Type:        privacy.TypeExport,
		Status:      privacy.StatusPending,
		RequestedAt: time.Now(),
	}

	if err := m.repo.SaveRequest(ctx, req); err != nil {
		return nil, apperror.New(http.StatusInternalServerError, "DB_ERROR", "Failed to save privacy request.", err)
	}

	m.logger.Info("Privacy export request initiated", log.String("user_id", userID.String()), log.String("req_id", req.ID.String()))

	return req, nil
}
