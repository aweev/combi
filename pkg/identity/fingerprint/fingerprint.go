package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strconv"
	"strings"
)

type DeviceSignals struct {
	DeviceID     string
	OS           string
	Browser      string
	Timezone     string
	Capabilities []string
	IPAddress    string
}

type Integrity struct {
	SignalCount int
	Score       int
}

func GenerateID(signals DeviceSignals) string {
	tokens := []string{
		strings.TrimSpace(signals.DeviceID),
		strings.ToLower(strings.TrimSpace(signals.OS)),
		strings.ToLower(strings.TrimSpace(signals.Browser)),
		strings.TrimSpace(signals.Timezone),
		strings.TrimSpace(signals.IPAddress),
	}
	sortedCaps := append([]string(nil), signals.Capabilities...)
	sort.Strings(sortedCaps)
	tokens = append(tokens, sortedCaps...)

	h := sha256.New()
	for _, token := range tokens {
		h.Write([]byte(strings.TrimSpace(token)))
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func CalculateRiskScore(signals DeviceSignals) Integrity {
	score := 20
	if signals.DeviceID == "" {
		score += 20
	}
	if !strings.Contains(strings.ToLower(signals.IPAddress), ".") {
		score += 20
	}
	if len(signals.Capabilities) < 3 {
		score += 10
	}
	if strings.Contains(strings.ToLower(signals.OS), "android") || strings.Contains(strings.ToLower(signals.OS), "ios") {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return Integrity{SignalCount: len(signals.Capabilities), Score: score}
}

func AnalyzeVelocity(lat1, lon1, lat2, lon2 float64) float64 {
	delta := distance(lat1, lon1, lat2, lon2)
	return delta
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	rand := func(deg float64) float64 { return deg * math.Pi / 180 }
	phi1 := rand(lat1)
	phi2 := rand(lat2)
	deltaPhi := rand(lat2 - lat1)
	deltaLambda := rand(lon2 - lon1)
	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

func relativePrecision(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func CompareDeviceSignals(a, b DeviceSignals) bool {
	return GenerateID(a) == GenerateID(b)
}
