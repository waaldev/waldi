package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"waldi/internal/store"
)

type BlogHost struct {
	Username string
	Custom   bool
}

var reservedSubdomains = map[string]bool{
	"admin":        true,
	"api":          true,
	"app":          true,
	"blog":         true,
	"cname":        true,
	"cdn":          true,
	"mail":         true,
	"smtp":         true,
	"imap":         true,
	"pop":          true,
	"ftp":          true,
	"static":       true,
	"media":        true,
	"support":      true,
	"help":         true,
	"status":       true,
	"docs":         true,
	"www":          true,
	"write":        true,
	"inbox":        true,
	"you":          true,
	"settings":     true,
	"login":        true,
	"logout":       true,
	"signup":       true,
	"invite":       true,
	"letters":      true,
	"autoconfig":   true,
	"autodiscover": true,
}

func BlogFromHost(host, baseDomain string) *BlogHost {
	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))

	if host == "" || baseDomain == "" {
		return nil
	}

	username, ok := strings.CutSuffix(host, "."+baseDomain)
	if ok && validBlogUsername(username) {
		return &BlogHost{Username: username}
	}

	if username, ok := strings.CutSuffix(host, ".localhost"); ok && validBlogUsername(username) {
		return &BlogHost{Username: username}
	}

	if username, ok := strings.CutSuffix(host, ".waldi.test"); ok && validBlogUsername(username) {
		return &BlogHost{Username: username}
	}

	return nil
}

// isBlogHost resolves a request Host to a blog, first via the cheap
// subdomain suffix match, then by consulting (and caching) verified custom
// domain mappings in the database.
func (s *Server) isBlogHost(ctx context.Context, host string) *BlogHost {
	if blog := BlogFromHost(host, s.baseDomain); blog != nil {
		return blog
	}

	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))
	if host == "" || !strings.Contains(host, ".") {
		return nil
	}

	if username, ok := s.customDomains.lookup(host); ok {
		if username == "" {
			return nil
		}
		return &BlogHost{Username: username, Custom: true}
	}

	if s.store == nil {
		return nil
	}

	user, err := s.store.UserByCustomDomain(ctx, host)
	if errors.Is(err, store.ErrNotFound) {
		s.customDomains.store(host, "", customDomainNegativeTTL)
		return nil
	}
	if err != nil {
		s.logger.Error("resolving custom domain", "err", err)
		return nil
	}

	s.customDomains.store(host, user.Username, customDomainPositiveTTL)
	return &BlogHost{Username: user.Username, Custom: true}
}

// cookieDomain returns the Set-Cookie Domain attribute that scopes a
// session cookie to both the base domain and its writer subdomains
// (username.baseDomain), so signing in on one host carries over to the
// other. Returns "" when host doesn't match a recognized base domain,
// meaning the cookie stays host-only.
func cookieDomain(host, baseDomain string) string {
	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))

	for _, base := range []string{baseDomain, "localhost", "waldi.test"} {
		if base == "" {
			continue
		}
		if host == base || strings.HasSuffix(host, "."+base) {
			return "." + base
		}
	}
	return ""
}

func hostWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func validBlogUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 || reservedSubdomains[username] {
		return false
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isLocalDevHost(host string) bool {
	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))
	switch host {
	case "localhost", "127.0.0.1", "waldi.test":
		return true
	default:
		return strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".waldi.test")
	}
}

func localBlogDomain(host string) string {
	host = hostWithoutPort(strings.ToLower(strings.TrimSpace(host)))
	if host == "waldi.test" || strings.HasSuffix(host, ".waldi.test") {
		return "waldi.test"
	}
	return "localhost"
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	return "http"
}

func requestPort(r *http.Request) string {
	_, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return ""
	}
	scheme := requestScheme(r)
	if scheme == "http" && port == "80" {
		return ""
	}
	if scheme == "https" && port == "443" {
		return ""
	}
	return ":" + port
}

// PublicBlogURL builds an absolute URL for a writer's public blog. On local
// dev hosts it uses username.localhost (or username.waldi.test) so subdomains
// work without extra DNS setup.
func PublicBlogURL(r *http.Request, baseDomain, username, path string) string {
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if path == "" {
		path = "/"
	} else if path[0] != '/' {
		path = "/" + path
	}

	domain := baseDomain
	if isLocalDevHost(r.Host) {
		domain = localBlogDomain(r.Host)
	}

	return requestScheme(r) + "://" + username + "." + domain + requestPort(r) + path
}

// PublicBlogURLForOwner builds the canonical public URL for a blog, using
// the owner's verified custom domain when they have one, and falling back
// to their username.baseDomain URL otherwise.
func PublicBlogURLForOwner(r *http.Request, baseDomain string, owner store.User, path string) string {
	if domain, ok := owner.ActiveCustomDomain(); ok {
		if path == "" {
			path = "/"
		} else if path[0] != '/' {
			path = "/" + path
		}
		return requestScheme(r) + "://" + domain + requestPort(r) + path
	}
	return PublicBlogURL(r, baseDomain, owner.Username, path)
}
