// Package currency formats monetary amounts for Africa OS locales.
//
// # Amount representation
//
// All amounts are passed as integer minor units (smallest currency subdivision):
//
//	NGN  100.50  →  10050  (kobo: 100 kobo = 1 NGN)
//	KES  200.00  →  20000  (cents: 100 cents = 1 KES)
//	XAF  1500    →  1500   (no subdivision: XAF has 0 decimal places per ISO 4217)
//	XOF  500     →  500    (same: 0 decimal places)
//
// Zero-decimal currencies (XAF, XOF, UGX, RWF, GNF, KMF) do not have
// a smaller sub-unit, so the caller passes the full unit value directly.
//
// # Currency vs locale
//
// Always pass both the ISO 4217 currency code AND the locale. The locale
// controls formatting style (decimal/grouping separators, numeral system),
// while the currency code controls symbol, decimal places, and symbol
// placement. A Nigerian wallet (locale en-NG) might hold XAF from a
// recent cross-border transfer — use Format(amount, "XAF", locale.EnNG).
package currency

import (
	"fmt"
	"math"
	"strings"

	"github.com/combi/pkg/i18n/locale"
)

// CurrencyInfo holds display metadata for an ISO 4217 currency.
type CurrencyInfo struct {
	Code          string
	DecimalPlaces int    // 0 for XAF/XOF, 2 for most, 3 for TND
	Symbol        string // primary symbol for all locales
	// SymbolByScript allows different symbols in different scripts:
	//   "Arab" → "ج.م." for EGP in Arabic contexts
	SymbolByScript map[locale.Script]string
	SymbolAfter    bool // true → "1 500 FCFA"; false → "₦1,500.50"
	SymbolSpace    bool // true → "KSh 100"; false → "₦100" (only used when !SymbolAfter)
}

// SymbolFor returns the display symbol for this currency in the given locale,
// choosing script-appropriate variants when available.
func (c CurrencyInfo) SymbolFor(l locale.Locale) string {
	if len(c.SymbolByScript) > 0 {
		script := locale.ScriptOf(l)
		if s, ok := c.SymbolByScript[script]; ok {
			return s
		}
	}
	return c.Symbol
}

