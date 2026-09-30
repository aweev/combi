// Package address handles structured African addresses.
package address

import (
	"strings"

	"github.com/combi/pkg/geo/pluscode"
	"github.com/combi/pkg/i18n/locale"
)

// Address is a flexible, Africa-aware structured address.
type Address struct {
	Line1      string
	Line2      string
	City       string
	District   string
	Region     string
	PostalCode string

	Quartier     string
	Woreda       string
	Kebele       string
	LGA          string
	Ward         string
	SubCity      string
	County       string
	SubCounty    string
	Constituency string
	Commune      string

	PlusCode  string
	Latitude  float64
	Longitude float64

	Country    string
	Verified   bool
	VerifiedAt string
}

// Format returns a human-readable address string per country conventions.
func (a Address) Format(loc locale.Locale) string {
	var lines []string
	country := strings.ToUpper(a.Country)

	switch country {
	case "CM":
		if a.Line1 != "" {
			lines = append(lines, a.Line1)
		}
		if a.Quartier != "" {
			lines = append(lines, "Quartier "+a.Quartier)
		}
		lines = append(lines, a.City)
		if strings.HasPrefix(string(loc), "fr") {
			lines = append(lines, "Cameroun")
		} else {
			lines = append(lines, "Cameroon")
		}

	case "NG":
		if a.Line1 != "" {
			lines = append(lines, a.Line1)
		}
		var l2 []string
		if a.City != "" {
			l2 = append(l2, a.City)
		}
		if a.LGA != "" {
			l2 = append(l2, a.LGA)
		}
		if len(l2) > 0 {
			lines = append(lines, strings.Join(l2, ", "))
		}
		if a.Region != "" {
			lines = append(lines, a.Region+" State")
		}
		lines = append(lines, "Nigeria")

	case "ET":
		lines = append(lines, a.City)
		if a.SubCity != "" {
			lines = append(lines, a.SubCity+" Sub-city")
		}
		if a.Woreda != "" {
			lines = append(lines, "Woreda "+a.Woreda)
		}
		if a.Kebele != "" {
			lines = append(lines, "Kebele "+a.Kebele)
		}
		if strings.HasPrefix(string(loc), "am") {
			lines = append(lines, "ኢትዮጵያ") // Ethiopia in Amharic
		} else {
			lines = append(lines, "Ethiopia")
		}

	case "CD":
		if a.Line1 != "" {
			lines = append(lines, a.Line1)
		}
		if a.Quartier != "" {
			lines = append(lines, "Quartier "+a.Quartier)
		}
		if a.Commune != "" {
			lines = append(lines, "Commune de "+a.Commune)
		}
		lines = append(lines, a.City)
		if strings.HasPrefix(string(loc), "fr") {
			lines = append(lines, "RD Congo")
		} else {
			lines = append(lines, "DR Congo")
		}

	default:
		// Generic fallback
		if a.Line1 != "" {
			lines = append(lines, a.Line1)
		}
		if a.City != "" {
			lines = append(lines, a.City)
		}
		if a.Region != "" {
			lines = append(lines, a.Region)
		}
		if a.Country != "" {
			lines = append(lines, a.Country)
		}
	}

	return strings.Join(lines, "\n")
}

// FromPlusCode constructs a minimal Address from a Plus Code.
func FromPlusCode(code string) (Address, error) {
	lat, lng, _, _, _, _, err := pluscode.Decode(code)
	if err != nil {
		return Address{}, err
	}
	// In a full implementation, we would do an offline reverse-geocode
	// to infer the country ISO2. Here we just return the PlusCode data.
	return Address{
		PlusCode:  code,
		Latitude:  lat,
		Longitude: lng,
	}, nil
}
