package direction_test

import (
	"testing"

	"github.com/combi/pkg/i18n/direction"
	"github.com/combi/pkg/i18n/locale"
)

func TestDirection_RTLLocales(t *testing.T) {
	rtlLocales := []locale.Locale{
		locale.HaNG_Arab,
		locale.HaNE_Arab,
		locale.FufCM_Arab,
	}
	for _, l := range rtlLocales {
		if !locale.IsRTL(l) {
			t.Errorf("%s should be RTL", l)
		}
		if direction.Of(l) != direction.RTL {
			t.Errorf("direction.Of(%s) should be RTL", l)
		}
		if direction.HTMLAttr(l) != "rtl" {
			t.Errorf("HTMLAttr(%s) should be rtl", l)
		}
	}
}

func TestDirection_LTRLocales(t *testing.T) {
	ltrLocales := []locale.Locale{
		locale.EnNG, locale.FrCM, locale.PcmNG, locale.YoNG, locale.HaNG,
	}
	for _, l := range ltrLocales {
		if locale.IsRTL(l) {
			t.Errorf("%s should not be RTL", l)
		}
		if direction.HTMLAttr(l) != "ltr" {
			t.Errorf("HTMLAttr(%s) should be ltr", l)
		}
	}
}

func TestDirection_FlipIcon(t *testing.T) {
	if !direction.FlipIcon("arrow-right", locale.HaNG_Arab) {
		t.Error("arrow-right should flip in RTL")
	}
	if !direction.FlipIcon("chevron-left", locale.HaNG_Arab) {
		t.Error("chevron-left should flip in RTL")
	}
	if direction.FlipIcon("star", locale.HaNG_Arab) {
		t.Error("star should not flip in RTL")
	}
	if direction.FlipIcon("close", locale.HaNG_Arab) {
		t.Error("close should not flip in RTL")
	}
	if direction.FlipIcon("arrow-right", locale.EnNG) {
		t.Error("arrow-right should not flip in LTR")
	}
}

func TestDirection_NumberSystem(t *testing.T) {
	if direction.NumberSystem(locale.HaNG_Arab) != "arab" {
		t.Error("Arabic-script Hausa should use arab numeral system")
	}
	if direction.NumberSystem(locale.HaNG) != "latn" {
		t.Error("Latin-script Hausa should use latn numeral system")
	}
	if direction.NumberSystem(locale.EnNG) != "latn" {
		t.Error("English should use latn numeral system")
	}
}

func TestDirection_BiDiIsolate(t *testing.T) {
	result := direction.UnicodeBidiIsolate("مرحبا", direction.RTL)
	runes := []rune(result)
	if runes[0] != '\u2067' {
		t.Error("RTL bidi isolate should start with RLI (U+2067)")
	}
	if runes[len(runes)-1] != '\u2069' {
		t.Error("RTL bidi isolate should end with PDI (U+2069)")
	}
}

func TestLocaleNegotiation(t *testing.T) {
	cases := []struct {
		header string
		want   locale.Locale
	}{
		{"en-NG", locale.EnNG},
		{"fr-CM", locale.FrCM},
		{"pcm-NG", locale.PcmNG},
		{"ha-NG-Arab", locale.HaNG_Arab},
		{"en-US", locale.EnNG},
		{"", locale.EnNG},
		{"fr-CM,en-NG;q=0.9", locale.FrCM},
		{"en-NG;q=0.5,fr-CM;q=0.9", locale.FrCM},
	}
	for _, tc := range cases {
		got := direction.LocaleForHTTP(tc.header, locale.EnNG)
		if got != tc.want {
			t.Errorf("LocaleForHTTP(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}
