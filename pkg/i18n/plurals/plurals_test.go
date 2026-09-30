package plurals_test

import (
	"testing"

	"github.com/combi/pkg/i18n/locale"
	"github.com/combi/pkg/i18n/plurals"
)

func TestPluralRules_English(t *testing.T) {
	cases := []struct {
		n    int64
		want plurals.Form
	}{
		{0, plurals.FormOther},
		{1, plurals.FormOne},
		{2, plurals.FormOther},
		{100, plurals.FormOther},
	}
	for _, tc := range cases {
		got := plurals.Select(tc.n, locale.EnNG)
		if got != tc.want {
			t.Errorf("English plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestPluralRules_French(t *testing.T) {
	cases := []struct {
		n    int64
		want plurals.Form
	}{
		{0, plurals.FormOne},
		{1, plurals.FormOne},
		{2, plurals.FormOther},
		{100, plurals.FormOther},
	}
	for _, tc := range cases {
		got := plurals.Select(tc.n, locale.FrCM)
		if got != tc.want {
			t.Errorf("French plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestPluralRules_Arabic(t *testing.T) {
	cases := []struct {
		n    int64
		want plurals.Form
	}{
		{0, plurals.FormZero},
		{1, plurals.FormOne},
		{2, plurals.FormTwo},
		{3, plurals.FormFew},
		{10, plurals.FormFew},
		{11, plurals.FormMany},
		{99, plurals.FormMany},
		{100, plurals.FormOther},
		{101, plurals.FormOne},
		{102, plurals.FormTwo},
		{103, plurals.FormFew},
	}
	for _, tc := range cases {
		got := plurals.Select(tc.n, locale.HaNG_Arab)
		if got != tc.want {
			t.Errorf("Arabic plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestPluralRules_Invariant(t *testing.T) {
	invariantLocales := []locale.Locale{
		locale.YoNG,
		locale.IgNG,
		locale.PcmNG,
		locale.PcmNGSlang,
		locale.AkGH,
	}
	for _, l := range invariantLocales {
		for _, n := range []int64{0, 1, 2, 100} {
			got := plurals.Select(n, l)
			if got != plurals.FormOther {
				t.Errorf("%s plural(%d) = %q, want FormOther (invariant)", l, n, got)
			}
		}
	}
}

func TestSelectArabicNegativeValues(t *testing.T) {
	cases := []struct {
		n    int64
		want plurals.Form
	}{
		{-0, plurals.FormZero},
		{-1, plurals.FormOne},
		{-2, plurals.FormTwo},
		{-3, plurals.FormFew},
		{-11, plurals.FormMany},
		{-100, plurals.FormOther},
	}
	for _, tc := range cases {
		got := plurals.Select(tc.n, locale.HaNG_Arab)
		if got != tc.want {
			t.Errorf("Arabic plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}