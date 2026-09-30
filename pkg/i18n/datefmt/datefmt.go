// Package datefmt formats dates and relative times for Africa OS locales.
package datefmt

import (
	"fmt"
	"strings"
	"time"

	"github.com/combi/pkg/i18n/locale"
)

// months holds native month names keyed by locale tag.
// Array size is [14]string to accommodate 13-month Ethiopian calendar.
var months = map[locale.Locale][14]string{
	locale.EnNG: {
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	},
	locale.FrCM: {
		"", "janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre",
	},
	locale.PcmNG: {
		"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	},
	locale.YoNG: {
		"", "Ṣẹ́rẹ́", "Èrèlé", "Ẹrẹ̀nà", "Ìgbé", "Ẹ̀bibi", "Òkúdu",
		"Agẹmọ", "Ògún", "Owewe", "Ọ̀wàrà", "Bélú", "Ọ̀pẹ̀",
	},
	locale.HaNG: {
		"", "Janairu", "Faburairu", "Maris", "Afirilu", "Mayu", "Yuni",
		"Yuli", "Agusta", "Satumba", "Oktoba", "Nuwamba", "Disamba",
	},
	locale.SwKE: {
		"", "Januari", "Februari", "Machi", "Aprili", "Mei", "Juni",
		"Julai", "Agosti", "Septemba", "Oktoba", "Novemba", "Desemba",
	},
	locale.LnsCM: {
		"", "Nyuŋ kibuŋ", "Nyuŋ bvem", "Nyuŋ keri", "Nyuŋ mvu",
		"Nyuŋ lii", "Nyuŋ tyi", "Nyuŋ sòn", "Nyuŋ ji", "Nyuŋ kinyo",
		"Nyuŋ lùm", "Nyuŋ kwi", "Nyuŋ sùŋ",
	},
	locale.AmET: {
		"", "መስከረም", "ጥቅምት", "ኅዳር", "ታሃሳስ", "ጥር", "የካቲት",
		"መጋቢት", "ሚያዝያ", "ግንቦት", "ሰኞ", "ሐምሌ", "ነሐሴ", "ጳጉሜ",
	},
	locale.TiET: {
		"", "መስከረም", "ጥቅምት", "ኅዳር", "ታሃሳስ", "ጥር", "የካቲት",
		"መጋቢት", "ሚያዝያ", "ግንቦት", "ሰኞ", "ሐምሌ", "ነሐሴ", "ጳጉሜ",
	},
}

func monthNamesFor(l locale.Locale) [14]string {
	if m, ok := months[l]; ok {
		return m
	}
	meta, ok := locale.Lookup(l)
	if !ok {
		return months[locale.EnNG]
	}
	for _, fb := range meta.FallbackChain {
		if m, ok := months[fb]; ok {
			return m
		}
	}
	return months[locale.EnNG]
}

// FormatDate formats t as a full date string for the given locale.
func FormatDate(t time.Time, l locale.Locale) string {
	meta, ok := locale.Lookup(l)
	if ok && meta.UseEthiopianCalendar {
		eth := ToEthiopic(t)
		m := monthNamesFor(l)
		return fmt.Sprintf("%d %s %d", eth.Day, m[eth.Month], eth.Year)
	}

	m := monthNamesFor(l)
	return fmt.Sprintf("%d %s %d", t.Day(), m[t.Month()], t.Year())
}

// ── Relative Strings ──────────────────────────────────────────────────────────

type relativeStrings struct {
	justNow    string
	minutesAgo func(n int64) string
	hoursAgo   func(n int64) string
	yesterday  string
	daysAgo    func(n int64) string
	inMinutes  func(n int64) string
	inHours    func(n int64) string
	tomorrow   string
	inDays     func(n int64) string
}

var relStrings = map[string]relativeStrings{
	"fr": {
		justNow:    "à l'instant",
		minutesAgo: func(n int64) string { return fmt.Sprintf("il y a %d min", n) },
		hoursAgo:   func(n int64) string { return fmt.Sprintf("il y a %d h", n) },
		yesterday:  "hier",
		daysAgo:    func(n int64) string { return fmt.Sprintf("il y a %d jours", n) },
		inMinutes:  func(n int64) string { return fmt.Sprintf("dans %d min", n) },
		inHours:    func(n int64) string { return fmt.Sprintf("dans %d h", n) },
		tomorrow:   "demain",
		inDays:     func(n int64) string { return fmt.Sprintf("dans %d jours", n) },
	},
	"pcm": {
		justNow:    "just now",
		minutesAgo: func(n int64) string { return fmt.Sprintf("%d mins ago", n) },
		hoursAgo:   func(n int64) string { return fmt.Sprintf("%d hours ago", n) },
		yesterday:  "yesterday",
		daysAgo:    func(n int64) string { return fmt.Sprintf("%d days ago", n) },
		inMinutes:  func(n int64) string { return fmt.Sprintf("in %d mins", n) },
		inHours:    func(n int64) string { return fmt.Sprintf("in %d hours", n) },
		tomorrow:   "tomorrow",
		inDays:     func(n int64) string { return fmt.Sprintf("in %d days", n) },
	},
	"sw": {
		justNow:    "sasa hivi",
		minutesAgo: func(n int64) string { return fmt.Sprintf("dakika %d zilizopita", n) },
		hoursAgo:   func(n int64) string { return fmt.Sprintf("masaa %d yaliyopita", n) },
		yesterday:  "jana",
		daysAgo:    func(n int64) string { return fmt.Sprintf("siku %d zilizopita", n) },
		inMinutes:  func(n int64) string { return fmt.Sprintf("baada ya dakika %d", n) },
		inHours:    func(n int64) string { return fmt.Sprintf("baada ya masaa %d", n) },
		tomorrow:   "kesho",
		inDays:     func(n int64) string { return fmt.Sprintf("baada ya siku %d", n) },
	},
	"en": {
		justNow:    "just now",
		minutesAgo: func(n int64) string { return fmt.Sprintf("%d minutes ago", n) },
		hoursAgo:   func(n int64) string { return fmt.Sprintf("%d hours ago", n) },
		yesterday:  "yesterday",
		daysAgo:    func(n int64) string { return fmt.Sprintf("%d days ago", n) },
		inMinutes:  func(n int64) string { return fmt.Sprintf("in %d minutes", n) },
		inHours:    func(n int64) string { return fmt.Sprintf("in %d hours", n) },
		tomorrow:   "tomorrow",
		inDays:     func(n int64) string { return fmt.Sprintf("in %d days", n) },
	},
}

func languageFamily(l locale.Locale) string {
	tag := string(l)
	lang := tag
	if i := strings.IndexByte(tag, '-'); i != -1 {
		lang = tag[:i]
	}
	if _, ok := relStrings[lang]; ok {
		return lang
	}
	return "en"
}

// FormatRelative returns a human-readable relative time string.
func FormatRelative(t, anchor time.Time, l locale.Locale) string {
	rs := relStrings[languageFamily(l)]
	diff := anchor.Sub(t)

	if diff < 0 {
		abs := -diff
		secs := int64(abs.Seconds())
		switch {
		case secs < 60:
			return rs.justNow
		case secs < 3600:
			return rs.inMinutes(secs / 60)
		case secs < 86400:
			return rs.inHours(secs / 3600)
		case secs < 172800:
			return rs.tomorrow
		default:
			return rs.inDays(secs / 86400)
		}
	}

	secs := int64(diff.Seconds())
	switch {
	case secs < 60:
		return rs.justNow
	case secs < 3600:
		return rs.minutesAgo(secs / 60)
	case secs < 86400:
		return rs.hoursAgo(secs / 3600)
	case secs < 172800:
		return rs.yesterday
	default:
		return rs.daysAgo(secs / 86400)
	}
}

// ── Ethiopian Calendar ────────────────────────────────────────────────────────

// EthiopicDate is a date in the Ethiopian calendar.
type EthiopicDate struct {
	Year, Month, Day int
}

var ethiopianMonths = [14]string{
	"",
	"Meskarem",
	"Tikimt",
	"Hidar",
	"Tahsas",
	"Tir",
	"Yekatit",
	"Megabit",
	"Miyazya",
	"Ginbot",
	"Sene",
	"Hamle",
	"Nehase",
	"Pagumē",
}

// ToEthiopic converts a Gregorian time.Time to the Ethiopian calendar.
func ToEthiopic(t time.Time) EthiopicDate {
	localDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	currentYearStart := ethiopianNewYearStart(localDate.Year(), t.Location())
	if localDate.Before(currentYearStart) {
		previousYearStart := ethiopianNewYearStart(localDate.Year()-1, t.Location())
		return ethiopicDateFromStart(localDate, previousYearStart, localDate.Year()-8)
	}
	return ethiopicDateFromStart(localDate, currentYearStart, localDate.Year()-7)
}

func ethiopicDateFromStart(date, start time.Time, year int) EthiopicDate {
	days := int(date.Sub(start).Hours() / 24)
	month := days/30 + 1
	day := days%30 + 1
	if month > 13 {
		month = 13
		day = days - 360 + 1
	}
	return EthiopicDate{Year: year, Month: month, Day: day}
}

func ethiopianNewYearStart(gregorianYear int, loc *time.Location) time.Time {
	startDay := 11
	if isGregorianLeapYear(gregorianYear + 1) {
		startDay = 12
	}
	return time.Date(gregorianYear, time.September, startDay, 0, 0, 0, 0, loc)
}

func isGregorianLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func FormatEthiopianTime(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()
	etHour := (hour + 6) % 12
	if etHour == 0 {
		etHour = 12
	}

	period := "ማታ"
	switch {
	case hour >= 6 && hour < 12:
		period = "ጠዋት"
	case hour >= 12 && hour < 18:
		period = "ከሰዓት"
	}

	return formatClockTime(etHour, minute, period)
}

func formatClockTime(hour, minute int, period string) string {
	return fmt.Sprintf("%02d:%02d %s", hour, minute, period)
}

// FormatEthiopianDateTime combines Ethiopian date and time in a single string.
func FormatEthiopianDateTime(t time.Time, l locale.Locale) string {
	return FormatDate(t, l) + " " + FormatEthiopianTime(t)
}
