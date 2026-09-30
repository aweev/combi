package country_test

import (
	"testing"

	"github.com/combi/pkg/geo/country"
)

func TestCountry_Get_CM(t *testing.T) {
	cm, ok := country.Get("CM")
	if !ok {
		t.Fatal("Expected CM to be found")
	}
	if cm.NameFR != "Cameroun" {
		t.Errorf("Expected Cameroun, got %s", cm.NameFR)
	}
	if !cm.IsCEMACMember {
		t.Error("Expected Cameroon to be a CEMAC member")
	}
	if cm.KYCRegulator != "COBAC" {
		t.Errorf("Expected COBAC, got %s", cm.KYCRegulator)
	}
}

func TestCountry_ByCurrency_XAF(t *testing.T) {
	// Our test data only has Cameroon for XAF right now, but we verify it correctly filters.
	cfaCountries := country.ByCurrency("XAF")
	foundCM := false
	for _, c := range cfaCountries {
		if c.ISO2 == "CM" {
			foundCM = true
		}
	}
	if !foundCM {
		t.Error("ByCurrency(XAF) should include Cameroon (CM)")
	}
}

func TestCountry_Neighbours_NG_IncludesCM(t *testing.T) {
	neighbours := country.Neighbours("NG")
	foundCM := false
	for _, n := range neighbours {
		if n.ISO2 == "CM" {
			foundCM = true
			break
		}
	}
	if !foundCM {
		t.Error("Nigeria's neighbours should include Cameroon (CM)")
	}
}
