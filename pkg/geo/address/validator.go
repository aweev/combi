package address

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	_ "embed"
)

//go:embed data/countries.json
var countryData []byte

type countryRules struct {
	Required []string `json:"required"`
}

var rules map[string]countryRules

func init() {
	if err := json.Unmarshal(countryData, &rules); err != nil {
		panic("address: failed to load countries.json: " + err.Error())
	}
}

// Validate checks the address against per-country rules.
func (a Address) Validate() error {
	reqFields := a.RequiredFields()
	if len(reqFields) == 0 {
		return nil // Unknown country or no specific rules
	}

	var missing []string
	val := reflect.ValueOf(a)

	for _, field := range reqFields {
		v := val.FieldByName(field)
		if !v.IsValid() {
			continue // Should not happen if JSON matches struct
		}
		if strings.TrimSpace(v.String()) == "" {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required address fields for %s: %s", a.Country, strings.Join(missing, ", "))
	}

	return nil
}

// RequiredFields returns the struct field names required for the country.
func (a Address) RequiredFields() []string {
	if r, ok := rules[strings.ToUpper(a.Country)]; ok {
		return r.Required
	}
	return nil
}
