package i18n

import (
	"strconv"
	"strings"
)

// LangFromAcceptLanguage picks the best supported UI language from an
// Accept-Language header, honoring q-values and matching on the primary
// subtag (so "en-GB" counts as "en"). It reports ok=false when the header
// is empty or lists no supported language, so callers can fall back to
// Default.
func LangFromAcceptLanguage(header string) (lang string, ok bool) {
	best := -1.0
	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		primary, _, _ := strings.Cut(strings.TrimSpace(tag), "-")
		primary = strings.ToLower(primary)
		if !supported[primary] {
			continue
		}
		q := acceptQuality(params)
		if q > best {
			best = q
			lang = primary
		}
	}
	if best <= 0 {
		return "", false
	}
	return lang, true
}

func acceptQuality(params string) float64 {
	for param := range strings.SplitSeq(params, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(param), "=")
		if !found || strings.ToLower(strings.TrimSpace(key)) != "q" {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || q < 0 || q > 1 {
			return 0
		}
		return q
	}
	return 1
}
