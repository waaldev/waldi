package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// sessionCookieDomain picks a Set-Cookie Domain that shares sessions across
// writer subdomains in production and waldi.test. On localhost, cookies are
// host-only; use /auth/bridge to sync sessions onto *.localhost.
func sessionCookieDomain(host, baseDomain string) string {
	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))

	if host == "waldi.test" || strings.HasSuffix(host, ".waldi.test") {
		return ".waldi.test"
	}

	base := strings.ToLower(strings.TrimSpace(baseDomain))
	if !isLocalDevHost(host) && base != "" && (host == base || strings.HasSuffix(host, "."+base)) {
		return "." + base
	}

	return ""
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time, domain string, secure bool) {
	c := &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	if domain != "" {
		c.Domain = domain
	}
	http.SetCookie(w, c)
}

func clearSessionCookie(w http.ResponseWriter, domain string, secure bool) {
	c := &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	if domain != "" {
		c.Domain = domain
	}
	http.SetCookie(w, c)
}

// setBridgeCookie stores an opaque probe token, separate from the real
// session token, so that JS on a cross-site custom domain can ask /api/me
// and /api/auth/bridge whether this browser is logged in on the app's
// shared domain. SameSite=None+Secure is what makes it (unlike
// waldi_session) actually reach the app on a cross-site fetch; scope it to
// the shared subdomain cookie domain only, and only over HTTPS since
// SameSite=None cookies without Secure are rejected by browsers.
func setBridgeCookie(w http.ResponseWriter, token string, expires time.Time, domain string, secure bool) {
	if domain == "" || !secure {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     bridgeCookie,
		Value:    token,
		Domain:   domain,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})
}

func clearBridgeCookie(w http.ResponseWriter, domain string, secure bool) {
	if domain == "" || !secure {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     bridgeCookie,
		Value:    "",
		Domain:   domain,
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})
}

func safeNextURL(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fallback
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fallback
	}
	return u.String()
}
