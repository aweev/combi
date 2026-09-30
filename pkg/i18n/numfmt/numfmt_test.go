package numfmt_test

import (
	"testing"

	"github.com/combi/pkg/i18n/locale"
	"github.com/combi/pkg/i18n/numfmt"
)

func TestFormatEthiopicDigits(t *testing.T) {
	if got := numfmt.Format(123, locale.AmET); got != "፻፳፫" {
		t.Fatalf("Format = %q, want %q", got, "፻፳፫")
	}
}

func TestFormatEthiopicFloat(t *testing.T) {
	if got := numfmt.FormatFloat(12.5, 1, locale.AmET); got != "፲፪.፭" {
		t.Fatalf("FormatFloat = %q, want %q", got, "፲፪.፭")
	}
}

func TestFormatIntegerLatn(t *testing.T) {
	if got := numfmt.Format(1234567, locale.EnNG); got != "1,234,567" {
		t.Fatalf("Format = %q, want %q", got, "1,234,567")
	}
}

func TestFormatIntegerArabicNumerals(t *testing.T) {
	if got := numfmt.Format(1234567, locale.HaNG_Arab); got != "١٬٢٣٤٬٥٦٧" {
		t.Fatalf("Format = %q, want %q", got, "١٬٢٣٤٬٥٦٧")
	}
}

func TestFormatFloat(t *testing.T) {
	if got := numfmt.FormatFloat(3.14159, 2, locale.FrCM); got != "3,14" {
		t.Fatalf("FormatFloat = %q, want %q", got, "3,14")
	}
}

func TestFormatPercent(t *testing.T) {
	if got := numfmt.FormatPercent(0.756, locale.EnNG); got != "75.6%" {
		t.Fatalf("FormatPercent = %q, want %q", got, "75.6%")
	}
	if got := numfmt.FormatPercent(0.756, locale.FrCM); got != "75,6 %" {
		t.Fatalf("FormatPercent = %q, want %q", got, "75,6 %")
	}
}

func TestFormatOrdinal(t *testing.T) {
	if got := numfmt.FormatOrdinal(1, locale.EnNG); got != "1st" {
		t.Fatalf("FormatOrdinal = %q, want %q", got, "1st")
	}
	if got := numfmt.FormatOrdinal(2, locale.EnNG); got != "2nd" {
		t.Fatalf("FormatOrdinal = %q, want %q", got, "2nd")
	}
	if got := numfmt.FormatOrdinal(1, locale.FrCM); got != "1er" {
		t.Fatalf("FormatOrdinal = %q, want %q", got, "1er")
	}
	if got := numfmt.FormatOrdinal(2, locale.FrCM); got != "2e" {
		t.Fatalf("FormatOrdinal = %q, want %q", got, "2e")
	}
}