// currencies is the authoritative registry of supported ISO 4217 currencies.
// Add new entries as markets open; see the pkg/compliance/currencies package
// for which currencies are legally tradeable per country.
var currencies = map[string]CurrencyInfo{
	// ── West / Central Africa ─────────────────────────────────────────────────
	"XAF": {
		Code: "XAF", DecimalPlaces: 0, Symbol: "FCFA",
		SymbolAfter: true, // "1 500 FCFA"
	},
	"XOF": {
		Code: "XOF", DecimalPlaces: 0, Symbol: "CFA",
		SymbolAfter: true, // "500 CFA"
	},
	"NGN": {
		Code: "NGN", DecimalPlaces: 2, Symbol: "₦",
		SymbolAfter: false, // "₦1,500.50"
	},
	"GHS": {
		Code: "GHS", DecimalPlaces: 2, Symbol: "GH₵",
		SymbolAfter: false,
	},
	"GNF": {
		Code: "GNF", DecimalPlaces: 0, Symbol: "FG",
		SymbolAfter: true,
	},
	// ── East Africa ───────────────────────────────────────────────────────────
	"KES": {
		Code: "KES", DecimalPlaces: 2, Symbol: "KSh",
		SymbolAfter: false, SymbolSpace: true, // "KSh 1,500.00"
	},
	"TZS": {
		Code: "TZS", DecimalPlaces: 2, Symbol: "TSh",
		SymbolAfter: false, SymbolSpace: true,
	},
	"UGX": {
		Code: "UGX", DecimalPlaces: 0, Symbol: "USh",
		SymbolAfter: false, SymbolSpace: true,
	},
	"RWF": {
		Code: "RWF", DecimalPlaces: 0, Symbol: "FRw",
		SymbolAfter: false,
	},
	"ETB": {
		Code: "ETB", DecimalPlaces: 2, Symbol: "ብር",
		SymbolAfter: true, // "100.00 ብር"
	},
	// ── Southern Africa ───────────────────────────────────────────────────────
	"ZAR": {
		Code: "ZAR", DecimalPlaces: 2, Symbol: "R",
		SymbolAfter: false,
	},
	"ZMW": {
		Code: "ZMW", DecimalPlaces: 2, Symbol: "ZK",
		SymbolAfter: false,
	},
	"MZN": {
		Code: "MZN", DecimalPlaces: 2, Symbol: "MT",
		SymbolAfter: true,
	},
	"AOA": {
		Code: "AOA", DecimalPlaces: 2, Symbol: "Kz",
		SymbolAfter: true,
	},
	"BWP": {
		Code: "BWP", DecimalPlaces: 2, Symbol: "P",
		SymbolAfter: false,
	},
	// ── North Africa ──────────────────────────────────────────────────────────
	"EGP": {
		Code: "EGP", DecimalPlaces: 2, Symbol: "E£",
		SymbolByScript: map[locale.Script]string{
			locale.ScriptArabic: "ج.م.",
		},
		// Symbol placement flips in Arabic script (RTL), handled in format().
		SymbolAfter: false,
	},
	"MAD": {
		Code: "MAD", DecimalPlaces: 2, Symbol: "MAD",
		SymbolAfter: true,
	},
	"DZD": {
		Code: "DZD", DecimalPlaces: 2, Symbol: "DA",
		SymbolAfter: true,
	},
	"TND": {
		Code: "TND", DecimalPlaces: 3, // unique in Africa
		Symbol:      "DT",
		SymbolAfter: true,
	},
	// ── Indian Ocean ──────────────────────────────────────────────────────────
	"MUR": {
		Code: "MUR", DecimalPlaces: 2, Symbol: "₨",
		SymbolAfter: false,
	},
	"KMF": {
		Code: "KMF", DecimalPlaces: 0, Symbol: "CF",
		SymbolAfter: false,
	},
}

// Lookup returns the CurrencyInfo for an ISO 4217 code and whether it was found.
func Lookup(code string) (CurrencyInfo, bool) {
	c, ok := currencies[strings.ToUpper(code)]
	return c, ok
}

// Format formats a monetary amount for display.
//
// amount is in minor units:
//   - Currencies with decimal places: 100 minor units = 1 major unit
//     (e.g., 150050 kobo = NGN 1,500.50)
//   - Currencies with 0 decimal places: minor unit IS the major unit
//     (e.g., 1500 = XAF 1,500)
//
// If currencyCode is unknown, Format returns a generic representation.
func Format(amount int64, currencyCode string, l locale.Locale) string {
	ci, ok := currencies[strings.ToUpper(currencyCode)]
	if !ok {
		// Unknown currency: generic fallback. Log a warning in production.
		return fmt.Sprintf("%d %s", amount, currencyCode)
	}

	meta, hasMeta := locale.Lookup(l)

	isNegative := amount < 0
	abs := amount
	if isNegative {
		abs = -abs
	}

	// Convert minor units to a display string.
	var displayAmount string
	if ci.DecimalPlaces == 0 {
		// No sub-unit: the amount IS the major unit.
		displayAmount = groupInteger(abs, sepFor(meta, hasMeta))
	} else {
		factor := int64(math.Pow10(ci.DecimalPlaces))
		major := abs / factor
		minor := abs % factor

		decSep := "."
		groupSep := ","
		if hasMeta {
			decSep = meta.DecimalSeparator
			groupSep = meta.GroupingSeparator
		}
		displayAmount = fmt.Sprintf("%s%s%0*d",
			groupInteger(major, groupSep),
			decSep,
			ci.DecimalPlaces,
			minor,
		)
	}

	// Apply numeral system transliteration (Eastern Arabic, Ethiopic).
	if hasMeta {
		displayAmount = transliterateDigits(displayAmount, meta.NumberScript)
	}

	sym := ci.Symbol
	if len(ci.SymbolByScript) > 0 {
		if hasMeta {
			if s, ok := ci.SymbolByScript[meta.Script]; ok {
				sym = s
			}
		}
	}

	var result string
	symbolAfter := ci.SymbolAfter
	// In RTL locales, symbol placement flips for currencies like EGP.
	if hasMeta && meta.Direction == locale.DirectionRTL {
		symbolAfter = !symbolAfter
	}

	if symbolAfter {
		result = displayAmount + "\u00A0" + sym // non-breaking space before suffix symbol
	} else if ci.SymbolSpace {
		result = sym + "\u00A0" + displayAmount
	} else {
		result = sym + displayAmount
	}

	if isNegative {
		result = "-" + result
	}
	return result
}

