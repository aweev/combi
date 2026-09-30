package country

import "strings"

type Region int

const (
	WestAfrica Region = iota
	CentralAfrica
	EastAfrica
	NorthAfrica
	SouthernAfrica
	HornOfAfrica
)

// CountryInfo holds Africa OS-relevant metadata for a country.
type CountryInfo struct {
	ISO2           string
	ISO3           string
	Name           string
	NameFR         string
	NameAR         string
	Capital        string
	Region         Region
	CallingCode    string
	Currencies     []string
	Languages      []string
	PrimaryLocale  string
	TimeZones      []string
	PhoneNumberLen []int
	HasPostalCodes bool

	// Economic & Regulatory flags
	IsEACMember    bool
	IsECOWASMember bool
	IsCOMESAMember bool
	IsSADCMember   bool
	IsCEMACMember  bool
	IsUEMOAMember  bool
	KYCRegulator   string
}

// ── Internal Data Registry ───────────────────────────────────────────────────

var countries = map[string]CountryInfo{
	"CM": {
		ISO2: "CM", ISO3: "CMR", Name: "Cameroon", NameFR: "Cameroun", NameAR: "الكاميرون",
		Capital: "Yaoundé", Region: CentralAfrica, CallingCode: "+237",
		Currencies: []string{"XAF"}, Languages: []string{"fr", "en"}, PrimaryLocale: "fr-CM",
		TimeZones: []string{"Africa/Douala"}, PhoneNumberLen: []int{9}, HasPostalCodes: false,
		IsCEMACMember: true, KYCRegulator: "COBAC",
	},
	"NG": {
		ISO2: "NG", ISO3: "NGA", Name: "Nigeria", NameFR: "Nigéria", NameAR: "نيجيريا",
		Capital: "Abuja", Region: WestAfrica, CallingCode: "+234",
		Currencies: []string{"NGN"}, Languages: []string{"en", "ha", "yo", "ig"}, PrimaryLocale: "en-NG",
		TimeZones: []string{"Africa/Lagos"}, PhoneNumberLen: []int{10, 11}, HasPostalCodes: false,
		IsECOWASMember: true, KYCRegulator: "CBN",
	},
	"KE": {
		ISO2: "KE", ISO3: "KEN", Name: "Kenya", NameFR: "Kenya", NameAR: "كينيا",
		Capital: "Nairobi", Region: EastAfrica, CallingCode: "+254",
		Currencies: []string{"KES"}, Languages: []string{"sw", "en"}, PrimaryLocale: "sw-KE",
		TimeZones: []string{"Africa/Nairobi"}, PhoneNumberLen: []int{9, 10}, HasPostalCodes: false,
		IsEACMember: true, IsCOMESAMember: true, KYCRegulator: "CBK",
	},
	"ZA": {
		ISO2: "ZA", ISO3: "ZAF", Name: "South Africa", NameFR: "Afrique du Sud", NameAR: "جنوب أفريقيا",
		Capital: "Pretoria", Region: SouthernAfrica, CallingCode: "+27",
		Currencies: []string{"ZAR"}, Languages: []string{"en", "zu", "xh"}, PrimaryLocale: "en-ZA",
		TimeZones: []string{"Africa/Johannesburg"}, PhoneNumberLen: []int{9}, HasPostalCodes: true, // ZA uses postal codes heavily
		IsSADCMember: true, KYCRegulator: "SARB",
	},
	"ET": {
		ISO2: "ET", ISO3: "ETH", Name: "Ethiopia", NameFR: "Éthiopie", NameAR: "إثيوبيا",
		Capital: "Addis Ababa", Region: HornOfAfrica, CallingCode: "+251",
		Currencies: []string{"ETB"}, Languages: []string{"am", "om", "ti"}, PrimaryLocale: "am-ET",
		TimeZones: []string{"Africa/Addis_Ababa"}, PhoneNumberLen: []int{9}, HasPostalCodes: false,
		IsCOMESAMember: true, KYCRegulator: "NBE",
	},
	"GH": {
		ISO2: "GH", ISO3: "GHA", Name: "Ghana", NameFR: "Ghana", NameAR: "غانا",
		Capital: "Accra", Region: WestAfrica, CallingCode: "+233",
		Currencies: []string{"GHS"}, Languages: []string{"en", "ak", "ee"}, PrimaryLocale: "en-GH",
		TimeZones: []string{"Africa/Accra"}, PhoneNumberLen: []int{9}, HasPostalCodes: false, // Ghana uses digital addresses (GhanaPostGPS)
		IsECOWASMember: true, KYCRegulator: "BOG",
	},
	"SN": {
		ISO2: "SN", ISO3: "SEN", Name: "Senegal", NameFR: "Sénégal", NameAR: "السنغال",
		Capital: "Dakar", Region: WestAfrica, CallingCode: "+221",
		Currencies: []string{"XOF"}, Languages: []string{"fr", "wo"}, PrimaryLocale: "fr-SN",
		TimeZones: []string{"Africa/Dakar"}, PhoneNumberLen: []int{9}, HasPostalCodes: false,
		IsECOWASMember: true, IsUEMOAMember: true, KYCRegulator: "BCEAO",
	},
	"CD": {
		ISO2: "CD", ISO3: "COD", Name: "DR Congo", NameFR: "RD Congo", NameAR: "جمهورية الكونغو الديمقراطية",
		Capital: "Kinshasa", Region: CentralAfrica, CallingCode: "+243",
		Currencies: []string{"CDF", "USD"}, Languages: []string{"fr", "sw", "ln"}, PrimaryLocale: "fr-CD",
		TimeZones: []string{"Africa/Kinshasa", "Africa/Lubumbashi"}, PhoneNumberLen: []int{9}, HasPostalCodes: false,
		IsSADCMember: true, IsCOMESAMember: true, IsEACMember: true, KYCRegulator: "BCC",
	},
}

