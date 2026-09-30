package limits

import (
	_ "embed"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

//go:embed data/limits.yaml
var limitsData []byte

// Limit defines the transaction caps for one KYC level in one country.
type Limit struct {
	Country        string
	KYCLevel       int
	DailySendUSD   decimal.Decimal
	MonthlySendUSD decimal.Decimal
	SingleTxUSD    decimal.Decimal
	CrossBorderPct decimal.Decimal // e.g. 0.5 = 50%
	Unlimited      bool            // true if limits are "unlimited" (like NG Tier 3)
	Notes          string
}

// rawLimit represents the YAML schema
type rawLimit struct {
	KYCLevel       int    `yaml:"kyc_level"`
	DailySendUSD   string `yaml:"daily_send_usd"`
	MonthlySendUSD string `yaml:"monthly_send_usd"`
	SingleTxUSD    string `yaml:"single_tx_usd"`
	CrossBorderPct string `yaml:"cross_border_pct"`
	Notes          string `yaml:"notes"`
}

var registry map[string][]Limit

func init() {
	var raw map[string][]rawLimit
	if err := yaml.Unmarshal(limitsData, &raw); err != nil {
		panic("compliance/limits: failed to parse limits.yaml: " + err.Error())
	}

	registry = make(map[string][]Limit)
	for country, rawLimits := range raw {
		country = strings.ToUpper(country)
		var parsed []Limit

		for _, rl := range rawLimits {
			lim := Limit{
				Country:  country,
				KYCLevel: rl.KYCLevel,
				Notes:    rl.Notes,
			}

			// Handle 'unlimited' caps
			if strings.ToLower(rl.DailySendUSD) == "unlimited" {
				lim.Unlimited = true
				lim.CrossBorderPct = parseDecimalOrZero(rl.CrossBorderPct)
			} else {
				lim.DailySendUSD = parseDecimalOrZero(rl.DailySendUSD)
				lim.MonthlySendUSD = parseDecimalOrZero(rl.MonthlySendUSD)
				lim.SingleTxUSD = parseDecimalOrZero(rl.SingleTxUSD)
				lim.CrossBorderPct = parseDecimalOrZero(rl.CrossBorderPct)
			}
			parsed = append(parsed, lim)
		}
		registry[country] = parsed
	}
}

func parseDecimalOrZero(val string) decimal.Decimal {
	if val == "" {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(val)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// For returns the limit for country and KYC level.
func For(country string, kycLevel int) (Limit, bool) {
	limits, ok := registry[strings.ToUpper(country)]
	if !ok {
		return Limit{}, false
	}
	for _, l := range limits {
		if l.KYCLevel == kycLevel {
			return l, true
		}
	}
	return Limit{}, false
}
