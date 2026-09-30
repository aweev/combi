// Package timezone resolves African timezones and business hours.
package timezone

import (
	"errors"
	"strings"
	"time"
	_ "time/tzdata" // embed tzdata so it works on scratch/alpine containers natively

	"github.com/combi/pkg/geo/country"
)

// ByCountry returns the primary *time.Location for an ISO 3166-1 alpha-2 code.
// Returns time.UTC for unknown codes.
func ByCountry(iso2 string) *time.Location {
	c, ok := country.Get(iso2)
	if !ok || len(c.TimeZones) == 0 {
		return time.UTC
	}
	loc, err := time.LoadLocation(c.TimeZones[0])
	if err != nil {
		return time.UTC
	}
	return loc
}

// AllByCountry returns all timezones used in a country.
func AllByCountry(iso2 string) []*time.Location {
	c, ok := country.Get(iso2)
	if !ok {
		return nil
	}
	var locs []*time.Location
	for _, tz := range c.TimeZones {
		if loc, err := time.LoadLocation(tz); err == nil {
			locs = append(locs, loc)
		}
	}
	return locs
}

// ByCoords returns the timezone for a coordinate pair.
// Note: A full offline geospatial DB is too large for the codebase,
// so this uses a lightweight bounding-box heuristic for major African hubs.
func ByCoords(lat, lng float64) (*time.Location, error) {
	// Kenya (Nairobi hub)
	if lat >= -4.7 && lat <= 4.6 && lng >= 34.0 && lng <= 41.9 {
		return time.LoadLocation("Africa/Nairobi")
	}
	// Nigeria (Lagos hub)
	if lat >= 4.2 && lat <= 13.9 && lng >= 2.6 && lng <= 14.6 {
		return time.LoadLocation("Africa/Lagos")
	}
	// South Africa (Johannesburg hub)
	if lat >= -34.8 && lat <= -22.1 && lng >= 16.4 && lng <= 32.9 {
		return time.LoadLocation("Africa/Johannesburg")
	}
	// Egypt (Cairo hub)
	if lat >= 22.0 && lat <= 31.5 && lng >= 25.0 && lng <= 35.0 {
		return time.LoadLocation("Africa/Cairo")
	}

	return nil, errors.New("coordinates outside known simplified boundary boxes")
}

// BusinessHours returns whether time t is within typical business hours for country iso2.
func BusinessHours(t time.Time, iso2 string) bool {
	iso2 = strings.ToUpper(iso2)
	loc := ByCountry(iso2)
	localTime := t.In(loc)

	hour := localTime.Hour()
	if hour < 8 || hour >= 17 { // Typical 8am - 5pm business day
		return false
	}

	weekday := localTime.Weekday()

	// Islamic workweek (North Africa / Egypt / Morocco)
	// Sunday to Thursday is the work week. Friday and Saturday are weekend.
	if iso2 == "EG" || iso2 == "DZ" || iso2 == "MA" || iso2 == "LY" {
		if weekday == time.Friday || weekday == time.Saturday {
			return false
		}
		return true
	}

	// Standard Sub-Saharan workweek (Monday to Friday)
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	return true
}
