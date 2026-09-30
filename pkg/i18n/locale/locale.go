// Package locale defines every locale Africa OS supports, including
// informal/pidgin variants and script variants as first-class citizens.
package locale

import "strings"

// Locale is a BCP 47 locale tag.
type Locale string

// Script represents the writing system used by a locale.
type Script string

const (
	ScriptLatin    Script = "Latn"
	ScriptArabic   Script = "Arab"
	ScriptNKo      Script = "Nkoo"
	ScriptEthiopic Script = "Ethi"
)

// Direction is the base text direction for a locale.
type Direction string

const (
	DirectionLTR Direction = "ltr"
	DirectionRTL Direction = "rtl"
)

// PluralFamily maps to the CLDR plural rule categories.
type PluralFamily string

const (
	PluralOne       PluralFamily = "one"
	PluralArabic    PluralFamily = "arabic"
	PluralFrench    PluralFamily = "french"
	PluralInvariant PluralFamily = "invariant"
)

type DateOrder string

type NumberScript string

const (
	DateOrderDMY DateOrder = "DMY" // 14/06/2026 — Nigeria, Cameroon, Ghana, most of Africa
	DateOrderYMD DateOrder = "YMD" // 2026-06-14 — ISO, used by Kenya in formal/tech contexts
	DateOrderMDY DateOrder = "MDY" // reserved; no current African market uses this
)

const (
	NumberScriptLatn NumberScript = "latn"
	NumberScriptArab NumberScript = "arab"
	NumberScriptEthi NumberScript = "ethi"
)

// Meta carries all the metadata a service or UI needs to render content correctly.
type Meta struct {
	Tag           Locale
	Script        Script
	Direction     Direction
	PluralFamily  PluralFamily
	NativeName    string
	EnglishName   string
	FallbackChain []Locale
	// Canonical marks this locale as the preferred representative for its
	// primary language subtag. Used by direction.LocaleForHTTP to produce
	// deterministic results when prefix-matching "fr" → fr-CM, not fr-SN.
	Canonical bool
	Formal    bool // true = formal register expected
	RTLMirror Locale

	PrimaryCurrency   string
	CurrencySymbol    string
	DecimalSeparator  string
	GroupingSeparator string

	Timezone             string
	DateOrder            DateOrder
	NumberScript         NumberScript
	UseEthiopianCalendar bool
}

// ── Supported locales ────────────────────────────────────────────────────────

const (
	LnsCM Locale = "lns-CM" // Lamnso' (Nso, Cameroon)

	EnNG Locale = "en-NG" // Nigerian English
	EnGH Locale = "en-GH" // Ghanaian English
	EnCM Locale = "en-CM" // Cameroonian English

	FrCM Locale = "fr-CM" // Cameroonian French
	FrSN Locale = "fr-SN" // Senegalese French
	FrCI Locale = "fr-CI" // Ivorian French

	PcmNG      Locale = "pcm-NG"         // Nigerian Pidgin (Naija)
	PcmNGSlang Locale = "pcm-NG-x-slang" // Nigerian Pidgin (Informal)

	PcmCM Locale = "pcm-CM"                // Cameroonian Pidgin English
	CfgCM Locale = "und-CM-x-camfranglais" // Camfranglais

	YoNG Locale = "yo-NG" // Yoruba (Nigeria)
	YoBJ Locale = "yo-BJ" // Yoruba (Benin)

	IgNG Locale = "ig-NG" // Igbo (Nigeria)

	HaNG      Locale = "ha-NG"      // Hausa Nigeria (Latin)
	HaNE      Locale = "ha-NE"      // Hausa Niger (Latin)
	HaNG_Arab Locale = "ha-NG-Arab" // Hausa Nigeria (Arabic/Ajami)
	HaNE_Arab Locale = "ha-NE-Arab" // Hausa Niger (Arabic/Ajami)

	FufCM      Locale = "fuf-CM"      // Fulfulde Cameroon (Latin)
	FufGN      Locale = "fuf-GN"      // Pular Guinea
	FufCM_Arab Locale = "fuf-CM-Arab" // Fulfulde (Arabic/Ajami)

	AkGH Locale = "ak-GH" // Akan/Twi
	EeGH Locale = "ee-GH" // Ewe

	SwKE Locale = "sw-KE" // Swahili Kenya
	SwTZ Locale = "sw-TZ" // Swahili Tanzania

	AmET Locale = "am-ET" // Amharic (Ethiopia)
	TiET Locale = "ti-ET" // Tigrinya (Ethiopia)
	OmET Locale = "om-ET" // Oromo (Ethiopia)
)