// FormatCompact formats large amounts with magnitude suffixes.
// 1_500_000 NGN → "₦1.5M"; 250_000 XAF → "250K FCFA"
func FormatCompact(amount int64, currencyCode string, l locale.Locale) string {
	ci, ok := currencies[strings.ToUpper(currencyCode)]
	if !ok {
		return Format(amount, currencyCode, l)
	}

	// Work in major units for threshold comparison.
	factor := int64(1)
	if ci.DecimalPlaces > 0 {
		factor = int64(math.Pow10(ci.DecimalPlaces))
	}
	major := amount / factor

	type threshold struct {
		value  int64
		suffix string
	}
	thresholds := []threshold{
		{1_000_000_000, "B"},
		{1_000_000, "M"},
		{1_000, "K"},
	}
	for _, t := range thresholds {
		if major >= t.value {
			compact := float64(major) / float64(t.value)
			// Format with one decimal place, dropping trailing .0.
			s := fmt.Sprintf("%.1f", compact)
			s = strings.TrimSuffix(s, ".0")
			// Re-assemble as minor units for consistent Format call.
			reconstructed := int64(compact*float64(t.value)) * factor
			_ = reconstructed
			// Build the compact string directly to avoid double-conversion.
			return buildCompact(s+t.suffix, ci, l)
		}
	}
	return Format(amount, currencyCode, l)
}

func buildCompact(compact string, ci CurrencyInfo, l locale.Locale) string {
	meta, hasMeta := locale.Lookup(l)
	sym := ci.Symbol
	if hasMeta && len(ci.SymbolByScript) > 0 {
		if s, ok := ci.SymbolByScript[meta.Script]; ok {
			sym = s
		}
	}
	if ci.SymbolAfter {
		return compact + "\u00A0" + sym
	}
	if ci.SymbolSpace {
		return sym + "\u00A0" + compact
	}
	return sym + compact
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func sepFor(meta locale.Meta, hasMeta bool) string {
	if hasMeta {
		return meta.GroupingSeparator
	}
	return ","
}

// groupInteger formats a non-negative integer with thousands separators.
func groupInteger(n int64, groupSep string) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, parts...), groupSep)
}

// transliterateDigits maps ASCII digits to locale-appropriate numeral systems.
func transliterateDigits(s string, ns locale.NumberScript) string {
	switch ns {
	case locale.NumberScriptArab:
		return mapRunes(s, func(r rune) string {
			if r >= '0' && r <= '9' {
				// Eastern Arabic: U+0660–U+0669
				return string(rune(0x0660 + (r - '0')))
			}
			return string(r)
		})
	case locale.NumberScriptEthi:
		// Ethiopic numbers use a multiplicative system for integers
		// (see numfmt.FormatEthiopicInteger), but for decimal fractions
		// we use individual digit mapping since the multiplicative system
		// only applies to whole numbers.
		// Format() already handles the integer part via groupInteger;
		// only the fractional digits need mapping here.
		// Ethiopic digit range: U+1369–U+1371 (1–9), 0 stays as "0"
		return mapRunes(s, func(r rune) string {
			if r >= '1' && r <= '9' {
				return string(rune(0x1368 + (r - '0')))
			}
			return string(r)
		})
	default:
		return s
	}
}

func mapRunes(s string, fn func(rune) string) string {
	var b strings.Builder
	b.Grow(len(s) * 3)
	for _, r := range s {
		b.WriteString(fn(r))
	}
	return b.String()
}
