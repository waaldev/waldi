package web

import (
	"testing"
	"time"
)

func TestCustomDomainCachePositiveAndNegative(t *testing.T) {
	c := newCustomDomainCache()

	if _, ok := c.lookup("blog.example.com"); ok {
		t.Fatal("expected no entry before store")
	}

	c.store("blog.example.com", "sara", customDomainPositiveTTL)
	username, ok := c.lookup("blog.example.com")
	if !ok || username != "sara" {
		t.Fatalf("got (%q, %v), want (\"sara\", true)", username, ok)
	}

	c.store("nobody.example.com", "", customDomainNegativeTTL)
	username, ok = c.lookup("nobody.example.com")
	if !ok || username != "" {
		t.Fatalf("got (%q, %v), want (\"\", true) for negative cache hit", username, ok)
	}
}

func TestCustomDomainCacheExpiry(t *testing.T) {
	c := newCustomDomainCache()
	c.store("blog.example.com", "sara", -1*time.Second)

	if _, ok := c.lookup("blog.example.com"); ok {
		t.Fatal("expected expired entry to miss")
	}
}

func TestCustomDomainCacheInvalidate(t *testing.T) {
	c := newCustomDomainCache()
	c.store("blog.example.com", "sara", customDomainPositiveTTL)
	c.invalidate("blog.example.com")

	if _, ok := c.lookup("blog.example.com"); ok {
		t.Fatal("expected invalidated entry to miss")
	}

	// invalidate on empty host is a no-op, not a panic.
	c.invalidate("")
}
