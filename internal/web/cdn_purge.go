package web

import (
	"context"
	"strings"
	"time"
)

func (s *Server) purgePublicCache(username string, extraHosts ...string) {
	hosts := s.blogPublicHosts(username, extraHosts...)

	for _, host := range hosts {
		s.customDomains.invalidate(host)
	}

	if s.cdnPurger == nil {
		return
	}

	prefixes := s.cdnPurgePrefixes(username, extraHosts...)
	urls := s.cdnPurgeURLs(hosts)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.cdnPurger.PurgePrefixes(ctx, prefixes); err != nil {
			s.logger.Error("purging cdn cache", "err", err, "prefixes", prefixes)
		}
		if len(urls) > 0 {
			if err := s.cdnPurger.PurgeURLs(ctx, urls); err != nil {
				s.logger.Error("purging cdn urls", "err", err, "urls", urls)
			}
		}
	}()
}

func (s *Server) blogPublicHosts(username string, extraHosts ...string) []string {
	base := strings.ToLower(strings.TrimSpace(s.baseDomain))
	if username == "" || base == "" {
		return uniqueHosts(extraHosts)
	}

	hosts := []string{username + "." + base}
	if s.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		user, err := s.store.UserByUsername(ctx, username)
		if err != nil {
			s.logger.Error("loading user for cache purge", "username", username, "err", err)
		} else {
			if domain, ok := user.ActiveCustomDomain(); ok {
				hosts = append(hosts, domain)
			} else if user.CustomDomain != nil {
				hosts = append(hosts, *user.CustomDomain)
			}
		}
	}

	return uniqueHosts(append(hosts, extraHosts...))
}

func (s *Server) cdnPurgePrefixes(username string, extraHosts ...string) []string {
	base := strings.ToLower(strings.TrimSpace(s.baseDomain))
	if base == "" {
		return nil
	}

	prefixes := []string{base + "/"}
	if username == "" {
		return prefixes
	}

	for _, host := range s.blogPublicHosts(username, extraHosts...) {
		prefixes = append(prefixes, host+"/")
	}
	return uniquePrefixes(prefixes)
}

func (s *Server) cdnPurgeURLs(hosts []string) []string {
	urls := make([]string, 0, len(hosts)*4)
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		urls = append(urls,
			"https://"+host+"/",
			"https://"+host+"/feed.xml",
			"https://"+host+"/sitemap.xml",
			"https://"+host+"/robots.txt",
		)
	}
	return uniqueURLs(urls)
}

func uniqueHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func uniquePrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		prefix = strings.TrimPrefix(prefix, "https://")
		prefix = strings.TrimPrefix(prefix, "http://")
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	return out
}

func uniqueURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			u = "https://" + u
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}
