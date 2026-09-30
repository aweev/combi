package numfmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/combi/pkg/i18n/locale"
)

// Maps standard Western Arabic digits to Eastern Arabic digits.
var easternDigits = [10]string{"٠", "١", "٢", "٣", "٤", "٥", "٦", "٧", "٨", "٩"}
var ethiopicDigits = [10]string{"0", "፩", "፪", "፫", "፬", "፭", "፮", "፯", "፰", "፱"}
var ethiopicTens = [10]string{"", "፲", "፳", "፴", "፵", "፶", "፷", "፸", "፹", "፺"}

const (
	ethiopicHundred     = "፻"
	ethiopicTenThousand = "፼"
)

// Format formats n as a locale-appropriate integer string.
func Format(n int64, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if !ok {
		return strconv.FormatInt(n, 10)
	}
	return formatInteger(n, meta)
}

// FormatFloat formats f with decimalPlaces precision.
func FormatFloat(f float64, decimalPlaces int, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if !ok {
		return fmt.Sprintf("%.*f", decimalPlaces, f)
	}

	isNegative := f < 0
	if isNegative {
		f = -f
	}
	factor := math.Pow10(decimalPlaces)
	rounded := math.Round(f*factor) / factor
	intPart := int64(rounded)
	fracPart := int64(math.Round((rounded - float64(intPart)) * factor))

	var result string
	if meta.NumberScript == locale.NumberScriptEthi {
		result = formatEthiopicInteger(intPart)
	} else {
		result = formatNumber(intPart, meta.GroupingSeparator)
	}

	if decimalPlaces > 0 {
		frac := padLeft(strconv.FormatInt(fracPart, 10), decimalPlaces, '0')
		if meta.NumberScript == locale.NumberScriptEthi {
			frac = mapDigits(frac, ethiopicDigits)
		}
		result += meta.DecimalSeparator + frac
	}
	if isNegative {
		result = "-" + result
	}
	if meta.NumberScript == locale.NumberScriptEthi {
		return result
	}
	return FormatDigit(result, meta.Tag)
}

// FormatPercent formats a fraction (0.0–1.0) as a percentage.
func FormatPercent(fraction float64, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if !ok {
		return fmt.Sprintf("%.1f%%", fraction*100)
	}
	formatted := FormatFloat(fraction*100, 1, l)
	if meta.PluralFamily == locale.PluralFrench {
		return formatted + " %"
	}
	return formatted + "%"
}

// FormatOrdinal formats n as an ordinal string.
func FormatOrdinal(n int64, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if !ok {
		return englishOrdinal(n)
	}

	switch meta.PluralFamily {
	case locale.PluralFrench:
		if n == 1 {
			return "1er"
		}
		return fmt.Sprintf("%de", n)
	case locale.PluralInvariant, locale.PluralArabic:
		return Format(n, l)
	default:
		return englishOrdinal(n)
	}
}

// FormatDigit converts LTR numbers to localized digital script displays.
func FormatDigit(numStr string, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if ok && meta.NumberScript == locale.NumberScriptArab {
		return mapDigits(numStr, easternDigits)
	}
	if ok && meta.NumberScript == locale.NumberScriptEthi {
		return mapDigits(numStr, ethiopicDigits)
	}
	return numStr
}

func mapDigits(numStr string, digits [10]string) string {
	var sb strings.Builder
	sb.Grow(len(numStr) * 3)
	for _, r := range numStr {
		if r >= '0' && r <= '9' {
			sb.WriteString(digits[r-'0'])
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func formatInteger(n int64, meta locale.Meta) string {
	isNegative := n < 0
	if isNegative {
		n = -n
	}
	var result string
	if meta.NumberScript == locale.NumberScriptEthi {
		result = formatEthiopicInteger(n)
	} else {
		result = formatNumber(n, meta.GroupingSeparator)
		result = FormatDigit(result, meta.Tag)
	}
	if isNegative {
		return "-" + result
	}
	return result
}

func formatEthiopicInteger(n int64) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	if n < 0 {
		n = -n
	}

	if n >= 10000 {
		thousands := n / 10000
		if thousands > 1 {
			b.WriteString(formatEthiopicInteger(thousands))
		}
		b.WriteString(ethiopicTenThousand)
		n %= 10000
	}

	if n >= 100 {
		hundreds := n / 100
		if hundreds > 1 {
			b.WriteString(formatEthiopicInteger(hundreds))
		}
		b.WriteString(ethiopicHundred)
		n %= 100
	}

	if n >= 10 {
		b.WriteString(ethiopicTens[n/10])
		n %= 10
	}
	if n > 0 {
		b.WriteString(ethiopicDigits[n])
	}
	return b.String()
}

func formatNumber(n int64, groupSep string) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, groupSep)
}

func padLeft(s string, width int, pad rune) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(string(pad), width-len(s)) + s
}

func englishOrdinal(n int64) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
		suffix = "th"
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
