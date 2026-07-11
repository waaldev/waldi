package web

import (
	"net"
	"testing"
)

func TestPlaceholderCustomDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"example.com", true},
		{"blog.example.com", true},
		{"myblog.example.org", true},
		{"site.localhost", true},
		{"realdomain.net", false},
		{"blog.realdomain.net", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := placeholderCustomDomain(tt.domain)
			if got != tt.want {
				t.Fatalf("placeholderCustomDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestValidCustomDomainFormat(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"blog.example.com", true},
		{"example.com", true},
		{"sub.blog.example.com", true},
		{"", false},
		{"nodots", false},
		{"-bad.example.com", false},
		{"bad-.example.com", false},
		{"has space.com", false},
		{"has_underscore.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := validCustomDomainFormat(tt.domain)
			if got != tt.want {
				t.Fatalf("validCustomDomainFormat(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestReservedCustomDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"waldi.blog", true},
		{"sara.waldi.blog", true},
		{"blog.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := reservedCustomDomain(tt.domain, "waldi.blog")
			if got != tt.want {
				t.Fatalf("reservedCustomDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestNormalizeCustomDomain(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"  Blog.Example.com  ", "blog.example.com"},
		{"https://blog.example.com/", "blog.example.com"},
		{"http://blog.example.com", "blog.example.com"},
		{"blog.example.com:8080", "blog.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := normalizeCustomDomain(tt.raw)
			if got != tt.want {
				t.Fatalf("normalizeCustomDomain(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCustomDomainCNAMETarget(t *testing.T) {
	if got := customDomainCNAMETarget("waldi.blog"); got != "cname.waldi.blog" {
		t.Fatalf("got %q", got)
	}
}

func TestCnamePointsToTarget(t *testing.T) {
	if !cnamePointsToTarget("cname.waldi.blog.", "cname.waldi.blog") {
		t.Fatal("expected match")
	}
	if cnamePointsToTarget("other.example.com", "cname.waldi.blog") {
		t.Fatal("expected mismatch")
	}
}

func TestIPsOverlap(t *testing.T) {
	tests := []struct {
		name    string
		domain  []net.IP
		target  []net.IP
		overlap bool
	}{
		{"shared ipv4", []net.IP{net.ParseIP("203.0.113.5")}, []net.IP{net.ParseIP("203.0.113.5")}, true},
		{"disjoint", []net.IP{net.ParseIP("203.0.113.5")}, []net.IP{net.ParseIP("198.51.100.1")}, false},
		{"one of several matches", []net.IP{net.ParseIP("198.51.100.1"), net.ParseIP("203.0.113.5")}, []net.IP{net.ParseIP("203.0.113.5")}, true},
		{"empty domain ips", nil, []net.IP{net.ParseIP("203.0.113.5")}, false},
		{"empty target ips", []net.IP{net.ParseIP("203.0.113.5")}, nil, false},
		{"shared ipv6", []net.IP{net.ParseIP("2001:db8::1")}, []net.IP{net.ParseIP("2001:db8::1")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ipsOverlap(tt.domain, tt.target); got != tt.overlap {
				t.Fatalf("ipsOverlap(%v, %v) = %v, want %v", tt.domain, tt.target, got, tt.overlap)
			}
		})
	}
}
