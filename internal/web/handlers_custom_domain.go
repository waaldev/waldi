package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"waldi/internal/store"
)

const domainChallengeTokenBytes = 20

var customDomainFormatRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

func normalizeCustomDomain(raw string) string {
	domain := strings.ToLower(strings.TrimSpace(raw))
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	if host, _, err := net.SplitHostPort(domain); err == nil {
		domain = host
	}
	return domain
}

func validCustomDomainFormat(domain string) bool {
	return len(domain) <= 253 && customDomainFormatRe.MatchString(domain)
}

// placeholderCustomDomain rejects well-known example and test domains that
// writers don't actually own.
func placeholderCustomDomain(domain string) bool {
	switch domain {
	case "example.com", "example.org", "example.net", "example.io",
		"test.com", "test.example", "localhost", "invalid":
		return true
	}
	for _, suffix := range []string{".example.com", ".example.org", ".example.net", ".example.io", ".localhost", ".local", ".test", ".invalid"} {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

// reservedCustomDomain rejects the base domain and any of its subdomains,
// since those are already served by waldi's own subdomain routing.
func reservedCustomDomain(domain, baseDomain string) bool {
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	return domain == baseDomain || strings.HasSuffix(domain, "."+baseDomain)
}

func challengeHostFor(domain string) string {
	return "_waldi-challenge." + domain
}

// customDomainCNAMETarget is the hostname users should CNAME their custom domain to.
func customDomainCNAMETarget(baseDomain string) string {
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if baseDomain == "" {
		return "cname.waldi.blog"
	}
	return "cname." + baseDomain
}

func cnamePointsToTarget(cname, target string) bool {
	cname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(cname)), ".")
	target = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(target)), ".")
	return cname == target
}

// ipsOverlap reports whether domainIPs and targetIPs share at least one
// address. Used to verify apex/root domains, which can't hold a CNAME record
// (RFC 1035 ยง3.6.2): instead of pointing a CNAME at the routing hostname,
// the domain's own A/AAAA records must resolve to the same address(es) as
// that hostname.
func ipsOverlap(domainIPs, targetIPs []net.IP) bool {
	return slices.ContainsFunc(domainIPs, func(d net.IP) bool {
		return slices.ContainsFunc(targetIPs, d.Equal)
	})
}

func lookupIPs(ctx context.Context, host string) []net.IP {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	ips := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		ips[i] = addr.IP
	}
	return ips
}

func (s *Server) handleSetCustomDomain(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.form")
		return
	}

	domain := normalizeCustomDomain(r.FormValue("custom_domain"))
	if domain == "" {
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.required")
		return
	}
	if !validCustomDomainFormat(domain) {
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.invalid")
		return
	}
	if placeholderCustomDomain(domain) {
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.placeholder")
		return
	}
	if reservedCustomDomain(domain, s.baseDomain) {
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.reserved")
		return
	}

	token, err := newAuthToken(domainChallengeTokenBytes)
	if err != nil {
		s.logger.Error("creating domain challenge token", "err", err)
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.save")
		return
	}

	oldDomain := ""
	if user.CustomDomain != nil {
		oldDomain = *user.CustomDomain
	}

	if _, err := s.store.SetCustomDomain(r.Context(), user.ID, domain, token); err != nil {
		if errors.Is(err, store.ErrDomainTaken) {
			s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.taken")
			return
		}
		s.logger.Error("setting custom domain", "err", err)
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.save")
		return
	}

	s.customDomains.invalidate(oldDomain)
	s.customDomains.invalidate(domain)
	s.purgePublicCache(user.Username, oldDomain)
	http.Redirect(w, r, "/settings?domain=set", http.StatusSeeOther)
}

func (s *Server) handleVerifyCustomDomain(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.db")
		return
	}

	current, err := s.store.UserByID(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("reloading user for domain verification", "err", err)
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.save")
		return
	}
	if current.CustomDomain == nil || current.CustomDomainToken == nil {
		s.renderDomainSettingsError(w, r, current, "blog.settings.domain.error.none")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	records, _ := net.DefaultResolver.LookupTXT(ctx, challengeHostFor(*current.CustomDomain))
	matched := false
	for _, record := range records {
		if strings.TrimSpace(record) == *current.CustomDomainToken {
			matched = true
			break
		}
	}
	if !matched {
		s.renderDomainSettingsError(w, r, current, "blog.settings.domain.error.pending")
		return
	}

	cnameTarget := customDomainCNAMETarget(s.baseDomain)
	routed := false
	if cname, err := net.DefaultResolver.LookupCNAME(ctx, *current.CustomDomain); err == nil && cnamePointsToTarget(cname, cnameTarget) {
		routed = true
	} else if domainIPs := lookupIPs(ctx, *current.CustomDomain); len(domainIPs) > 0 {
		// Apex/root domains can't hold a CNAME, so accept an A/AAAA record
		// match against the routing hostname's current address(es) instead.
		routed = ipsOverlap(domainIPs, lookupIPs(ctx, cnameTarget))
	}
	if !routed {
		s.renderDomainSettingsError(w, r, current, "blog.settings.domain.error.cname_pending")
		return
	}

	if _, err := s.store.VerifyCustomDomain(r.Context(), user.ID); err != nil {
		s.logger.Error("verifying custom domain", "err", err)
		s.renderDomainSettingsError(w, r, current, "blog.settings.domain.error.save")
		return
	}

	s.customDomains.invalidate(*current.CustomDomain)
	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/settings?domain=verified", http.StatusSeeOther)
}

func (s *Server) handleRemoveCustomDomain(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.db")
		return
	}

	var removedDomain string
	if user.CustomDomain != nil {
		removedDomain = *user.CustomDomain
		s.customDomains.invalidate(removedDomain)
	}

	if err := s.store.ClearCustomDomain(r.Context(), user.ID); err != nil {
		s.logger.Error("clearing custom domain", "err", err)
		s.renderDomainSettingsError(w, r, *user, "blog.settings.domain.error.save")
		return
	}

	s.purgePublicCache(user.Username, removedDomain)
	http.Redirect(w, r, "/settings?domain=removed", http.StatusSeeOther)
}

func (s *Server) renderDomainSettingsError(w http.ResponseWriter, r *http.Request, user store.User, messageKey string) {
	pd := s.newPageData(r, &user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	pd.BlogSettings = blogSettingsViewFor(user, s.baseDomain)
	pd.BlogSettings.DomainError = pd.T(messageKey)
	s.renderer.RenderStatus(w, http.StatusBadRequest, "blog_settings.html", pd)
}
