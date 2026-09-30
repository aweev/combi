package money

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoney_Allocation(t *testing.T) {
	t.Run("Split $100 50/50", func(t *testing.T) {
		m := New(100.00, "USD")
		parts, err := m.Allocate(50, 50)
		require.NoError(t, err)

		// Expect: 50.00, 50.00
		assert.Equal(t, "50.00", parts[0].Amount().StringFixed(2))
		assert.Equal(t, "50.00", parts[1].Amount().StringFixed(2))
	})

	t.Run("Split $100 into 3 parts (33.33...)", func(t *testing.T) {
		// In float math: 100/3 = 33.3333...
		// In money: 33.34, 33.33, 33.33 -> Sum is 100.00
		m := New(100.00, "USD")
		parts, err := m.Allocate(1, 1, 1) // Equal 3rds
		require.NoError(t, err)

		// Expect: 33.34, 33.33, 33.33 (Sum = 100.00)
		assert.Equal(t, "33.34", parts[0].Amount().StringFixed(2))
		assert.Equal(t, "33.33", parts[1].Amount().StringFixed(2))
		assert.Equal(t, "33.33", parts[2].Amount().StringFixed(2))

		sum, _ := parts[0].Add(parts[1])
		sum, _ = sum.Add(parts[2])
		assert.Equal(t, "100.00", sum.Amount().StringFixed(2))
	})

	t.Run("Split $0.05 50/50", func(t *testing.T) {
		m := New(0.05, "USD")
		parts, err := m.Allocate(50, 50)
		require.NoError(t, err)

		// Expect: 0.03, 0.02
		assert.Equal(t, "0.03", parts[0].Amount().StringFixed(2))
		assert.Equal(t, "0.02", parts[1].Amount().StringFixed(2))
	})
}

func TestMoney_Operations(t *testing.T) {
	usd := New(10.00, "USD")
	eur := New(10.00, "EUR")

	t.Run("Add Mismatch", func(t *testing.T) {
		_, err := usd.Add(eur)
		assert.ErrorIs(t, err, ErrCurrencyMismatch)
	})

	t.Run("Multiplication", func(t *testing.T) {
		res := usd.Mul(decimal.NewFromFloat(1.5))

		// Fix: Use StringFixed(2) to match "15.00".
		// decimal.String() returns "15" because it strips trailing zeros.
		assert.Equal(t, "15.00", res.Amount().StringFixed(2))
	})
}
