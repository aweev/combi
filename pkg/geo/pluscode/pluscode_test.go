package pluscode_test

import (
	"math"
	"testing"

	"github.com/combi/pkg/geo/pluscode"
)

func TestPlusCode_Encode_Yaounde(t *testing.T) {
	// Yaoundé, Cameroon: 3.8480, 11.5021
	code := pluscode.Encode(3.8480, 11.5021, 10)

	// Validating prefix format common to the region
	if !pluscode.IsValid(code) {
		t.Fatalf("Expected valid code, got %s", code)
	}
}

func TestPlusCode_Decode_RoundTrip(t *testing.T) {
	lat := 9.0765 // Abuja
	lng := 7.3986

	code := pluscode.Encode(lat, lng, 10)
	dLat, dLng, _, _, _, _, err := pluscode.Decode(code)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Because plus codes represent bounding boxes, the decoded center
	// won't perfectly match the input coordinates, but should be very close.
	if math.Abs(lat-dLat) > 0.001 || math.Abs(lng-dLng) > 0.001 {
		t.Errorf("Decoded coordinates too far from original: got %f, %f", dLat, dLng)
	}
}

func TestPlusCode_Shorten_RelativeToCity(t *testing.T) {
	refLat := 3.8480
	refLng := 11.5021

	// Dynamically generate a code at the reference coordinate
	// so the Google library mathematically guarantees it can shorten it.
	fullCode := pluscode.Encode(refLat, refLng, 10)

	short, err := pluscode.Shorten(fullCode, refLat, refLng)
	if err != nil {
		t.Fatalf("Shorten failed: %v", err)
	}

	// A shortened code should be shorter than the full code
	if len(short) >= len(fullCode) {
		t.Errorf("Expected shortened code to be shorter than %d chars", len(fullCode))
	}
}

func TestPlusCode_Distance(t *testing.T) {
	// Two points roughly 1km apart
	code1 := pluscode.Encode(3.8480, 11.5021, 10)
	code2 := pluscode.Encode(3.8570, 11.5021, 10)

	dist, err := pluscode.Distance(code1, code2)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}

	// Roughly 1000 metres (allow some tolerance for Earth's curvature math)
	if dist < 900 || dist > 1100 {
		t.Errorf("Expected distance around 1000m, got %f", dist)
	}
}
