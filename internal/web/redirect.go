package web

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strings"
	"waldi/internal/store"
)

// reservedPublicBlogPaths are first-path-segment app routes that must not be
// redirected from a writer subdomain to their custom domain.
var reservedPublicBlogPaths = map[string]bool{
	"login":           true,
	"signup":          true,
	"logout":          true,
	"verify-email":    true,
	"forgot-password": true,
	"reset-password":  true,
	"api":             true,
	"auth":            true,
	"you":             true,
	"settings":        true,
	"write":           true,
	"inbox":           true,
	"letters":         true,
	"wildcard":        true,
	"follow":          true,
	"unfollow":        true,
	"static":          true,
	"media":           true,
	"internal":        true,
}

// redirect sends the client to dest. Cross-host redirects on localhost subdomains
// use an HTML page because Chrome rejects them as ERR_INVALID_REDIRECT.
func redirect(w http.ResponseWriter, r *http.Request, dest string) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		dest = "/"
	}
	if isCrossHostRedirect(r, dest) {
		renderCrossHostRedirect(w, dest)
		return
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func isCrossHostRedirect(r *http.Request, dest string) bool {
	if !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
		return false
	}
	u, err := url.Parse(dest)
	if err != nil {
		return false
	}
	if !isLocalDevHost(r.Host) && !isLocalDevHost(u.Host) {
		return false
	}
	return hostWithoutPort(u.Host) != hostWithoutPort(r.Host)
}

func renderCrossHostRedirect(w http.ResponseWriter, dest string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	escaped := html.EscapeString(dest)
	js, _ := json.Marshal(dest)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8">`))
	_, _ = w.Write([]byte(`<meta http-equiv="refresh" content="0;url=` + escaped + `">`))
	_, _ = w.Write([]byte(`</head><body><script>location.replace(` + string(js) + `)</script>`))
	_, _ = w.Write([]byte(`<p><a href="` + escaped + `">Continue</a></p></body></html>`))
}

// withQueryParam adds key=value to dest's query string, preserving any
// existing query parameters.
func withQueryParam(dest, key, value string) string {
	u, err := url.Parse(dest)
	if err != nil {
		return dest
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

// permanentRedirect sends the client to dest with a 301 status. Cross-host
// redirects on localhost subdomains use an HTML page because Chrome rejects
// them as ERR_INVALID_REDIRECT.
func permanentRedirect(w http.ResponseWriter, r *http.Request, dest string) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		dest = "/"
	}
	if isCrossHostRedirect(r, dest) {
		renderCrossHostRedirect(w, dest)
		return
	}
	http.Redirect(w, r, dest, http.StatusMovedPermanently)
}

func isPublicBlogRedirectPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return true
	}
	switch path {
	case "/feed.xml", "/sitemap.xml", "/robots.txt":
		return true
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return false
	}
	return !reservedPublicBlogPaths[rest]
}

// subdomainToCustomDomainRedirect builds the canonical custom-domain URL for
// a public blog request served on a writer subdomain. Returns "" when no
// redirect should happen.
func subdomainToCustomDomainRedirect(r *http.Request, baseDomain string, owner store.User) string {
	if BlogFromHost(r.Host, baseDomain) == nil {
		return ""
	}
	if !isPublicBlogRedirectPath(r.URL.Path) {
		return ""
	}
	domain, ok := owner.ActiveCustomDomain()
	if !ok {
		return ""
	}
	dest := requestScheme(r) + "://" + domain + requestPort(r) + r.URL.Path
	if r.URL.RawQuery != "" {
		dest += "?" + r.URL.RawQuery
	}
	return dest
}

// maybeRedirectSubdomainToCustomDomain permanently redirects public blog
// traffic from a writer subdomain to the owner's verified custom domain.
func (s *Server) maybeRedirectSubdomainToCustomDomain(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	blog := BlogFromHost(r.Host, s.baseDomain)
	if blog == nil || !isPublicBlogRedirectPath(r.URL.Path) {
		return false
	}
	if s.store == nil {
		return false
	}

	owner, err := s.store.UserByUsername(r.Context(), blog.Username)
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		s.logger.Error("loading owner for subdomain redirect", "err", err)
		return false
	}

	dest := subdomainToCustomDomainRedirect(r, s.baseDomain, owner)
	if dest == "" {
		return false
	}
	permanentRedirect(w, r, dest)
	return true
}
