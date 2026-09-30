// Package plurals implements CLDR plural rule evaluation.
package plurals

import (
	"github.com/combi/pkg/i18n/locale"
)

// Form is a CLDR plural category string.
type Form string

const (
	FormZero  Form = "zero"
	FormOne   Form = "one"
	FormTwo   Form = "two"
	FormFew   Form = "few"
	FormMany  Form = "many"
	FormOther Form = "other"
)

// Select returns the CLDR plural form for a given count and locale.
func Select(n int64, l locale.Locale) Form {
	meta, ok := locale.Lookup(l)
	if !ok {
		return selectOne(n)
	}
	switch meta.PluralFamily {
	case locale.PluralInvariant:
		return FormOther
	case locale.PluralFrench:
		return selectFrench(n)
	case locale.PluralArabic:
		return selectArabic(n)
	default:
		return selectOne(n)
	}
}

func selectOne(n int64) Form {
	if n == 1 {
		return FormOne
	}
	return FormOther
}

func selectFrench(n int64) Form {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	if abs == 0 || abs == 1 {
		return FormOne
	}
	return FormOther
}

func selectArabic(n int64) Form {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	if abs == 0 {
		return FormZero
	}
	mod100 := abs % 100
	switch {
	case mod100 == 1:
		return FormOne
	case mod100 == 2:
		return FormTwo
	case mod100 >= 3 && mod100 <= 10:
		return FormFew
	case mod100 >= 11 && mod100 <= 99:
		return FormMany
	default:
		return FormOther
	}
}

func PluralForms(l locale.Locale) []Form {
	meta, ok := locale.Lookup(l)
	if !ok {
		return []Form{FormOne, FormOther}
	}
	switch meta.PluralFamily {
	case locale.PluralInvariant:
		return []Form{FormOther}
	case locale.PluralFrench:
		return []Form{FormOne, FormOther}
	case locale.PluralArabic:
		return []Form{FormZero, FormOne, FormTwo, FormFew, FormMany, FormOther}
	default:
		return []Form{FormOne, FormOther}
	}
}
