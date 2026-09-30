package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// =============================================================================
// RENEWAL WORKER
// Responsible for identifying due subscriptions and processing payments.
// =============================================================================

type RenewalWorker struct {
	service financial.Service
	repo    financial.Repository
	logger  log.Logger
}

func NewRenewalWorker(s financial.Service, r financial.Repository, l log.Logger) *RenewalWorker {
	return &RenewalWorker{
		service: s,
		repo:    r,
		logger:  l.With(log.String("worker", "financial_renewal")),
	}
}

func (w *RenewalWorker) JobType() core.JobType {
	return financial.JobFinancialRenewSubscriptions
}

// ProcessJob iterates through subscriptions expiring today.
// In a high-scale system, this would dispatch individual 'RenewSingleSubscription' jobs.
// For NGO Platform scale, batch processing in one worker is acceptable and simpler.
func (w *RenewalWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// 1. Define the cutoff (Renew anything ending before or right now)
	cutoff := time.Now().UTC()

	// 2. Fetch Due Subscriptions (Custom Repo Method assumed here)
	// We scan for Active subscriptions where CurrentPeriodEnd <= cutoff
	// Note: In a real implementation, you ensure this query is indexed.
	subs, err := w.repo.ScanDueSubscriptions(ctx, cutoff, 100) // Process 100 at a time
	if err != nil {
		return nil, fmt.Errorf("failed to scan due subscriptions: %w", err)
	}

	if len(subs) == 0 {
		w.logger.Info("No subscriptions due for renewal")
		return json.Marshal(map[string]string{"status": "no_work"})
	}

	w.logger.Info("Processing subscription batch", log.Int("count", len(subs)))

	results := map[string]int{
		"success": 0,
		"failed":  0,
		"skipped": 0,
	}

	// 3. Process Each Subscription
	for _, sub := range subs {
		if err := w.processSingleRenewal(ctx, sub); err != nil {
			w.logger.Error("Failed to renew subscription", err,
				log.String("subscription_id", sub.ID.String()),
				log.String("user_id", sub.UserID.String()))
			results["failed"]++
		} else {
			results["success"]++
		}
	}
	data, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (w *RenewalWorker) processSingleRenewal(ctx context.Context, sub *financial.Subscription) error {
	// A. Idempotency Check: Has an invoice already been generated for this period?
	// This prevents double billing if the worker crashes and restarts.
	existingInvoices, err := w.repo.ListInvoicesForUser(ctx, sub.UserID)
	if err == nil {
		for _, inv := range existingInvoices {
			if inv.SubscriptionID != nil && *inv.SubscriptionID == sub.ID {
				// Check if invoice covers the *next* period we are trying to bill
				// Logic depends on how CurrentPeriodEnd is managed.
				// Here we assume if there's an open invoice created recently, we skip.
				if inv.Status == financial.StatusInvoiceOpen && inv.CreatedAt.After(time.Now().Add(-24*time.Hour)) {
					return nil // Skip, already in progress
				}
			}
		}
	}

	// B. Fetch Price to calculate amount
	price, err := w.repo.GetPrice(ctx, sub.PriceID)
	if err != nil {
		return err
	}

	// C. Create Invoice (Transactional)
	var invoiceID uuid.UUID
	err = w.repo.WithTx(ctx, func(txRepo financial.Repository) error {
		nextStart := sub.CurrentPeriodEnd
		// Helper function from service package usually, duplicated logic here for worker isolation
		nextEnd := w.calculateNextEnd(nextStart, *price.Interval)

		inv := &financial.Invoice{
			ID:             uuid.New(),
			UserID:         sub.UserID,
			SubscriptionID: &sub.ID,
			Status:         financial.StatusInvoiceOpen,
			AmountDue:      price.Amount,
			Currency:       price.Currency,
			DueDate:        time.Now().UTC(), // Due Immediately
			PeriodStart:    nextStart,
			PeriodEnd:      nextEnd,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := txRepo.CreateInvoice(ctx, inv); err != nil {
			return err
		}
		invoiceID = inv.ID
		return nil
	})
	if err != nil {
		return err
	}

	// D. Attempt Payment
	// We call the Service layer here because it encapsulates the complex Gateway + Ledger logic
	// If payment fails, the Service handles updating the Invoice to 'UNCOLLECTIBLE'
	err = w.service.PayInvoice(ctx, invoiceID)
	if err != nil {
		// We return the error so it's logged, but the Invoice exists now (UNCOLLECTIBLE),
		// so the system state is consistent (user owes money).
		// We do NOT want to retry the job indefinitely if the card was declined.
		if errors.Is(err, financial.ErrPaymentDeclined) {
			return nil // Considered "Handled" business case
		}
		return err // Technical error, maybe retry?
	}

	return nil
}

func (w *RenewalWorker) calculateNextEnd(start time.Time, interval financial.Interval) time.Time {
	// Simple logic duplication for worker safety
	switch interval {
	case financial.IntervalMonth:
		return start.AddDate(0, 1, 0)
	case financial.IntervalYear:
		return start.AddDate(1, 0, 0)
	default:
		return start.AddDate(0, 1, 0)
	}
}

// =============================================================================
// REFUND WORKER
// Handles asynchronous refund processing to keep API latency low.
// =============================================================================

type RefundPayload struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	ActorID       uuid.UUID `json:"actor_id"`
	Amount        int64     `json:"amount"`
	Reason        string    `json:"reason"`
}

type RefundWorker struct {
	adminSvc financial.AdminService
	logger   log.Logger
}

func NewRefundWorker(s financial.AdminService, l log.Logger) *RefundWorker {
	return &RefundWorker{
		adminSvc: s,
		logger:   l.With(log.String("worker", "financial_refund")),
	}
}

func (w *RefundWorker) JobType() core.JobType {
	return financial.JobRefundTransaction
}

func (w *RefundWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload RefundPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		// Non-retryable error: The payload is corrupt.
		return nil, queue.NewNonRetryableError(fmt.Errorf("invalid job payload: %w", err))
	}

	w.logger.Info("Processing refund",
		log.String("tx_id", payload.TransactionID.String()),
		log.Int("amount", int(payload.Amount)))

	// Execute Refund
	tx, err := w.adminSvc.IssueRefund(ctx, payload.TransactionID, payload.ActorID, payload.Amount, payload.Reason)
	if err != nil {
		// Error Categorization
		// 1. If the transaction is already refunded, we treat it as success (Idempotency)
		if errors.Is(err, financial.ErrTransactionNotRefundable) {
			w.logger.Warn("Transaction not refundable (likely already processed)", log.String("tx_id", payload.TransactionID.String()))
			return nil, nil
		}

		// 2. If it's a validation error (e.g. amount > original), it's permanent.
		if errors.Is(err, financial.ErrInvalidInput) {
			return nil, queue.NewNonRetryableError(err)
		}

		// 3. Technical errors (Gateway down, DB lock) -> Retry
		return nil, err
	}

	return json.Marshal(map[string]string{
		"refund_tx_id": tx.ID.String(),
		"status":       string(tx.Status),
	})
}
