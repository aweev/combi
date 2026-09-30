package fingerprint_test

import (
	"testing"

	"github.com/combi/pkg/identity/fingerprint"
)

func TestGenerateIDIsStable(t *testing.T) {
	signals := fingerprint.DeviceSignals{
		DeviceID:     "device123",
		OS:           "Android",
		Browser:      "Chrome",
		Timezone:     "Africa/Nairobi",
		Capabilities: []string{"camera", "gps", "nfc"},
		IPAddress:    "102.34.56.78",
	}
	first := fingerprint.GenerateID(signals)
	second := fingerprint.GenerateID(signals)
	if first != second {
		t.Fatal("expected stable fingerprint")
	}
}

func TestCalculateRiskScore(t *testing.T) {
	result := fingerprint.CalculateRiskScore(fingerprint.DeviceSignals{OS: "Android", Capabilities: []string{"gps"}, IPAddress: "102345678"})
	if result.Score < 0 || result.Score > 100 {
		t.Fatalf("unexpected risk score: %d", result.Score)
	}
}

func TestCompareDeviceSignals(t *testing.T) {
	a := fingerprint.DeviceSignals{DeviceID: "a", OS: "Android", Browser: "Chrome", Timezone: "UTC", Capabilities: []string{"gps"}, IPAddress: "1.1.1.1"}
	b := fingerprint.DeviceSignals{DeviceID: "a", OS: "Android", Browser: "Chrome", Timezone: "UTC", Capabilities: []string{"gps"}, IPAddress: "1.1.1.1"}
	if !fingerprint.CompareDeviceSignals(a, b) {
		t.Fatal("expected same device signals")
	}
}
