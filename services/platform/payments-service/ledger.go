package payments

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/log"
)

var _ financial.Ledger = (*LedgerService)(nil)

type LedgerService struct {
	logger log.Logger
}

func NewLedgerService(logger log.Logger) *LedgerService {
	return &LedgerService{
		logger: logger.With(log.String("component", "ledger_service")),
	}
}

func (s *LedgerService) Post(ctx context.Context, txRepo financial.Repository, journal *financial.JournalEntry) error {
	if journal == nil {
		return fmt.Errorf("%w: journal entry cannot be nil", financial.ErrInvalidInput)
	}

	if err := s.validateBalance(journal); err != nil {
		s.logger.Error("Ledger validation failed", err, log.String("transaction_id", journal.TransactionID.String()))
		return err
	}

	if err := txRepo.CreateJournalEntry(ctx, journal); err != nil {
		s.logger.Error("Failed to persist journal entry", err)
		return fmt.Errorf("failed to save ledger entry: %w", err)
	}

	return nil
}

func (s *LedgerService) validateBalance(journal *financial.JournalEntry) error {
	var totalDebits int64
	var totalCredits int64

	if len(journal.Entries) < 2 {
		return fmt.Errorf("%w: journal entry must have at least two legs", financial.ErrInvalidInput)
	}

	for _, entry := range journal.Entries {
		if entry.Amount <= 0 {
			return fmt.Errorf("%w: ledger entry amount must be positive, found %d", financial.ErrInvalidInput, entry.Amount)
		}

		switch entry.Direction {
		case financial.Debit:
			totalDebits += entry.Amount
		case financial.Credit:
			totalCredits += entry.Amount
		default:
			return fmt.Errorf("%w: invalid direction '%s'", financial.ErrInvalidInput, entry.Direction)
		}
	}

	if totalDebits != totalCredits {
		return fmt.Errorf("%w: ledger unbalanced - debits (%d) do not equal credits (%d)",
			financial.ErrInvalidInput, totalDebits, totalCredits)
	}

	return nil
}
