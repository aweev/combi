package currency_test

import (
	"testing"

	"github.com/combi/pkg/i18n/currency"
	"github.com/combi/pkg/i18n/locale"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name         string
		amount       int64
		currencyCode string
		loc          locale.Locale
		want         string
	}{
		{
			name:         "NGN standard format (2 decimals, symbol before, no space)",
			amount:       520050,
			currencyCode: "NGN",
			loc:          locale.EnNG,
			want:         "₦5,200.50",
		},
		{
			name:         "KES format (2 decimals, symbol before, with NBSP)",
			amount:       10000,
			currencyCode: "KES",
			loc:          locale.SwKE,
			// Implementation uses \u00A0 (Non-breaking space) for spacing
			want: "KSh\u00A0100.00",
		},
		{
			name:         "TZS format (2 decimals, symbol before, with NBSP)",
			amount:       123450,
			currencyCode: "TZS",
			loc:          locale.SwTZ,
			want:         "TSh\u00A01,234.50",
		},
		{
			name: "XOF format (0 decimals, symbol after)",
			// 10000 minor units = 10,000 major units for 0-decimal currencies
			amount:       10000,
			currencyCode: "XOF",
			loc:          locale.YoBJ,
			want:         "10\u202f000\u00A0CFA",
		},
		{
			name:         "TND format (3 decimals, unique in Africa)",
			amount:       1500250, // 1,500.250 DT
			currencyCode: "TND",
			loc:          locale.EnNG, // Testing TND logic using an existing locale
			want:         "1,500.250\u00A0DT",
		},
		{
			name:         "Negative amount format",
			amount:       -150050,
			currencyCode: "NGN",
			loc:          locale.EnNG,
			want:         "-₦1,500.50",
		},
		{
			name:         "Fallback for unknown currency",
			amount:       404,
			currencyCode: "ZZZ",
			loc:          locale.EnNG,
			want:         "404 ZZZ",
		},
		{
			name:         "Case-insensitive currency code lookup",
			amount:       10050,
			currencyCode: "ngn",
			loc:          locale.EnNG,
			want:         "₦100.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currency.Format(tt.amount, tt.currencyCode, tt.loc)
			if got != tt.want {
				t.Errorf("Format(%d, %q) = %q, want %q", tt.amount, tt.currencyCode, got, tt.want)
			}
		})
	}
}

func TestFormatCompact(t *testing.T) {
	tests := []struct {
		name         string
		amount       int64
		currencyCode string
		loc          locale.Locale
		want         string
	}{
		{
			name:         "Millions suffix (NGN 2 decimals)",
			amount:       150000000, // 1,500,000.00 minor units -> 1.5M major units
			currencyCode: "NGN",
			loc:          locale.EnNG,
			want:         "₦1.5M",
		},
		{
			name:         "Thousands suffix (XAF 0 decimals)",
			amount:       250000, // 250,000 minor units -> 250K major units
			currencyCode: "XAF",
			loc:          locale.EnNG, // Testing XAF logic using an existing locale
			want:         "250K\u00A0FCFA",
		},
		{
			name:         "Billions suffix (KES 2 decimals, spaced)",
			amount:       200000000000, // 2,000,000,000.00 minor units
			currencyCode: "KES",
			loc:          locale.SwKE,
			want:         "KSh\u00A02B",
		},
		{
			name:         "Below thousand threshold fallback",
			amount:       99900, // 999.00 minor units
			currencyCode: "NGN",
			loc:          locale.EnNG,
			want:         "₦999.00", // Not compacted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currency.FormatCompact(tt.amount, tt.currencyCode, tt.loc)
			if got != tt.want {
				t.Errorf("FormatCompact(%d, %q) = %q, want %q", tt.amount, tt.currencyCode, got, tt.want)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	tests := []struct {
		code string
		want bool
		sym  string
	}{
		{"NGN", true, "₦"},
		{"ngn", true, "₦"}, // Ensure case-insensitivity behaves correctly
		{"XAF", true, "FCFA"},
		{"INVALID", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			info, ok := currency.Lookup(tt.code)
			if ok != tt.want {
				t.Errorf("Lookup(%q) ok = %v, want %v", tt.code, ok, tt.want)
			}
			if ok && info.Symbol != tt.sym {
				t.Errorf("Lookup(%q).Symbol = %q, want %q", tt.code, info.Symbol, tt.sym)
			}
		})
	}
}
