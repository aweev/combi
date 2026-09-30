package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/combi/pkg/events"
	walletv1 "github.com/combi/gen/go/combi/wallet/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidCurrency     = errors.New("invalid currency")
)

// LedgerEntry represents a double-entry bookkeeping journal line.
type LedgerEntry struct {
	ID        string
	WalletID  string
	Amount    int64 // Subunits
	Currency  walletv1.CurrencyType
	Type      string // "CREDIT" or "DEBIT"
	Reference string // EscrowID or PaymentID
	Status    string // "PENDING", "SETTLED", "FAILED"
	CreatedAt time.Time
}

// Escrow represents funds locked for a tournament or marketplace.
type Escrow struct {
	ID        string
	WalletID  string
	Reference string
	Amount    int64
	Currency  walletv1.CurrencyType
	Status    string // "HELD", "RELEASED", "REFUNDED"
	CreatedAt time.Time
}

// WalletStore manages wallet balances and escrow state.
type WalletStore interface {
	GetBalance(identityID string, currency walletv1.CurrencyType) (*walletv1.Balance, error)
	Credit(identityID string, amount int64, currency walletv1.CurrencyType) error
	LockFunds(walletID string, amount int64, currency walletv1.CurrencyType) error
	ReleaseFunds(escrowID string, splits map[string]int64) error
	RecordEscrow(escrow *Escrow) error
	GetEscrow(escrowID string) (*Escrow, error)
}

// WalletManager coordinates wallet and ledger workflows using Event Sagas over NATS.
type WalletManager struct {
	store   WalletStore
	emitter events.Bus
}

func NewWalletManager(store WalletStore, emitter events.Bus) *WalletManager {
	return &WalletManager{
		store:   store,
		emitter: emitter,
	}
}

func (m *WalletManager) GetBalance(ctx context.Context, req *walletv1.GetBalanceRequest) (*walletv1.GetBalanceResponse, error) {
	balance, err := m.store.GetBalance(req.IdentityId, req.Currency)
	if err != nil {
		return nil, err
	}
	return &walletv1.GetBalanceResponse{
		Balances: []*walletv1.Balance{balance},
	}, nil
}

func (m *WalletManager) BuyCoins(ctx context.Context, req *walletv1.BuyCoinsRequest) (*walletv1.BuyCoinsResponse, error) {
	// Start Saga: BuyCoins (Credit virtual coins)
	// We record the intent and immediately dispatch an event to verify payment.
	
	err := m.store.Credit(req.IdentityId, req.CoinsToCredit, walletv1.CurrencyType_CURRENCY_TYPE_COINS)
	if err != nil {
		return nil, err
	}

	// Dispatch integration event using NATS
	_ = m.emitter.Publish(ctx, events.Event{
		ID:            req.IdempotencyKey,
		Topic:         "wallet.coins.purchased",
		Source:        "wallet-ledger-service",
		TenantID:      "default",
		CorrelationID: req.PaymentId,
		OccurredAt:    time.Now().UTC(),
	})

	balance, _ := m.store.GetBalance(req.IdentityId, walletv1.CurrencyType_CURRENCY_TYPE_COINS)
	
	return &walletv1.BuyCoinsResponse{
		UpdatedAvailableCoins: balance.AvailableSubunit,
	}, nil
}

func (m *WalletManager) HoldEscrow(ctx context.Context, req *walletv1.HoldEscrowRequest) (*walletv1.HoldEscrowResponse, error) {
	// Start Saga: HoldEscrow
	err := m.store.LockFunds(req.WalletId, req.AmountSubunit, req.Currency)
	if err != nil {
		return nil, err
	}

	escrow := &Escrow{
		ID:        "esc_" + req.IdempotencyKey[:8], // Simple ID generation for scaffold
		WalletID:  req.WalletId,
		Reference: req.ReferenceId,
		Amount:    req.AmountSubunit,
		Currency:  req.Currency,
		Status:    "HELD",
		CreatedAt: time.Now().UTC(),
	}

	if err := m.store.RecordEscrow(escrow); err != nil {
		return nil, err
	}

	_ = m.emitter.Publish(ctx, events.Event{
		ID:            escrow.ID,
		Topic:         "wallet.escrow.held",
		Source:        "wallet-ledger-service",
		TenantID:      "default",
		CorrelationID: req.ReferenceId,
		OccurredAt:    time.Now().UTC(),
	})

	return &walletv1.HoldEscrowResponse{
		EscrowId: escrow.ID,
		Success:  true,
	}, nil
}

