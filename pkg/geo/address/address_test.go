package address_test

import (
	"strings"
	"testing"

	"github.com/combi/pkg/geo/address"
	"github.com/combi/pkg/i18n/locale"
)

func TestAddress_Format_Cameroon(t *testing.T) {
	addr := address.Address{
		Country:  "CM",
		Line1:    "BP 1234",
		Quartier: "Bastos",
		City:     "Yaoundé",
		Region:   "Centre",
	}

	// Francophone formatting
	gotFR := addr.Format(locale.FrCM)
	if !strings.Contains(gotFR, "Quartier Bastos") || !strings.Contains(gotFR, "Cameroun") {
		t.Errorf("FR CM format failed: \n%s", gotFR)
	}

	// Anglophone formatting
	gotEN := addr.Format(locale.EnCM)
	if !strings.Contains(gotEN, "Cameroon") {
		t.Errorf("EN CM format failed: \n%s", gotEN)
	}
}

func TestAddress_Format_Nigeria(t *testing.T) {
	addr := address.Address{
		Country: "NG",
		Line1:   "15 Ademola Adetokunbo Crescent",
		City:    "Wuse 2",
		LGA:     "Abuja Municipal",
		Region:  "FCT",
	}
	got := addr.Format(locale.EnNG)
	if !strings.Contains(got, "Wuse 2, Abuja Municipal") || !strings.Contains(got, "FCT State") {
		t.Errorf("NG format failed: \n%s", got)
	}
}

func TestAddress_Format_Ethiopia_Amharic(t *testing.T) {
	addr := address.Address{
		Country: "ET",
		City:    "Addis Ababa",
		SubCity: "Bole",
		Woreda:  "03",
		Kebele:  "01",
	}
	got := addr.Format(locale.AmET)
	if !strings.Contains(got, "Bole Sub-city") || !strings.Contains(got, "ኢትዮጵያ") {
		t.Errorf("ET format failed: \n%s", got)
	}
}

func TestAddress_Format_DRC_French(t *testing.T) {
	addr := address.Address{
		Country:  "CD",
		Line1:    "Av. de l'Université n°123",
		Quartier: "Ozone",
		Commune:  "Ngaliema",
		City:     "Kinshasa",
	}
	// Note: Fallback or unknown locale uses DR Congo / RD Congo logic
	// We use FrCM to trigger the "fr" prefix match for Francophone output.
	got := addr.Format(locale.FrCM)
	if !strings.Contains(got, "Commune de Ngaliema") || !strings.Contains(got, "RD Congo") {
		t.Errorf("CD format failed: \n%s", got)
	}
}

func TestAddress_Validate_KE_RequiresCounty(t *testing.T) {
	addr := address.Address{
		Country: "KE",
		Line1:   "123 Westlands Road",
		City:    "Nairobi",
	}
	// Missing "County"
	err := addr.Validate()
	if err == nil || !strings.Contains(err.Error(), "County") {
		t.Error("Kenya address should require County")
	}
}

func TestAddress_Validate_ZA_RequiresPostalCode(t *testing.T) {
	addr := address.Address{
		Country: "ZA",
		Line1:   "123 Nelson Mandela Blvd",
		City:    "Cape Town",
		Region:  "Western Cape",
	}
	// Missing PostalCode
	err := addr.Validate()
	if err == nil || !strings.Contains(err.Error(), "PostalCode") {
		t.Error("South Africa address should require PostalCode")
	}
}

func TestAddress_FromPlusCode(t *testing.T) {
	fullCode := "6FV2VR3W+QG"
	addr, err := address.FromPlusCode(fullCode)
	if err != nil {
		t.Fatalf("FromPlusCode failed: %v", err)
	}
	if addr.PlusCode != fullCode {
		t.Errorf("Expected PlusCode %s, got %s", fullCode, addr.PlusCode)
	}
	if addr.Latitude == 0 || addr.Longitude == 0 {
		t.Error("Expected valid coordinates to be derived from Plus Code")
	}
}
