// Package direction provides text direction utilities for RTL/LTR rendering.
package direction

import (
	"strings"

	"github.com/combi/pkg/i18n/locale"
)

// Dir is the text direction constant.
type Dir = locale.Direction

const (
	LTR = locale.DirectionLTR
	RTL = locale.DirectionRTL
)

// Of returns the text direction for a locale.
func Of(l locale.Locale) Dir {
	if locale.IsRTL(l) {
		return RTL
	}
	return LTR
}

// HTMLAttr returns the HTML dir attribute string for a locale.
func HTMLAttr(l locale.Locale) string {
	return string(Of(l))
}

// CSSLogical maps a physical CSS property to its logical equivalent.
func CSSLogical(physicalProp string, l locale.Locale) string {
	isRTL := Of(l) == RTL

	mappings := map[string]struct {
		ltr string
		rtl string
	}{
		"margin-left":      {"margin-inline-start", "margin-inline-end"},
		"margin-right":     {"margin-inline-end", "margin-inline-start"},
		"padding-left":     {"padding-inline-start", "padding-inline-end"},
		"padding-right":    {"padding-inline-end", "padding-inline-start"},
		"border-left":      {"border-inline-start", "border-inline-end"},
		"border-right":     {"border-inline-end", "border-inline-start"},
		"left":             {"inset-inline-start", "inset-inline-end"},
		"right":            {"inset-inline-end", "inset-inline-start"},
		"text-align:left":  {"text-align:start", "text-align:start"},
		"text-align:right": {"text-align:end", "text-align:end"},
	}

	key := strings.ToLower(strings.TrimSpace(physicalProp))
	if m, ok := mappings[key]; ok {
		if isRTL {
			return m.rtl
		}
		return m.ltr
	}
	return physicalProp
}

// UnicodeBidiIsolate wraps a string in Unicode bidi isolation characters.
func UnicodeBidiIsolate(s string, dir Dir) string {
	if dir == RTL {
		// RLI + string + PDI
		return "\u2067" + s + "\u2069"
	}
	// LRI + string + PDI
	return "\u2066" + s + "\u2069"
}

// FlipIcon returns an indication of whether an icon should be mirrored.
func FlipIcon(icon string, l locale.Locale) bool {
	if Of(l) != RTL {
		return false
	}
	flippable := map[string]bool{
		"arrow-left":      true,
		"arrow-right":     true,
		"chevron-left":    true,
		"chevron-right":   true,
		"back":            true,
		"forward":         true,
		"send":            true,
		"reply":           true,
		"quote":           true,
		"indent":          true,
		"outdent":         true,
		"play":            true,
		"skip-forward":    true,
		"skip-backward":   true,
		"list-bullet":     true,
		"sort-ascending":  true,
		"sort-descending": true,
	}
	return flippable[icon]
}

// NumberSystem returns the preferred numeral system for a locale.
func NumberSystem(l locale.Locale) string {
	switch locale.ScriptOf(l) {
	case locale.ScriptArabic:
		return "arab"
	default:
		return "latn"
	}
}

// LocaleForHTTP derives a locale from an HTTP Accept-Language header value.
func LocaleForHTTP(acceptLanguage string, defaultLocale locale.Locale) locale.Locale {
	if acceptLanguage == "" {
		return defaultLocale
	}

	tags := parseAcceptLanguage(acceptLanguage)

	// Exact match
	for _, tag := range tags {
		candidate := locale.Locale(tag)
		if _, ok := locale.Lookup(candidate); ok {
			return candidate
		}
	}

	// Prefix match
	for _, tag := range tags {
		for registered := range locale.All {
			if strings.HasPrefix(string(registered), tag) {
				return registered
			}
		}
	}

	return defaultLocale
}

// parseAcceptLanguage parses an Accept-Language header.
func parseAcceptLanguage(header string) []string {
	type langQ struct {
		tag string
		q   float32
	}

	parts := strings.Split(header, ",")
	langs := make([]langQ, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.SplitN(part, ";", 2)
		tag := strings.TrimSpace(segments[0])
		q := float32(1.0)
		if len(segments) == 2 {
			var qv float32
			if _, err := parseQ(segments[1], &qv); err == nil {
				q = qv
			}
		}
		langs = append(langs, langQ{tag: tag, q: q})
	}

	for i := 1; i < len(langs); i++ {
		for j := i; j > 0 && langs[j].q > langs[j-1].q; j-- {
			langs[j], langs[j-1] = langs[j-1], langs[j]
		}
	}

	result := make([]string, len(langs))
	for i, l := range langs {
		result[i] = l.tag
	}
	return result
}

func parseQ(s string, q *float32) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "q=")
	s = strings.TrimPrefix(s, "Q=")
	var v float32
	_, err := parseFloat(s, &v)
	if err != nil {
		return 0, err
	}
	*q = v
	return 1, nil
}

func parseFloat(s string, f *float32) (int, error) {
	var result float32
	var decimal float32 = 1
	afterDot := false
	for _, c := range s {
		if c == '.' {
			afterDot = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		digit := float32(c - '0')
		if afterDot {
			decimal /= 10
			result += digit * decimal
		} else {
			result = result*10 + digit
		}
	}
	*f = result
	return 1, nil
}

type parseError struct{ s string }

func (e *parseError) Error() string { return "direction: cannot parse float: " + e.s }