var All = map[Locale]Meta{
	EnNG: {
		Tag: EnNG, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "English (Nigeria)",
		EnglishName: "Nigerian English", FallbackChain: []Locale{EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	EnGH: {
		Tag: EnGH, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "English (Ghana)",
		EnglishName: "Ghanaian English", FallbackChain: []Locale{EnGH, EnNG},
		Formal:          true,
		PrimaryCurrency: "GHS", CurrencySymbol: "GH₵",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Accra", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	EnCM: {
		Tag: EnCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "English (Cameroon)",
		EnglishName: "Cameroonian English", FallbackChain: []Locale{EnCM, EnNG},
		Formal:          true,
		PrimaryCurrency: "XAF", CurrencySymbol: "CFA",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	FrCM: {
		Tag: FrCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "Français (Cameroun)",
		EnglishName: "Cameroonian French", FallbackChain: []Locale{FrCM, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "XAF", CurrencySymbol: "FCFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	FrSN: {
		Tag: FrSN, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "Français (Sénégal)",
		EnglishName: "Senegalese French", FallbackChain: []Locale{FrSN, FrCM, EnNG},
		Formal:          true,
		PrimaryCurrency: "XOF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Dakar", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	FrCI: {
		Tag: FrCI, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "Français (Côte d'Ivoire)",
		EnglishName: "Ivorian French", FallbackChain: []Locale{FrCI, FrCM, EnNG},
		Formal:          true,
		PrimaryCurrency: "XOF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Abidjan", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	PcmNG: {
		Tag: PcmNG, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Naija",
		EnglishName: "Nigerian Pidgin", FallbackChain: []Locale{PcmNG, EnNG},
		Canonical: true, Formal: false,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	PcmNGSlang: {
		Tag: PcmNGSlang, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Naija (Street)",
		EnglishName: "Nigerian Pidgin (Informal)", FallbackChain: []Locale{PcmNGSlang, PcmNG, EnNG},
		Formal:          false,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	PcmCM: {
		Tag: PcmCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Pidgin (Cameroon)",
		EnglishName: "Cameroonian Pidgin", FallbackChain: []Locale{PcmCM, EnCM, EnNG},
		Formal:          false,
		PrimaryCurrency: "XAF", CurrencySymbol: "CFA",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	CfgCM: {
		Tag: CfgCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Camfranglais",
		EnglishName: "Camfranglais", FallbackChain: []Locale{CfgCM, PcmCM, FrCM, EnCM, EnNG},
		Formal:          false,
		PrimaryCurrency: "XAF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	YoNG: {
		Tag: YoNG, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Yorùbá",
		EnglishName: "Yoruba (Nigeria)", FallbackChain: []Locale{YoNG, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	YoBJ: {
		Tag: YoBJ, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Yorùbá (Bénin)",
		EnglishName: "Yoruba (Benin)", FallbackChain: []Locale{YoBJ, YoNG, FrSN, EnNG},
		Formal:          true,
		PrimaryCurrency: "XOF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Porto-Novo", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	IgNG: {
		Tag: IgNG, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Igbo",
		EnglishName: "Igbo (Nigeria)", FallbackChain: []Locale{IgNG, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	HaNG: {
		Tag: HaNG, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Hausa",
		EnglishName: "Hausa (Nigeria, Latin)", FallbackChain: []Locale{HaNG, EnNG},
		Canonical: true, Formal: true, RTLMirror: HaNG_Arab,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	HaNG_Arab: {
		Tag: HaNG_Arab, Script: ScriptArabic, Direction: DirectionRTL,
		PluralFamily: PluralArabic, NativeName: "هَوْسَ",
		EnglishName: "Hausa (Nigeria, Arabic/Ajami)", FallbackChain: []Locale{HaNG_Arab, HaNG, EnNG},
		Formal: true, RTLMirror: HaNG,
		PrimaryCurrency: "NGN", CurrencySymbol: "₦",
		DecimalSeparator: ".", GroupingSeparator: "٬",
		Timezone: "Africa/Lagos", DateOrder: DateOrderDMY, NumberScript: NumberScriptArab,
	},
	HaNE: {
		Tag: HaNE, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Hausa (Nijar)",
		EnglishName: "Hausa (Niger, Latin)", FallbackChain: []Locale{HaNE, HaNG, FrSN, EnNG},
		Formal: true, RTLMirror: HaNE_Arab,
		PrimaryCurrency: "XOF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Niamey", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	HaNE_Arab: {
		Tag: HaNE_Arab, Script: ScriptArabic, Direction: DirectionRTL,
		PluralFamily: PluralArabic, NativeName: "هَوْسَ (نِيجَر)",
		EnglishName: "Hausa (Niger, Arabic/Ajami)", FallbackChain: []Locale{HaNE_Arab, HaNE, HaNG, FrSN, EnNG},
		Formal: true, RTLMirror: HaNE,
		PrimaryCurrency: "XOF", CurrencySymbol: "CFA",
		DecimalSeparator: ",", GroupingSeparator: "٬",
		Timezone: "Africa/Niamey", DateOrder: DateOrderDMY, NumberScript: NumberScriptArab,
	},
	FufCM: {
		Tag: FufCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "Fulfulde",
		EnglishName: "Fulfulde (Cameroon, Latin)", FallbackChain: []Locale{FufCM, FrCM, EnCM, EnNG},
		Canonical: true, Formal: true, RTLMirror: FufCM_Arab,
		PrimaryCurrency: "XAF", CurrencySymbol: "FCFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	FufCM_Arab: {
		Tag: FufCM_Arab, Script: ScriptArabic, Direction: DirectionRTL,
		PluralFamily: PluralFrench, NativeName: "𞤊𞤵𞤤𞤬𞤵𞤤𞤣𞤫",
		EnglishName: "Fulfulde (Cameroon, Arabic/Ajami)", FallbackChain: []Locale{FufCM_Arab, FufCM, FrCM, EnNG},
		Formal: true, RTLMirror: FufCM,
		PrimaryCurrency: "XAF", CurrencySymbol: "FCFA",
		DecimalSeparator: ",", GroupingSeparator: "٬",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptArab,
	},
	FufGN: {
		Tag: FufGN, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "Pular",
		EnglishName: "Pular (Guinea)", FallbackChain: []Locale{FufGN, FufCM, FrCM, EnNG},
		Formal:          true,
		PrimaryCurrency: "GNF", CurrencySymbol: "FG",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Conakry", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	AkGH: {
		Tag: AkGH, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Akan",
		EnglishName: "Akan/Twi (Ghana)", FallbackChain: []Locale{AkGH, EnGH, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "GHS", CurrencySymbol: "GH₵",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Accra", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	EeGH: {
		Tag: EeGH, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Eʋegbe",
		EnglishName: "Ewe (Ghana)", FallbackChain: []Locale{EeGH, EnGH, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "GHS", CurrencySymbol: "GH₵",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Accra", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	SwKE: {
		Tag: SwKE, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Kiswahili (Kenya)",
		EnglishName: "Swahili (Kenya)", FallbackChain: []Locale{SwKE, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "KES", CurrencySymbol: "KSh",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Nairobi", DateOrder: DateOrderYMD, NumberScript: NumberScriptLatn,
	},
	SwTZ: {
		Tag: SwTZ, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Kiswahili (Tanzania)",
		EnglishName: "Swahili (Tanzania)", FallbackChain: []Locale{SwTZ, SwKE, EnNG},
		Formal:          true,
		PrimaryCurrency: "TZS", CurrencySymbol: "TSh",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Dar_es_Salaam", DateOrder: DateOrderYMD, NumberScript: NumberScriptLatn,
	},
	AmET: {
		Tag: AmET, Script: ScriptEthiopic, Direction: DirectionLTR,
		PluralFamily: PluralFrench, NativeName: "አማርኛ",
		EnglishName: "Amharic", FallbackChain: []Locale{AmET, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "ETB", CurrencySymbol: "ብር",
		DecimalSeparator: ".", GroupingSeparator: "፣",
		Timezone: "Africa/Addis_Ababa", DateOrder: DateOrderDMY,
		NumberScript: NumberScriptEthi, UseEthiopianCalendar: true,
	},
	TiET: {
		Tag: TiET, Script: ScriptEthiopic, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "ትግርኛ",
		EnglishName: "Tigrinya", FallbackChain: []Locale{TiET, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "ETB", CurrencySymbol: "ብር",
		DecimalSeparator: ".", GroupingSeparator: "፣",
		Timezone: "Africa/Addis_Ababa", DateOrder: DateOrderDMY,
		NumberScript: NumberScriptEthi, UseEthiopianCalendar: true,
	},
	OmET: {
		Tag: OmET, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralOne, NativeName: "Oromoo",
		EnglishName: "Oromo (Ethiopia)", FallbackChain: []Locale{OmET, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "ETB", CurrencySymbol: "ብር",
		DecimalSeparator: ".", GroupingSeparator: ",",
		Timezone: "Africa/Addis_Ababa", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
	LnsCM: {
		Tag: LnsCM, Script: ScriptLatin, Direction: DirectionLTR,
		PluralFamily: PluralInvariant, NativeName: "Lamnsɔ'",
		EnglishName: "Lamnso'", FallbackChain: []Locale{LnsCM, EnCM, FrCM, EnNG},
		Canonical: true, Formal: true,
		PrimaryCurrency: "XAF", CurrencySymbol: "FCFA",
		DecimalSeparator: ",", GroupingSeparator: "\u202F",
		Timezone: "Africa/Douala", DateOrder: DateOrderDMY, NumberScript: NumberScriptLatn,
	},
}

// Lookup returns the Meta for a locale.
func Lookup(l Locale) (Meta, bool) {
	m, ok := All[l]
	return m, ok
}

// MustLookup returns the Meta or panics for unknown locales.
func MustLookup(l Locale) Meta {
	m, ok := All[l]
	if !ok {
		panic("i18n: unregistered locale: " + string(l))
	}
	return m
}

// IsRTL reports whether l uses right-to-left script.
func IsRTL(l Locale) bool {
	if m, ok := All[l]; ok {
		return m.Direction == DirectionRTL
	}
	return false
}

// IsFormal reports whether formal register is expected for l.
func IsFormal(l Locale) bool {
	if m, ok := All[l]; ok {
		return m.Formal
	}
	return true
}

// ScriptOf returns the writing system for l.
func ScriptOf(l Locale) Script {
	if m, ok := All[l]; ok {
		return m.Script
	}
	return ScriptLatin
}

// FallbackChain returns the ordered list of locales to try.
func FallbackChain(l Locale) []Locale {
	if m, ok := All[l]; ok {
		return m.FallbackChain
	}
	return []Locale{EnNG}
}

// SoftBreakEthiopicText inserts zero-width spaces between Ethiopic syllables.
func SoftBreakEthiopicText(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(runes) * 2)

	for i, r := range runes {
		b.WriteRune(r)
		if i+1 < len(runes) && isEthiopicRune(r) && isEthiopicRune(runes[i+1]) {
			b.WriteRune('\u200B')
		}
	}
	return b.String()
}

func isEthiopicRune(r rune) bool {
	return (r >= 0x1200 && r <= 0x137F) || (r >= 0x1380 && r <= 0x139F)
}
