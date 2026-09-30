package kyctiers

import (
	_ "embed"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed data/tiers.yaml
var tiersData []byte

// Tier defines the KYC requirements for one level in one country.
type Tier struct {
	Level             int      `yaml:"level"`
	RequiredDocuments []string `yaml:"required_documents"`
	AutoApprovable    bool     `yaml:"auto_approvable"`
	ExpiryYears       int      `yaml:"expiry_years"`
	Notes             string   `yaml:"notes"`
	Regulator         string   `yaml:"-"` // injected from parent
}

type countryTiers struct {
	Regulator string `yaml:"regulator"`
	Tiers     []Tier `yaml:"tiers"`
}

var registry map[string][]Tier

func init() {
	var raw map[string]countryTiers
	if err := yaml.Unmarshal(tiersData, &raw); err != nil {
		panic("compliance/kyctiers: failed to parse tiers.yaml: " + err.Error())
	}

	registry = make(map[string][]Tier)
	for country, data := range raw {
		for i := range data.Tiers {
			data.Tiers[i].Regulator = data.Regulator
		}
		registry[strings.ToUpper(country)] = data.Tiers
	}
}

// For returns the tier definitions for country (ISO2) and level.
func For(country string, level int) (Tier, bool) {
	tiers, ok := registry[strings.ToUpper(country)]
	if !ok {
		return Tier{}, false
	}
	for _, t := range tiers {
		if t.Level == level {
			return t, true
		}
	}
	return Tier{}, false
}

// AllFor returns all tier definitions for a country.
func AllFor(country string) []Tier {
	return registry[strings.ToUpper(country)]
}
