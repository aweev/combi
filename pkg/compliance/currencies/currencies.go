package currencies

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed data/allowed.yaml
var allowedData []byte

var allowed map[string][]string

func init() {
	var raw map[string][]string
	if err := yaml.Unmarshal(allowedData, &raw); err != nil {
		panic("compliance/currencies: failed to parse allowed.yaml: " + err.Error())
	}

	allowed = make(map[string][]string)
	for country, currencies := range raw {
		var upperCurrencies []string
		for _, c := range currencies {
			upperCurrencies = append(upperCurrencies, strings.ToUpper(c))
		}
		allowed[strings.ToUpper(country)] = upperCurrencies
	}
}

// IsAllowed returns true if the currency is legally permitted to be held/traded in the country.
func IsAllowed(country, currencyCode string) bool {
	country = strings.ToUpper(country)
	currencyCode = strings.ToUpper(currencyCode)

	currencies, ok := allowed[country]
	if !ok {
		return false
	}
	for _, c := range currencies {
		if c == currencyCode {
			return true
		}
	}
	return false
}
