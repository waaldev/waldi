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

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := retryCachePurge(ctx, func(ctx context.Context) error {
			return s.cdnPurger.PurgeHosts(ctx, hosts)
		}); err != nil {
			s.logger.Error("purging cdn cache", "err", err, "hosts", hosts)
		}
	}()
}

func retryCachePurge(ctx context.Context, purge func(context.Context) error) error {
	return retryCachePurgeWithDelay(ctx, 250*time.Millisecond, purge)
}

func retryCachePurgeWithDelay(ctx context.Context, delay time.Duration, purge func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = purge(ctx); err == nil {
			return nil
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(delay << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
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
