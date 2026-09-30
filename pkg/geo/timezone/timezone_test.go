package timezone_test

import (
	"testing"
	"time"

	"github.com/combi/pkg/geo/timezone"
)

func TestTimezone_ByCountry_ET_UTC3(t *testing.T) {
	loc := timezone.ByCountry("ET")
	now := time.Now().In(loc)
	_, offset := now.Zone()
	if offset != 3*3600 { // UTC+3
		t.Errorf("Ethiopia should be UTC+3, got offset %d", offset)
	}
}

func TestTimezone_ByCountry_NG_UTC1(t *testing.T) {
	loc := timezone.ByCountry("NG")
	now := time.Now().In(loc)
	_, offset := now.Zone()
	if offset != 3600 { // UTC+1
		t.Errorf("Nigeria should be UTC+1, got offset %d", offset)
	}
}

func TestTimezone_ByCountry_ZA_UTC2(t *testing.T) {
	loc := timezone.ByCountry("ZA")
	now := time.Now().In(loc)
	_, offset := now.Zone()
	if offset != 2*3600 { // UTC+2
		t.Errorf("South Africa should be UTC+2, got offset %d", offset)
	}
}

func TestTimezone_ByCoords_Nairobi(t *testing.T) {
	loc, err := timezone.ByCoords(-1.2921, 36.8219) // Nairobi
	if err != nil {
		t.Fatalf("ByCoords failed: %v", err)
	}
	if loc.String() != "Africa/Nairobi" {
		t.Errorf("Expected Africa/Nairobi, got %s", loc.String())
	}
}

func TestTimezone_DSTNotObserved_NG(t *testing.T) {
	loc := timezone.ByCountry("NG")
	// January
	jan := time.Date(2026, 1, 15, 12, 0, 0, 0, loc)
	_, offsetJan := jan.Zone()
	// July
	jul := time.Date(2026, 7, 15, 12, 0, 0, 0, loc)
	_, offsetJul := jul.Zone()

	if offsetJan != offsetJul {
		t.Errorf("Nigeria does not observe DST, offsets should match: %d vs %d", offsetJan, offsetJul)
	}
}

func TestTimezone_DSTObserved_EG(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Cairo")
	if err != nil {
		t.Skip("Africa/Cairo tzdata missing")
	}
	jan := time.Date(2023, 1, 15, 12, 0, 0, 0, loc)
	_, offsetJan := jan.Zone()

	jul := time.Date(2023, 7, 15, 12, 0, 0, 0, loc)
	_, offsetJul := jul.Zone()

	if offsetJan == offsetJul {
		t.Errorf("Egypt observes DST, offsets should differ: %d vs %d", offsetJan, offsetJul)
	}
}

func TestTimezone_BusinessHours_NG_SundayNotBusiness(t *testing.T) {
	// A Sunday in 2026
	sunday := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	if timezone.BusinessHours(sunday, "NG") {
		t.Error("Sunday should not be business hours in Nigeria")
	}

	// A Tuesday
	tuesday := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	if !timezone.BusinessHours(tuesday, "NG") {
		t.Error("Tuesday 10am should be business hours in Nigeria")
	}
}

func TestTimezone_BusinessHours_MoroccoFriday(t *testing.T) {
	// A Friday in 2026
	friday := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	if timezone.BusinessHours(friday, "MA") {
		t.Error("Friday should be the weekend in Morocco (for the sake of Islamic workweek rules)")
	}

	// A Sunday
	sunday := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	if !timezone.BusinessHours(sunday, "MA") {
		t.Error("Sunday 10am should be a business day in Morocco")
	}
}
