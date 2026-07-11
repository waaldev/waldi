package i18n

// persianCountries lists ISO 3166-1 alpha-2 country codes where Waldi
// defaults anonymous visitors to Persian, based on Cloudflare's CF-IPCountry
// geolocation header.
var persianCountries = map[string]bool{
	"IR": true, // Iran
	"AF": true, // Afghanistan (Dari)
}

// LangFromCountry maps a CF-IPCountry code to a supported UI language. It
// reports ok=false for an empty code or Cloudflare's placeholders for
// unknown location ("XX") and Tor exit nodes ("T1"), so callers can fall
// back to Default instead of guessing.
func LangFromCountry(country string) (lang string, ok bool) {
	switch country {
	case "", "XX", "T1":
		return "", false
	}
	if persianCountries[country] {
		return "fa", true
	}
	return "en", true
}
