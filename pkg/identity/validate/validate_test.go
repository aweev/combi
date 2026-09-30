package validate_test

import (
	"testing"

	"github.com/combi/pkg/identity/validate"
)

func TestCheckSouthAfricaID(t *testing.T) {
	res := validate.Check("8001015009087", validate.IDSA)
	if !res.IsValid {
		t.Fatalf("expected valid SA ID, got err=%v", res.Error)
	}
}

func TestCheckSouthAfricaIDInvalidChecksum(t *testing.T) {
	res := validate.Check("8001015009088", validate.IDSA)
	if res.IsValid || res.Error == nil {
		t.Fatalf("expected checksum failure, got %v", res)
	}
}

func TestExtractZA(t *testing.T) {
	info, err := validate.ExtractZA("8001015009087")
	if err != nil {
		t.Fatal(err)
	}
	if info.Gender != validate.GenderMale {
		t.Fatalf("unexpected gender: %v", info.Gender)
	}
	if info.Citizenship != validate.StatusCitizen {
		t.Fatalf("unexpected citizenship: %v", info.Citizenship)
	}
	if info.DOB.Year() != 1980 || info.DOB.Month() != 1 || info.DOB.Day() != 1 {
		t.Fatalf("unexpected DOB: %v", info.DOB)
	}
}

func TestMask(t *testing.T) {
	if got := validate.Mask("12345678901", validate.IDNIN); got != "********901" {
		t.Fatalf("mask failed: %q", got)
	}
	if got := validate.Mask("GHA-123456789-9", validate.IDGHA); got != "GHA-*****-9" {
		t.Fatalf("mask failed: %q", got)
	}
}

func TestEnrichInvalid(t *testing.T) {
	res := validate.Result{IsValid: false, IDType: validate.IDSA, Cleaned: ""}
	info, err := res.Enrich()
	if info != nil || err != nil {
		t.Fatalf("expected nil info for invalid result, got %v %v", info, err)
	}
}
