package sanctions

import "strings"

// Source represents a regulatory sanctions list.
type Source struct {
	ID          string
	Name        string
	Description string
	Region      string // "Global", "WestAfrica", "CentralAfrica", etc.
}

var sources = []Source{
	{ID: "UN_SC", Name: "UN Security Council Consolidated List", Region: "Global"},
	{ID: "US_OFAC", Name: "US OFAC SDN List", Region: "Global"},
	{ID: "EU_FSF", Name: "EU Financial Sanctions Files", Region: "Global"},
	{ID: "COBAC_AML", Name: "COBAC AML/CFT Watchlist", Region: "CentralAfrica"},
	{ID: "CBN_BVL", Name: "CBN Bank Verification Watchlist", Region: "WestAfrica"},
	{ID: "CBK_CRB", Name: "CBK Credit Reference Bureau Defaulters", Region: "EastAfrica"},
}

// ForCountry returns the list of sanction sources that must be checked for a given country.
func ForCountry(iso2 string) []Source {
	iso2 = strings.ToUpper(iso2)
	var active []Source

	for _, s := range sources {
		if s.Region == "Global" {
			active = append(active, s)
			continue
		}
		// Apply regional lists based on country
		if iso2 == "CM" || iso2 == "GA" || iso2 == "TD" {
			if s.Region == "CentralAfrica" {
				active = append(active, s)
			}
		}
		if iso2 == "NG" || iso2 == "GH" || iso2 == "SN" {
			if s.Region == "WestAfrica" {
				active = append(active, s)
			}
		}
		if iso2 == "KE" || iso2 == "TZ" || iso2 == "UG" {
			if s.Region == "EastAfrica" {
				active = append(active, s)
			}
		}
	}
	return active
}
