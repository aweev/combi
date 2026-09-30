// Package phonefmt formats and validates phone numbers for Africa OS markets.
package phonefmt

import (
	"fmt"
	"strings"

	"github.com/combi/pkg/i18n/locale"
)

// dialCodes maps a locale to its country calling code.
var dialCodes = map[locale.Locale]string{
	locale.EnNG:       "234",
	locale.PcmNG:      "234",
	locale.PcmNGSlang: "234",
	locale.YoNG:       "234",
	locale.IgNG:       "234",
	locale.HaNG:       "234",
	locale.HaNG_Arab:  "234",
	locale.EnCM:       "237",
	locale.FrCM:       "237",
	locale.PcmCM:      "237",
	locale.LnsCM:      "237",
	locale.FufCM:      "237",
	locale.FufCM_Arab: "237",
	locale.CfgCM:      "237",
	locale.EnGH:       "233",
	locale.AkGH:       "233",
	locale.EeGH:       "233",
	locale.FrSN:       "221",
	locale.SwKE:       "254",
	locale.SwTZ:       "255",
}

// DialCode returns the country calling code for a locale (without "+").
func DialCode(l locale.Locale) string {
	if cc, ok := dialCodes[l]; ok {
		return cc
	}
	for _, fb := range locale.FallbackChain(l) {
		if cc, ok := dialCodes[fb]; ok {
			return cc
		}
	}
	return ""
}

// clean strips all non-digit characters and normalises a "00XX" prefix to "XX".
func clean(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	s = strings.TrimPrefix(s, "00")
	return s
}

type countryFmt struct {
	cc       string
	intlLen  int
	localLen int
	format   func(nat string) string
	e164     func(cc, nat string) string
}

var countries = []countryFmt{
	{
		// Nigeria: +234 XXX XXX XXXX
		cc: "234", intlLen: 13, localLen: 11,
		format: func(n string) string { return fmt.Sprintf("+234 %s %s %s", n[0:3], n[3:6], n[6:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
	{
		// Cameroon: +237 6XX XXX XXX
		cc: "237", intlLen: 12, localLen: 0,
		format: func(n string) string { return fmt.Sprintf("+237 %s %s %s", n[0:3], n[3:6], n[6:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
	{
		// Ghana: +233 XX XXX XXXX
		cc: "233", intlLen: 12, localLen: 10,
		format: func(n string) string { return fmt.Sprintf("+233 %s %s %s", n[0:2], n[2:5], n[5:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
	{
		// Senegal: +221 XX XXX XX XX
		cc: "221", intlLen: 12, localLen: 0,
		format: func(n string) string { return fmt.Sprintf("+221 %s %s %s %s", n[0:2], n[2:5], n[5:7], n[7:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
	{
		// Kenya: +254 XXX XXX XXX
		cc: "254", intlLen: 12, localLen: 10,
		format: func(n string) string { return fmt.Sprintf("+254 %s %s %s", n[0:3], n[3:6], n[6:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
	{
		// Tanzania: +255 XXX XXX XXX
		cc: "255", intlLen: 12, localLen: 10,
		format: func(n string) string { return fmt.Sprintf("+255 %s %s %s", n[0:3], n[3:6], n[6:]) },
		e164:   func(cc, n string) string { return "+" + cc + n },
	},
}

// FormatInternational formats rawPhone into a human-readable international form.
func FormatInternational(rawPhone string) string {
	d := clean(rawPhone)
	for _, c := range countries {
		if strings.HasPrefix(d, c.cc) && len(d) == c.intlLen {
			return c.format(d[len(c.cc):])
		}
		if c.localLen > 0 && strings.HasPrefix(d, "0") && len(d) == c.localLen {
			return c.format(d[1:])
		}
	}
	if len(d) > 10 {
		return "+" + d[:3] + " " + d[3:]
	}
	return "+" + d
}

// FormatE164 returns the ITU E.164 canonical form of rawPhone.
func FormatE164(rawPhone string) string {
	d := clean(rawPhone)
	for _, c := range countries {
		if strings.HasPrefix(d, c.cc) && len(d) == c.intlLen {
			return c.e164(c.cc, d[len(c.cc):])
		}
		if c.localLen > 0 && strings.HasPrefix(d, "0") && len(d) == c.localLen {
			return c.e164(c.cc, d[1:])
		}
	}
	return ""
}

// FormatNational returns the domestic dialling form (no country code).
func FormatNational(rawPhone string) string {
	d := clean(rawPhone)
	for _, c := range countries {
		if strings.HasPrefix(d, c.cc) && len(d) == c.intlLen {
			nat := d[len(c.cc):]
			if c.localLen > 0 {
				return "0" + nat
			}
			return nat
		}
	}
	return d
}
