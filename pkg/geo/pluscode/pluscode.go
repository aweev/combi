// Package pluscode wraps the Open Location Code (Plus Codes) algorithm.
// Plus Codes work without internet — they are a deterministic algorithm
// for encoding lat/lng coordinates as short human-speakable codes.
package pluscode

import (
	"errors"
	"math"

	olc "github.com/google/open-location-code/go"
)

// Encode converts WGS84 coordinates to a Plus Code.
// codeLen: 8 for ~14m precision (e.g. "6FX2+7W"), 10 for ~3m precision.
func Encode(lat, lng float64, codeLen int) string {
	return olc.Encode(lat, lng, codeLen)
}

// Decode returns the center lat/lng and bounding box of a Plus Code.
func Decode(code string) (lat, lng, latLo, latHi, lngLo, lngHi float64, err error) {
	if !IsValid(code) {
		return 0, 0, 0, 0, 0, 0, errors.New("invalid plus code")
	}
	area, err := olc.Decode(code)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	latC, lngC := area.Center()
	return latC, lngC,
		area.LatLo, area.LatHi,
		area.LngLo, area.LngHi, nil
}

// Shorten returns a shortened Plus Code relative to a reference location.
// Example: Shorten("6FX22222+22", 3.848, 11.502) → "2222+22"
func Shorten(code string, refLat, refLng float64) (string, error) {
	return olc.Shorten(code, refLat, refLng)
}

// IsValid checks if the string is a syntactically valid Plus Code.
// Note: IsValid returns true for both full and short codes.
func IsValid(code string) bool {
	return olc.Check(code) == nil
}

// Distance returns the approximate distance in metres between the centers of two Plus Codes.
func Distance(code1, code2 string) (float64, error) {
	lat1, lng1, _, _, _, _, err := Decode(code1)
	if err != nil {
		return 0, err
	}
	lat2, lng2, _, _, _, _, err := Decode(code2)
	if err != nil {
		return 0, err
	}
	return haversineDistance(lat1, lng1, lat2, lng2), nil
}

// haversineDistance calculates the great-circle distance between two points in metres.
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in metres
	rad := math.Pi / 180

	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
