package address_test

import (
	"testing"

	"github.com/combi/pkg/identity/address"
)

func TestNewDigitalAddress(t *testing.T) {
	d := address.New(1.2921, 36.8219, "Near the post office", "Nairobi CBD", "Nairobi")
	if !d.IsValidPlusCode() {
		t.Fatalf("expected valid plus code, got %s", d.PlusCode)
	}
}

func TestNormalizeLandmark(t *testing.T) {
	got := address.NormalizeLandmark("Near the post office, opposite the bank")
	if got != "the post office the bank" {
		t.Fatalf("normalization failed: %s", got)
	}
}

func TestDistanceAndVerifyResidence(t *testing.T) {
	d := address.New(1.2921, 36.8219, "sample", "area", "region")
	if !d.VerifyResidence(1.2922, 36.8220, "sample") {
		t.Fatal("expected residence verify true")
	}
	if address.Distance(1.2921, 36.8219, 2.0, 37.0) <= 0.25 {
		t.Fatal("expected distance greater than 0.25km")
	}
}
