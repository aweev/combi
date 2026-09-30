package address

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strings"

	olc "github.com/google/open-location-code/go"
)

type DigitalAddress struct {
	PlusCode  string
	Latitude  float64
	Longitude float64
	Landmark  string
	Area      string
	Region    string
}

var plusCodePattern = regexp.MustCompile(`^[23456789CFGHJMPQRVWX]+$`)

func New(lat, lon float64, landmark, area, region string) DigitalAddress {
	plus := olc.Encode(lat, lon, 11)
	return DigitalAddress{
		PlusCode:  plus,
		Latitude:  lat,
		Longitude: lon,
		Landmark:  NormalizeLandmark(landmark),
		Area:      strings.TrimSpace(area),
		Region:    strings.TrimSpace(region),
	}
}

func NormalizeLandmark(input string) string {
	normalized := strings.TrimSpace(strings.ToLower(input))
	normalized = strings.ReplaceAll(normalized, "near", "")
	normalized = strings.ReplaceAll(normalized, "opposite", "")
	normalized = strings.ReplaceAll(normalized, "beside", "")
	normalized = strings.ReplaceAll(normalized, ",", "")
	return strings.Join(strings.Fields(normalized), " ")
}

func (d DigitalAddress) Hash() string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s", d.PlusCode, d.Landmark, d.Area, d.Region, fmt.Sprintf("%.6f,%.6f", d.Latitude, d.Longitude))
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func (d DigitalAddress) VerifyResidence(lat, lon float64, landmark string) bool {
	if NormalizeLandmark(landmark) != d.Landmark {
		return false
	}
	return Distance(d.Latitude, d.Longitude, lat, lon) < 0.25
}

func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	phi1 := rad(lat1)
	phi2 := rad(lat2)
	deltaPhi := rad(lat2 - lat1)
	deltaLambda := rad(lon2 - lon1)

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}

func (d DigitalAddress) IsValidPlusCode() bool {
	return plusCodePattern.MatchString(strings.ReplaceAll(strings.ToUpper(d.PlusCode), "+", ""))
}