func (m *WalletManager) ReleaseEscrow(ctx context.Context, req *walletv1.ReleaseEscrowRequest) (*walletv1.ReleaseEscrowResponse, error) {
	err := m.store.ReleaseFunds(req.EscrowId, req.DestinationSubunitSplits)
	if err != nil {
		return nil, err
	}

	_ = m.emitter.Publish(ctx, events.Event{
		ID:            req.IdempotencyKey,
		Topic:         "wallet.escrow.released",
		Source:        "wallet-ledger-service",
		TenantID:      "default",
		CorrelationID: req.EscrowId,
		OccurredAt:    time.Now().UTC(),
	})

	return &walletv1.ReleaseEscrowResponse{
		Success: true,
	}, nil
}

// InMemoryWalletStore for scaffolding
type InMemoryWalletStore struct {
	mu       sync.RWMutex
	balances map[string]map[walletv1.CurrencyType]*walletv1.Balance
	escrows  map[string]*Escrow
}

func NewInMemoryWalletStore() *InMemoryWalletStore {
	return &InMemoryWalletStore{
		balances: make(map[string]map[walletv1.CurrencyType]*walletv1.Balance),
		escrows:  make(map[string]*Escrow),
	}
}

func (s *InMemoryWalletStore) GetBalance(identityID string, currency walletv1.CurrencyType) (*walletv1.Balance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if currMap, ok := s.balances[identityID]; ok {
		if bal, ok := currMap[currency]; ok {
			return bal, nil
		}
	}
	
	return &walletv1.Balance{
		AvailableSubunit:   0,
		PendingSubunit:     0,
		LockedSubunit:      0,
		Currency:           currency,
		EstimatedReleaseAt: timestamppb.New(time.Now().UTC()),
	}, nil
}

func (s *InMemoryWalletStore) Credit(identityID string, amount int64, currency walletv1.CurrencyType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.balances[identityID]; !ok {
		s.balances[identityID] = make(map[walletv1.CurrencyType]*walletv1.Balance)
	}

	if _, ok := s.balances[identityID][currency]; !ok {
		s.balances[identityID][currency] = &walletv1.Balance{Currency: currency}
	}

	s.balances[identityID][currency].AvailableSubunit += amount
	return nil
}

func (s *InMemoryWalletStore) LockFunds(walletID string, amount int64, currency walletv1.CurrencyType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if currMap, ok := s.balances[walletID]; ok {
		if bal, ok := currMap[currency]; ok {
			if bal.AvailableSubunit < amount {
				return ErrInsufficientBalance
			}
			bal.AvailableSubunit -= amount
			bal.LockedSubunit += amount
			return nil
		}
	}
	return ErrInsufficientBalance
}

func (s *InMemoryWalletStore) RecordEscrow(escrow *Escrow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.escrows[escrow.ID] = escrow
	return nil
}

func (s *InMemoryWalletStore) GetEscrow(escrowID string) (*Escrow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.escrows[escrowID]
	if !ok {
		return nil, errors.New("escrow not found")
	}
	return e, nil
}

func (s *InMemoryWalletStore) ReleaseFunds(escrowID string, splits map[string]int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	escrow, ok := s.escrows[escrowID]
	if !ok {
		return errors.New("escrow not found")
	}

	if escrow.Status != "HELD" {
		return errors.New("escrow is not held")
	}

	// For scaffold, we assume splits are valid and total escrow.Amount
	for destWallet, amount := range splits {
		if _, ok := s.balances[destWallet]; !ok {
			s.balances[destWallet] = make(map[walletv1.CurrencyType]*walletv1.Balance)
		}
		if _, ok := s.balances[destWallet][escrow.Currency]; !ok {
			s.balances[destWallet][escrow.Currency] = &walletv1.Balance{Currency: escrow.Currency}
		}
		s.balances[destWallet][escrow.Currency].AvailableSubunit += amount
	}

	// Remove from locked
	s.balances[escrow.WalletID][escrow.Currency].LockedSubunit -= escrow.Amount
	escrow.Status = "RELEASED"

	return nil
}