// Map of shared land borders (ISO2 -> []ISO2)
var borders = map[string][]string{
	"CM": {"NG", "TD", "CF", "CG", "GA", "GQ"},                   // Cameroon
	"NG": {"BJ", "NE", "TD", "CM"},                               // Nigeria
	"KE": {"SS", "ET", "SO", "UG", "TZ"},                         // Kenya
	"ZA": {"NA", "BW", "ZW", "MZ", "SZ", "LS"},                   // South Africa
	"ET": {"ER", "DJ", "SO", "KE", "SS", "SD"},                   // Ethiopia
	"GH": {"CI", "BF", "TG"},                                     // Ghana
	"SN": {"MR", "ML", "GN", "GW", "GM"},                         // Senegal
	"CD": {"CG", "CF", "SS", "UG", "RW", "BI", "TZ", "ZM", "AO"}, // DRC
}

// ── Exported API ─────────────────────────────────────────────────────────────

// Get returns the CountryInfo for an ISO 3166-1 alpha-2 code.
func Get(iso2 string) (CountryInfo, bool) {
	c, ok := countries[strings.ToUpper(iso2)]
	return c, ok
}

// ByRegion returns all countries in a region.
func ByRegion(r Region) []CountryInfo {
	var res []CountryInfo
	for _, c := range countries {
		if c.Region == r {
			res = append(res, c)
		}
	}
	return res
}

// ByCurrency returns all countries using an ISO 4217 currency code.
func ByCurrency(code string) []CountryInfo {
	code = strings.ToUpper(code)
	var res []CountryInfo
	for _, c := range countries {
		for _, curr := range c.Currencies {
			if curr == code {
				res = append(res, c)
				break
			}
		}
	}
	return res
}

// Neighbours returns countries that share a land border.
func Neighbours(iso2 string) []CountryInfo {
	iso2 = strings.ToUpper(iso2)
	var res []CountryInfo
	for _, neighborISO := range borders[iso2] {
		if c, ok := countries[neighborISO]; ok {
			res = append(res, c)
		}
	}
	return res
}
