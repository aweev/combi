package payments

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ financial.AdminService = (*AdminService)(nil)

type AdminService struct {
	repo          financial.Repository
	gateway       financial.PaymentGateway
	ledger        financial.Ledger
	auditRecorder audit.EventRecorder
	dispatcher    queue.Dispatcher
	eventBus      events.Bus
	logger        log.Logger
}

func NewAdminService(
	repo financial.Repository,
	gateway financial.PaymentGateway,
	ledger financial.Ledger,
	auditRecorder audit.EventRecorder,
	dispatcher queue.Dispatcher,
	eventBus events.Bus,
	logger log.Logger,
) *AdminService {
	return &AdminService{
		repo:          repo,
		gateway:       gateway,
		ledger:        ledger,
		auditRecorder: auditRecorder,
		dispatcher:    dispatcher,
		eventBus:      eventBus,
		logger:        logger.With(log.String("service", "financial_admin")),
	}
}

func (s *AdminService) IssueRefund(ctx context.Context, transactionID, actorID uuid.UUID, amount int64, reason string) (*financial.Transaction, error) {
	var finalRefundTransaction *financial.Transaction
	actor := &auth.Actor{UserID: actorID} // Admin actor

	err := s.repo.WithTx(ctx, func(txRepo financial.Repository) error {
		originalTx, err := txRepo.GetTransaction(ctx, transactionID)
		if err != nil {
			return err
		}
		if originalTx.Status != financial.StatusSucceeded {
			return fmt.Errorf("%w: cannot refund transaction status '%s'", financial.ErrTransactionNotRefundable, originalTx.Status)
		}
		if amount <= 0 || amount > originalTx.Amount {
			return fmt.Errorf("%w: refund amount invalid", financial.ErrInvalidInput)
		}

		// 1. Gateway Refund
		refundResult, err := s.gateway.Refund(ctx, originalTx.ProviderID, amount, reason)
		if err != nil {
			return fmt.Errorf("payment gateway refund failed: %w", err)
		}

		// 2. Transaction Record
		refundTx := &financial.Transaction{
			ID:                   uuid.New(),
			UserID:               originalTx.UserID,
			Type:                 financial.TypeRefund,
			Status:               refundResult.Status,
			Amount:               amount,
			Currency:             originalTx.Currency,
			Description:          fmt.Sprintf("Refund: %s", reason),
			Provider:             originalTx.Provider,
			ProviderID:           refundResult.ProviderTransactionID,
			RelatedTransactionID: &originalTx.ID,
			ProcessedAt:          time.Now().UTC(),
		}
		if err := txRepo.CreateTransaction(ctx, refundTx); err != nil {
			return err
		}

		// 3. Ledger Post
		if refundTx.Status == financial.StatusSucceeded {
			if err := s.postRefundToLedger(ctx, txRepo, refundTx); err != nil {
				return err
			}
		}

		finalRefundTransaction = refundTx
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit & Events
	s.audit(ctx, actorID, core.EventTransactionRefunded, finalRefundTransaction.ID, "Refund issued")

	payload := map[string]interface{}{
		"refund_id":       finalRefundTransaction.ID,
		"original_tx_id":  transactionID,
		"amount_refunded": finalRefundTransaction.Amount,
		"reason":          reason,
	}
	_ = s.publishEvent(ctx, core.EventTransactionRefunded, actor, finalRefundTransaction.ID, "transaction", payload)

	return finalRefundTransaction, nil
}

func (s *AdminService) CreateProduct(ctx context.Context, params financial.CreateProductParams) (finalProduct *financial.Product, err error) {
	// 1. Create in Gateway
	providerProductID, err := s.gateway.CreateProduct(ctx, params.Name, params.Description)
	if err != nil {
		return nil, fmt.Errorf("gateway product creation failed: %w", err)
	}

	createdProviderPriceIDs := make([]string, 0, len(params.Prices))

	// Rollback closure
	defer func() {
		if err != nil {
			s.logger.Warn("Rolling back gateway creation due to DB failure", log.String("product_id", providerProductID))
			// Archive Prices
			for _, priceID := range createdProviderPriceIDs {
				_ = s.gateway.ArchivePrice(ctx, priceID)
			}
			// Archive Product
			_ = s.gateway.ArchiveProduct(ctx, providerProductID)
		}
	}()

	product := &financial.Product{
		ID:          uuid.New(),
		Type:        params.Type,
		Name:        params.Name,
		Description: params.Description,
		IsActive:    params.IsActive,
		Prices:      make([]financial.Price, 0, len(params.Prices)),
		CreatedAt:   time.Now().UTC(),
	}

	// 2. Create Prices in Gateway
	for _, p := range params.Prices {
		priceReq := financial.PriceCreateRequest{
			ProductProviderID: providerProductID,
			Amount:            p.Amount,
			Currency:          p.Currency,
			Interval:          p.Interval,
		}
		providerPriceID, priceErr := s.gateway.CreatePrice(ctx, priceReq)
		if priceErr != nil {
			err = fmt.Errorf("gateway price creation failed: %w", priceErr)
			return nil, err
		}
		createdProviderPriceIDs = append(createdProviderPriceIDs, providerPriceID)

		product.Prices = append(product.Prices, financial.Price{
			ID:              uuid.New(),
			ProductID:       product.ID,
			Amount:          p.Amount,
			Currency:        p.Currency,
			Interval:        p.Interval,
			ProviderPriceID: providerPriceID,
			IsActive:        p.IsActive,
		})
	}

	// 3. Save to DB
	if dbErr := s.repo.CreateProduct(ctx, product); dbErr != nil {
		err = fmt.Errorf("failed to create product in repository: %w", dbErr)
		return nil, err
	}

	s.logger.Info("Product created", log.String("id", product.ID.String()))
	return product, nil
}

func (s *AdminService) UpdateProduct(ctx context.Context, productID uuid.UUID, params financial.UpdateProductParams) (*financial.Product, error) {
	product, err := s.repo.GetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		product.Name = *params.Name
	}
	if params.Description != nil {
		product.Description = *params.Description
	}
	if params.IsActive != nil {
		product.IsActive = *params.IsActive
	}

	// Note: We update local DB only. Syncing name updates to Stripe is optional but usually good practice.
	if err := s.repo.UpdateProduct(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (s *AdminService) ListProducts(ctx context.Context) ([]*financial.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *AdminService) GetTransaction(ctx context.Context, transactionID uuid.UUID) (*financial.Transaction, error) {
	return s.repo.GetTransaction(ctx, transactionID)
}

func (s *AdminService) ListTransactions(ctx context.Context, filter financial.TransactionFilter) ([]*financial.Transaction, int, error) {
	return s.repo.ListTransactions(ctx, filter)
}

func (s *AdminService) DisburseGrant(ctx context.Context, params financial.DisburseGrantParams) (*financial.Transaction, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	var grantTransaction *financial.Transaction

	err := s.repo.WithTx(ctx, func(txRepo financial.Repository) error {
		tx := &financial.Transaction{
			ID:          uuid.New(),
			UserID:      &params.RecipientUserID,
			Type:        financial.TypePayout,
			Status:      financial.StatusSucceeded, // Assumed instantaneous for internal grant records
			Amount:      params.Amount,
			Currency:    params.Currency,
			Description: params.Description,
			Provider:    "internal",
			ProviderID:  params.ReferenceID.String(),
			ProcessedAt: time.Now().UTC(),
		}

		if err := txRepo.CreateTransaction(ctx, tx); err != nil {
			return err
		}

		if err := s.postDisbursementToLedger(ctx, txRepo, tx); err != nil {
			return err
		}

		grantTransaction = tx
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.audit(ctx, actor.UserID, core.EventAction("financial.grant.disbursed"), grantTransaction.ID, "Grant Disbursed")
	return grantTransaction, nil
}

// --- Helpers ---

func (s *AdminService) postRefundToLedger(ctx context.Context, txRepo financial.Repository, tx *financial.Transaction) error {
	// Reverses the logic: Revenue (Debit), Cash (Credit)
	clearingAccount, err := txRepo.GetLedgerAccountByName(ctx, "Stripe Clearing")
	if err != nil {
		return err
	}
	revenueAccount, err := txRepo.GetLedgerAccountByName(ctx, "General Revenue")
	if err != nil {
		return err
	}

	journal := &financial.JournalEntry{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		Description:   "Refund processed",
		PostedAt:      time.Now().UTC(),
		Entries: []financial.LedgerEntry{
			{ID: uuid.New(), AccountID: revenueAccount.ID, Direction: financial.Debit, Amount: tx.Amount},
			{ID: uuid.New(), AccountID: clearingAccount.ID, Direction: financial.Credit, Amount: tx.Amount},
		},
	}
	return s.ledger.Post(ctx, txRepo, journal)
}

func (s *AdminService) postDisbursementToLedger(ctx context.Context, txRepo financial.Repository, tx *financial.Transaction) error {
	bankAccount, err := txRepo.GetLedgerAccountByName(ctx, "Bank Account EUR")
	if err != nil {
		return err
	}
	grantExpenseAccount, err := txRepo.GetLedgerAccountByName(ctx, "Grant Expense")
	if err != nil {
		return err
	}

	journal := &financial.JournalEntry{
		ID:            uuid.New(),
		TransactionID: tx.ID,
		Description:   tx.Description,
		PostedAt:      time.Now().UTC(),
		Entries: []financial.LedgerEntry{
			{ID: uuid.New(), AccountID: grantExpenseAccount.ID, Direction: financial.Debit, Amount: tx.Amount}, // Expense Debit
			{ID: uuid.New(), AccountID: bankAccount.ID, Direction: financial.Credit, Amount: tx.Amount},        // Asset Credit
		},
	}
	return s.ledger.Post(ctx, txRepo, journal)
}

func (s *AdminService) audit(ctx context.Context, actorID uuid.UUID, action core.EventAction, targetID uuid.UUID, msg string) {
	b := audit.NewEvent(actorID, "user", action).WithTarget(targetID, "financial", msg).WithSuccess().WithContext(ctx)
	if err := s.auditRecorder.Record(context.Background(), b.Build()); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *AdminService) publishEvent(ctx context.Context, action core.EventAction, actor *auth.Actor, targetID uuid.UUID, targetType string, payload any) error {
	evt, _ := events.NewEvent(action, actor.UserID, "user", payload)
	evt.TargetID = &targetID
	evt.TargetType = &targetType
	return s.eventBus.Publish(ctx, evt)
}
