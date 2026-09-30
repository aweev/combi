package money

type Currency struct {
	Code     string
	Exponent int // e.g., 2 for USD ($1.00), 0 for JPY (¥100), 3 for BHD
	Symbol   string
}

var (
	USD = Currency{Code: "USD", Exponent: 2, Symbol: "$"}
	EUR = Currency{Code: "EUR", Exponent: 2, Symbol: "€"}
	GBP = Currency{Code: "GBP", Exponent: 2, Symbol: "£"}
	JPY = Currency{Code: "JPY", Exponent: 0, Symbol: "¥"}

	// African launch-market currencies — foundation-phase stopgap set so
	// that GetCurrency and the fxcore default (NGN) resolve honestly. F2
	// replaces this hand-maintained map with the single embedded currency
	// registry shared by pkg/money (arithmetic), pkg/i18n/currency
	// (display), and pkg/compliance (legality per country).
	NGN = Currency{Code: "NGN", Exponent: 2, Symbol: "₦"}     // Nigeria
	GHS = Currency{Code: "GHS", Exponent: 2, Symbol: "GH₵"}   // Ghana
	KES = Currency{Code: "KES", Exponent: 2, Symbol: "KSh"}   // Kenya
	TZS = Currency{Code: "TZS", Exponent: 2, Symbol: "TSh"}   // Tanzania
	UGX = Currency{Code: "UGX", Exponent: 0, Symbol: "USh"}   // Uganda
	RWF = Currency{Code: "RWF", Exponent: 0, Symbol: "FRw"}    // Rwanda
	ETB = Currency{Code: "ETB", Exponent: 2, Symbol: "Br"}    // Ethiopia
	XOF = Currency{Code: "XOF", Exponent: 0, Symbol: "CFA"}    // UEMOA (Senegal)
	XAF = Currency{Code: "XAF", Exponent: 0, Symbol: "FCFA"}   // CEMAC (Cameroon)
	CDF = Currency{Code: "CDF", Exponent: 2, Symbol: "FC"}     // DR Congo
	ZAR = Currency{Code: "ZAR", Exponent: 2, Symbol: "R"}      // South Africa
	EGP = Currency{Code: "EGP", Exponent: 2, Symbol: "E£"}     // Egypt
	MAD = Currency{Code: "MAD", Exponent: 2, Symbol: "MAD"}    // Morocco
)

// Currencies allows lookup by code
var Currencies = map[string]Currency{
	"USD": USD,
	"EUR": EUR,
	"GBP": GBP,
	"JPY": JPY,
	"NGN": NGN,
	"GHS": GHS,
	"KES": KES,
	"TZS": TZS,
	"UGX": UGX,
	"RWF": RWF,
	"ETB": ETB,
	"XOF": XOF,
	"XAF": XAF,
	"CDF": CDF,
	"ZAR": ZAR,
	"EGP": EGP,
	"MAD": MAD,
}

// GetCurrency looks up a currency by code (safe default is USD)
func GetCurrency(code string) Currency {
	if c, ok := Currencies[code]; ok {
		return c
	}
	return USD // Default or handle error
}
