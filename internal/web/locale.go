package web

import (
	"net/http"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

const localeCookie = "waldi_lang"

// resolveLocale picks the UI language for a request: the signed-in user's
// saved preference, then the language cookie, then Cloudflare's CF-IPCountry
// geolocation header (a proxy for the visitor's region/timezone), falling
// back to i18n.Default.
func resolveLocale(r *http.Request, user *store.User) (lang, dir string) {
	if user != nil && i18n.Supported(user.Locale) {
		lang = user.Locale
	} else if cookie, err := r.Cookie(localeCookie); err == nil && i18n.Supported(cookie.Value) {
		lang = cookie.Value
	} else if fromCountry, ok := i18n.LangFromCountry(r.Header.Get("CF-IPCountry")); ok {
		lang = fromCountry
	} else {
		lang = i18n.Default
	}
	return lang, i18n.Dir(lang)
}

func setLocaleCookie(w http.ResponseWriter, r *http.Request, baseDomain, lang string) {
	c := &http.Cookie{
		Name:     localeCookie,
		Value:    lang,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestScheme(r) == "https",
	}
	if domain := sessionCookieDomain(r.Host, baseDomain); domain != "" {
		c.Domain = domain
	}
	http.SetCookie(w, c)
}
