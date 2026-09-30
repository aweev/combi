package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrInvalidAmount    = errors.New("invalid amount")
)

// Money represents a monetary value in a specific currency.
// It is immutable.
type Money struct {
	amount   decimal.Decimal
	currency Currency
}

// New creates a new Money instance from a float (be careful with floats)
func New(amount float64, currencyCode string) Money {
	return Money{
		amount:   decimal.NewFromFloat(amount),
		currency: GetCurrency(currencyCode),
	}
}

// NewFromDecimal creates Money from a decimal
func NewFromDecimal(amount decimal.Decimal, currencyCode string) Money {
	return Money{
		amount:   amount,
		currency: GetCurrency(currencyCode),
	}
}

// NewFromCents creates Money from minor units (e.g., 100 cents = $1.00)
func NewFromCents(cents int64, currencyCode string) Money {
	cur := GetCurrency(currencyCode)
	amount := decimal.New(cents, int32(-cur.Exponent))
	return Money{
		amount:   amount,
		currency: cur,
	}
}

// --- Getters ---

func (m Money) Amount() decimal.Decimal { return m.amount }
func (m Money) Currency() Currency      { return m.currency }
func (m Money) IsZero() bool            { return m.amount.IsZero() }
func (m Money) IsNegative() bool        { return m.amount.IsNegative() }

// AmountFloat is useful for JSON responses, but use carefully.
func (m Money) AmountFloat() float64 {
	f, _ := m.amount.Float64()
	return f
}

// --- Arithmetic ---

func (m Money) Add(other Money) (Money, error) {
	if m.currency.Code != other.currency.Code {
		return Money{}, fmt.Errorf("%w: cannot add %s to %s", ErrCurrencyMismatch, other.currency.Code, m.currency.Code)
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency.Code != other.currency.Code {
		return Money{}, fmt.Errorf("%w: cannot subtract %s from %s", ErrCurrencyMismatch, other.currency.Code, m.currency.Code)
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

func (m Money) Mul(multiplier decimal.Decimal) Money {
	return Money{amount: m.amount.Mul(multiplier), currency: m.currency}
}

// Allocate splits the money according to a list of ratios, ensuring no pennies are lost.
// Example: Split $0.05 into [50, 50] ratios -> [$0.03, $0.02] (or vice versa).
// This is CRITICAL for NGOs splitting donations into overhead vs program costs.
func (m Money) Allocate(ratios ...int) ([]Money, error) {
	if len(ratios) == 0 {
		return nil, errors.New("no ratios provided")
	}

	totalRatio := 0
	for _, r := range ratios {
		totalRatio += r
	}
	if totalRatio == 0 {
		return nil, errors.New("total ratio cannot be zero")
	}

	// Convert to minor units (cents) to handle integer division
	minorUnits := m.amount.Shift(int32(m.currency.Exponent)).IntPart()

	remainder := minorUnits
	results := make([]Money, len(ratios))

	for i, ratio := range ratios {
		// Calculate share: (Total * Ratio) / TotalRatios
		share := (minorUnits * int64(ratio)) / int64(totalRatio)
		results[i] = NewFromCents(share, m.currency.Code)
		remainder -= share
	}

	// Distribute the remainder (the lost pennies) to the first results
	for i := 0; i < int(remainder); i++ {
		// Add 1 cent to results[i]
		penny := NewFromCents(1, m.currency.Code)
		sum, _ := results[i].Add(penny)
		results[i] = sum
	}

	return results, nil
}

// --- Serialization ---

// MarshalJSON marshals Money as a simple struct.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount    decimal.Decimal `json:"amount"`
		Currency  string          `json:"currency"`
		Formatted string          `json:"formatted"`
	}{
		Amount:    m.amount,
		Currency:  m.currency.Code,
		Formatted: m.String(),
	})
}

// UnmarshalJSON expects { "amount": "100.50", "currency": "USD" }
func (m *Money) UnmarshalJSON(data []byte) error {
	var temp struct {
		Amount   decimal.Decimal `json:"amount"`
		Currency string          `json:"currency"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	m.amount = temp.Amount
	m.currency = GetCurrency(temp.Currency)
	return nil
}

// Value implements driver.Valuer for SQL.
// Caution: This stores just the numeric value if used on a single column.
// Ideally, use a Composite Type in Postgres or Embed this in a struct that handles 2 columns.
func (m Money) Value() (driver.Value, error) {
	return m.amount.String(), nil
}

// String implements fmt.Stringer
func (m Money) String() string {
	return fmt.Sprintf("%s%s", m.currency.Symbol, m.amount.StringFixed(int32(m.currency.Exponent)))
}
