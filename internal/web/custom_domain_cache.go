package web

import (
	"sync"
	"time"
)

const (
	customDomainPositiveTTL = 60 * time.Second
	customDomainNegativeTTL = 30 * time.Second
)

// customDomainCache caches host -> username resolution for custom domains so
// most requests don't need a database round trip. A cached empty username
// means "no verified custom domain owns this host" (negative cache).
type customDomainCache struct {
	mu      sync.RWMutex
	entries map[string]customDomainEntry
}

type customDomainEntry struct {
	username string
	expires  time.Time
}

func newCustomDomainCache() *customDomainCache {
	return &customDomainCache{entries: make(map[string]customDomainEntry)}
}

func (c *customDomainCache) lookup(host string) (username string, ok bool) {
	c.mu.RLock()
	entry, found := c.entries[host]
	c.mu.RUnlock()
	if !found || time.Now().After(entry.expires) {
		return "", false
	}
	return entry.username, true
}

func (c *customDomainCache) store(host, username string, ttl time.Duration) {
	c.mu.Lock()
	c.entries[host] = customDomainEntry{username: username, expires: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *customDomainCache) invalidate(host string) {
	if host == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, host)
	c.mu.Unlock()
}
